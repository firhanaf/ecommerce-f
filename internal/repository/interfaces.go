package repository

import (
	"context"
	"time"

	"ecommerce-api/internal"
	"github.com/google/uuid"
)

// ─── User ────────────────────────────────────────────────────────────────────
// Ganti implementasi (Postgres → MySQL) cukup buat struct baru yang implement interface ini

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByPhone(ctx context.Context, phone string) (*domain.User, error)
	FindAll(ctx context.Context, page, limit int) ([]domain.User, int, error)
	Update(ctx context.Context, user *domain.User) error
	UpdateStatus(ctx context.Context, id uuid.UUID, isActive bool) error
	UpdatePhoneVerified(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// ─── OTP ─────────────────────────────────────────────────────────────────────

type OTPRepository interface {
	Create(ctx context.Context, otp *domain.OTPToken) error
	FindLatest(ctx context.Context, userID uuid.UUID, otpType string) (*domain.OTPToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
	IncrementAttempts(ctx context.Context, id uuid.UUID) error
	CountRecent(ctx context.Context, userID uuid.UUID, otpType string, since time.Time) (int, error)
}

// ─── Address ─────────────────────────────────────────────────────────────────

type AddressRepository interface {
	Create(ctx context.Context, addr *domain.Address) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Address, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Address, error)
	Update(ctx context.Context, addr *domain.Address) error
	Delete(ctx context.Context, id uuid.UUID) error
	SetDefault(ctx context.Context, userID, addressID uuid.UUID) error
}

// ─── Category ────────────────────────────────────────────────────────────────

type CategoryRepository interface {
	Create(ctx context.Context, cat *domain.Category) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Category, error)
	FindAll(ctx context.Context) ([]domain.Category, error)
	Update(ctx context.Context, cat *domain.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ─── Product ─────────────────────────────────────────────────────────────────

type ProductFilter struct {
	CategoryID *uuid.UUID
	Search     string
	IsActive   *bool
	Page       int
	Limit      int
}

type ProductRepository interface {
	Create(ctx context.Context, product *domain.Product) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Product, error)
	FindAll(ctx context.Context, filter ProductFilter) ([]domain.Product, int, error)
	Update(ctx context.Context, product *domain.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProductVariantRepository interface {
	Create(ctx context.Context, variant *domain.ProductVariant) error
	FindByProductID(ctx context.Context, productID uuid.UUID) ([]domain.ProductVariant, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ProductVariant, error)
	Update(ctx context.Context, variant *domain.ProductVariant) error
	DecrementStock(ctx context.Context, variantID uuid.UUID, qty int) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProductImageRepository interface {
	Create(ctx context.Context, image *domain.ProductImage) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ProductImage, error)
	FindByProductID(ctx context.Context, productID uuid.UUID) ([]domain.ProductImage, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ─── Cart ────────────────────────────────────────────────────────────────────

type CartRepository interface {
	FindOrCreateByUserID(ctx context.Context, userID uuid.UUID) (*domain.Cart, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) (*domain.Cart, error)
	AddItem(ctx context.Context, item *domain.CartItem) error
	UpdateItemQuantity(ctx context.Context, cartItemID uuid.UUID, qty int) error
	RemoveItem(ctx context.Context, cartItemID uuid.UUID) error
	ClearCart(ctx context.Context, cartID uuid.UUID) error
}

// ─── Order ───────────────────────────────────────────────────────────────────

type OrderFilter struct {
	UserID *uuid.UUID
	Status *domain.OrderStatus
	Page   int
	Limit  int
}

// StockDecrement dipakai untuk decrement stok dalam satu transaksi bersama order
type StockDecrement struct {
	VariantID uuid.UUID
	Quantity  int
}

type OrderRepository interface {
	// Create menyimpan order + order_items + decrement stok dalam satu transaksi DB
	Create(ctx context.Context, order *domain.Order, decrements []StockDecrement) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	FindAll(ctx context.Context, filter OrderFilter) ([]domain.Order, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error
}

// ─── Payment ─────────────────────────────────────────────────────────────────

type PaymentRepository interface {
	Create(ctx context.Context, payment *domain.Payment) error
	FindByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error)
	FindByTransactionID(ctx context.Context, txID string) (*domain.Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.PaymentStatus, rawResponse map[string]any) error
}

// ─── Shipment ────────────────────────────────────────────────────────────────

type ShipmentRepository interface {
	Create(ctx context.Context, shipment *domain.Shipment) error
	FindByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Shipment, error)
	Update(ctx context.Context, shipment *domain.Shipment) error
}

// ─── Audit Log ───────────────────────────────────────────────────────────────

type AuditFilter struct {
	ActorID    *uuid.UUID
	EntityType string
	EntityID   *uuid.UUID
	Action     *domain.AuditAction
	Page       int
	Limit      int
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *domain.AuditLog) error
	FindAll(ctx context.Context, filter AuditFilter) ([]domain.AuditLog, int, error)
}
