package router

import (
	"net/http"

	"ecommerce-api/internal/handler"
	"ecommerce-api/internal/middleware"
	"ecommerce-api/pkg/jwt"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
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

func New(h Handlers, tokenSvc jwt.TokenService) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(chimiddleware.CleanPath)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v1", func(r chi.Router) {

		// ── Public routes ────────────────────────────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", h.Auth.Register)
			r.Post("/verify-otp", h.Auth.VerifyOTP)
			r.Post("/resend-otp", h.Auth.ResendOTP)
			r.Post("/login", h.Auth.Login)
			r.Post("/refresh", h.Auth.Refresh)
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

				// Order management
				r.Get("/orders", h.Admin.ListOrders)
				r.Put("/orders/{id}/status", h.Order.UpdateStatus)
				r.Post("/orders/{id}/shipment", h.Order.CreateShipment)
			})
		})
	})

	return r
}
