package router

import (
	"net/http"

	"ecommerce-api/internal/handler"
	"ecommerce-api/internal/middleware"
	"ecommerce-api/pkg/jwt"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Handlers struct {
	Auth     *handler.AuthHandler
	Product  *handler.ProductHandler
	Category *handler.CategoryHandler
	Address  *handler.AddressHandler
	Cart     *handler.CartHandler
	Order    *handler.OrderHandler
	Payment  *handler.PaymentHandler
	Admin    *handler.AdminHandler
}

// UploadsDir adalah direktori lokal untuk file upload (dipakai saat STORAGE=local).
// Kosong berarti local storage tidak dipakai.
var UploadsDir string

func New(h Handlers, tokenSvc jwt.TokenService) http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(chimiddleware.CleanPath)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Serve file upload lokal (hanya aktif jika UploadsDir diset)
	if UploadsDir != "" {
		fs := http.FileServer(http.Dir(UploadsDir))
		r.Handle("/uploads/*", http.StripPrefix("/uploads/", fs))
	}

	r.Route("/api/v1", func(r chi.Router) {

		// ── Public routes ────────────────────────────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", h.Auth.Register)
			r.Post("/verify-otp", h.Auth.VerifyOTP)
			r.Post("/resend-otp", h.Auth.ResendOTP)
			r.Post("/login", h.Auth.Login)
			r.Post("/refresh", h.Auth.Refresh)
			r.Post("/forgot-password", h.Auth.ForgotPassword)
			r.Post("/reset-password", h.Auth.ResetPassword)
		})

		// Katalog produk & kategori — public
		r.Get("/products", h.Product.List)
		r.Get("/products/{slug}", h.Product.GetBySlug)
		r.Get("/categories", h.Category.List)

		// Webhook Midtrans — public (Midtrans tidak kirim JWT)
		r.Post("/payments/webhook", h.Payment.Webhook)

		// ── Buyer routes (JWT required) ──────────────────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(tokenSvc))

			r.Route("/addresses", func(r chi.Router) {
				r.Get("/", h.Address.List)
				r.Post("/", h.Address.Create)
				r.Put("/{id}", h.Address.Update)
				r.Delete("/{id}", h.Address.Delete)
				r.Put("/{id}/default", h.Address.SetDefault)
			})

			r.Route("/cart", func(r chi.Router) {
				r.Get("/", h.Cart.Get)
				r.Post("/items", h.Cart.AddItem)
				r.Put("/items/{itemID}", h.Cart.UpdateItem)
				r.Delete("/items/{itemID}", h.Cart.RemoveItem)
			})

			r.Route("/orders", func(r chi.Router) {
				r.Get("/", h.Order.List)
				r.Post("/", h.Order.Create)
				r.Get("/{id}", h.Order.GetByID)
				r.Post("/{id}/pay", h.Payment.InitiateForOrder)
				r.Post("/{id}/cancel", h.Order.Cancel)
				r.Get("/{id}/shipment", h.Order.GetShipment)
				r.Get("/{id}/payment", h.Payment.GetByOrder)
			})
		})

		// ── Admin routes (JWT + role admin required) ─────────────────────────
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(tokenSvc))
			r.Use(middleware.RequireRole("admin"))

			r.Route("/admin", func(r chi.Router) {
				// User management
				r.Get("/users", h.Admin.ListUsers)
				r.Put("/users/{id}/status", h.Admin.UpdateUserStatus)

				// Audit log
				r.Get("/audit-logs", h.Admin.ListAuditLogs)

				// Category management
				r.Post("/categories", h.Category.Create)
				r.Put("/categories/{id}", h.Category.Update)
				r.Delete("/categories/{id}", h.Category.Delete)

				// Product management
				r.Post("/products", h.Product.Create)
				r.Put("/products/{id}", h.Product.Update)
				r.Delete("/products/{id}", h.Product.Delete)
				r.Post("/products/{id}/images", h.Product.UploadImage)
				r.Delete("/products/{id}/images/{imageID}", h.Product.DeleteImage)

				// Variant management
				r.Post("/products/{id}/variants", h.Product.CreateVariant)
				r.Put("/products/{id}/variants/{variantID}", h.Product.UpdateVariant)
				r.Delete("/products/{id}/variants/{variantID}", h.Product.DeleteVariant)
				r.Put("/products/{id}/variants/{variantID}/stock", h.Product.AdjustStock)

				// Order management
				r.Get("/orders", h.Admin.ListOrders)
				r.Put("/orders/{id}/status", h.Order.UpdateStatus)
				r.Post("/orders/{id}/shipment", h.Order.CreateShipment)
				r.Put("/orders/{id}/shipment", h.Order.UpdateShipmentStatus)
			})
		})
	})

	return r
}
