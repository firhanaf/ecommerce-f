package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ecommerce-api/config"
	"ecommerce-api/internal/handler"
	"ecommerce-api/internal/repository/postgres"
	"ecommerce-api/internal/router"
	authUC    "ecommerce-api/internal/usecase/auth"
	cartUC    "ecommerce-api/internal/usecase/cart"
	orderUC   "ecommerce-api/internal/usecase/order"
	paymentUC "ecommerce-api/internal/usecase/payment"
	productUC "ecommerce-api/internal/usecase/product"
	"ecommerce-api/pkg/database"
	"ecommerce-api/pkg/jwt"
	"ecommerce-api/pkg/notification"
	pkgotp "ecommerce-api/pkg/otp"
	"ecommerce-api/pkg/payment"
	"ecommerce-api/pkg/storage"
)

func main() {
	ctx := context.Background()

	// ── 1. Load config ───────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// ── 2. Init infrastructure ───────────────────────────────────────────────

	// Database
	db, err := database.NewPool(ctx, cfg.Database.DSN())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("database connected")

	// Storage — pilih backend berdasarkan STORAGE_TYPE di .env
	//   local → simpan ke ./uploads/, serve via /uploads/*
	//   s3    → AWS S3 atau MinIO (jika AWS_S3_ENDPOINT diset)
	storageType := cfg.Storage.Type
	var storageClient storage.Storage
	switch storageType {
	case "local":
		uploadsDir := cfg.Storage.LocalDir
		if uploadsDir == "" {
			uploadsDir = "./uploads"
		}
		storageClient, err = storage.NewLocalStorage(uploadsDir, cfg.Storage.LocalBaseURL)
		if err != nil {
			log.Fatalf("failed to init local storage: %v", err)
		}
		router.UploadsDir = uploadsDir
		log.Printf("storage: local filesystem at %s", uploadsDir)
	default:
		storageClient, err = storage.NewS3StorageWithEndpoint(ctx, cfg.AWS.S3Bucket, cfg.AWS.Region, cfg.AWS.S3Endpoint)
		if err != nil {
			log.Fatalf("failed to init storage: %v", err)
		}
		if cfg.AWS.S3Endpoint != "" {
			log.Printf("storage: MinIO at %s (bucket: %s)", cfg.AWS.S3Endpoint, cfg.AWS.S3Bucket)
		} else {
			log.Printf("storage: AWS S3 (bucket: %s)", cfg.AWS.S3Bucket)
		}
	}

	// Token service
	// Mau ganti ke Paseto? Ganti NewJWTService dengan NewPasetoService — middleware tidak berubah
	tokenSvc := jwt.NewJWTService(
		cfg.JWT.SecretKey,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
	)

	// Payment gateway
	// Mau ganti ke Xendit? Ganti NewMidtransGateway — payment usecase tidak berubah
	paymentGW := payment.NewMidtransGateway(cfg.Midtrans.ServerKey, cfg.Midtrans.Production)

	// ── 3. Init external services ────────────────────────────────────────────
	otpSender := pkgotp.NewFonnteClient(cfg.Fonnte.Token)
	notifier := notification.NewFonnteNotifier(cfg.Fonnte.Token)

	// ── 4. Init repositories (implementasi Postgres) ─────────────────────────
	userRepo := postgres.NewUserRepository(db)
	addressRepo := postgres.NewAddressRepository(db)
	categoryRepo := postgres.NewCategoryRepository(db)
	productRepo := postgres.NewProductRepository(db)
	variantRepo := postgres.NewProductVariantRepository(db)
	imageRepo := postgres.NewProductImageRepository(db)
	cartRepo := postgres.NewCartRepository(db)
	orderRepo := postgres.NewOrderRepository(db)
	paymentRepo := postgres.NewPaymentRepository(db)
	shipmentRepo := postgres.NewShipmentRepository(db)
	auditRepo := postgres.NewAuditLogRepository(db)
	otpRepo := postgres.NewOTPRepository(db)

	// ── 5. Init usecases ─────────────────────────────────────────────────────
	authUseCase := authUC.NewUseCase(userRepo, otpRepo, tokenSvc, otpSender, auditRepo)
	productUseCase := productUC.NewUseCase(productRepo, variantRepo, imageRepo, storageClient, auditRepo)
	cartUseCase := cartUC.NewUseCase(cartRepo, variantRepo)
	orderUseCase := orderUC.NewUseCase(orderRepo, cartRepo, variantRepo, addressRepo, paymentRepo, shipmentRepo, auditRepo, userRepo, notifier, cfg.Fonnte.AdminPhone)
	paymentUseCase := paymentUC.NewUseCase(paymentRepo, orderRepo, userRepo, paymentGW, auditRepo, notifier, cfg.Fonnte.AdminPhone)

	// ── 6. Init handlers ─────────────────────────────────────────────────────
	handlers := router.Handlers{
		Auth:     handler.NewAuthHandler(authUseCase),
		Product:  handler.NewProductHandler(productUseCase),
		Category: handler.NewCategoryHandler(categoryRepo),
		Address:  handler.NewAddressHandler(addressRepo),
		Cart:     handler.NewCartHandler(cartUseCase),
		Order:    handler.NewOrderHandler(orderUseCase),
		Payment:  handler.NewPaymentHandler(paymentUseCase),
		Admin:    handler.NewAdminHandler(userRepo, auditRepo, orderUseCase),
	}

	// ── 7. Init router ───────────────────────────────────────────────────────
	r := router.New(handlers, tokenSvc)

	// ── 8. Start server dengan graceful shutdown ──────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// channel untuk catch OS signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server running on port %s (env: %s)", cfg.App.Port, cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// tunggu signal
	<-quit
	log.Println("shutting down server...")

	// beri waktu 10 detik untuk request yang sedang berjalan selesai
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited")
}
