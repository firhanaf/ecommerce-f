package domain

import (
	"github.com/google/uuid"
	"time"
)

// ─── User ────────────────────────────────────────────────────────────────────

type UserRole string

const (
	RoleBuyer UserRole = "buyer"
	RoleAdmin UserRole = "admin"
)

type User struct {
	ID            uuid.UUID
	Name          string
	Email         string
	PasswordHash  string
	Role          UserRole
	Phone         string
	AvatarURL     string
	IsActive      bool
	PhoneVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ─── OTP ─────────────────────────────────────────────────────────────────────

const (
	OTPTypePhoneVerification = "phone_verification"
	OTPTypeResetPassword     = "reset_password"
)

type OTPToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Code      string
	Type      string
	ExpiresAt time.Time
	UsedAt    *time.Time
	Attempts  int
	CreatedAt time.Time
}

// ─── Address ─────────────────────────────────────────────────────────────────

type Address struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	RecipientName string
	Phone         string
	Street        string
	City          string
	Province      string
	PostalCode    string
	IsDefault     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ─── Category ────────────────────────────────────────────────────────────────

type Category struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	ParentID  *uuid.UUID
	IsActive  bool
	CreatedAt time.Time
}

// ─── Product ─────────────────────────────────────────────────────────────────

type Product struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description string
	CategoryID  *uuid.UUID
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// populated on demand (bukan selalu di-query)
	Variants []ProductVariant
	Images   []ProductImage
}

type ProductVariant struct {
	ID        uuid.UUID
	ProductID uuid.UUID
	Name      string
	SKU       string
	Price     int64
	Stock     int
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProductImage struct {
	ID        uuid.UUID
	ProductID uuid.UUID
	URL       string
	IsPrimary bool
	SortOrder int
	CreatedAt time.Time
}

// ─── Cart ────────────────────────────────────────────────────────────────────

type Cart struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Items     []CartItem
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CartItem struct {
	ID        uuid.UUID
	CartID    uuid.UUID
	VariantID uuid.UUID
	Quantity  int
	CreatedAt time.Time
	UpdatedAt time.Time

	// populated untuk display
	Variant *ProductVariant
	Product *Product
}

// ─── Order ───────────────────────────────────────────────────────────────────

type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "pending_payment"
	OrderStatusPaid           OrderStatus = "paid"
	OrderStatusProcessing     OrderStatus = "processing"
	OrderStatusShipped        OrderStatus = "shipped"
	OrderStatusDelivered      OrderStatus = "delivered"
	OrderStatusCompleted      OrderStatus = "completed"
	OrderStatusCancelled      OrderStatus = "cancelled"
	OrderStatusRefunded       OrderStatus = "refunded"
)

type Order struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	AddressID       uuid.UUID
	SnapshotAddress map[string]any // JSONB snapshot
	Status          OrderStatus
	Subtotal        int64
	ShippingCost    int64
	Total           int64
	Courier         string
	CourierService  string
	Notes           string
	Items           []OrderItem
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type OrderItem struct {
	ID           uuid.UUID
	OrderID      uuid.UUID
	VariantID    *uuid.UUID
	ProductName  string
	VariantName  string
	ProductImage string
	Quantity     int
	UnitPrice    int64
	Subtotal     int64
	CreatedAt    time.Time
}

// ─── Payment ─────────────────────────────────────────────────────────────────

type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusSettlement PaymentStatus = "settlement"
	PaymentStatusCancel     PaymentStatus = "cancel"
	PaymentStatusExpire     PaymentStatus = "expire"
	PaymentStatusRefund     PaymentStatus = "refund"
	PaymentStatusFailed     PaymentStatus = "failed"
)

type Payment struct {
	ID            uuid.UUID
	OrderID       uuid.UUID
	Provider      string
	PaymentMethod string
	Status        PaymentStatus
	TransactionID string
	Amount        float64
	RawResponse   map[string]any
	PaidAt        *time.Time
	ExpiredAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ─── Shipment ────────────────────────────────────────────────────────────────

type ShipmentStatus string

const (
	ShipmentStatusPending   ShipmentStatus = "pending"
	ShipmentStatusPickedUp  ShipmentStatus = "picked_up"
	ShipmentStatusInTransit ShipmentStatus = "in_transit"
	ShipmentStatusDelivered ShipmentStatus = "delivered"
	ShipmentStatusReturned  ShipmentStatus = "returned"
)

type Shipment struct {
	ID             uuid.UUID
	OrderID        uuid.UUID
	Courier        string
	CourierService string
	TrackingNumber string
	Status         ShipmentStatus
	ShippedAt      *time.Time
	DeliveredAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ─── Audit Log ───────────────────────────────────────────────────────────────

type AuditAction string

const (
	AuditCreate AuditAction = "CREATE"
	AuditUpdate AuditAction = "UPDATE"
	AuditDelete AuditAction = "DELETE"
	AuditLogin  AuditAction = "LOGIN"
	AuditLogout AuditAction = "LOGOUT"
	AuditExport AuditAction = "EXPORT"
)

type AuditLog struct {
	ID         uuid.UUID
	ActorID    *uuid.UUID
	ActorRole  string
	Action     AuditAction
	EntityType string
	EntityID   *uuid.UUID
	OldData    map[string]any
	NewData    map[string]any
	IPAddress  string
	UserAgent  string
	Notes      string
	CreatedAt  time.Time
}
