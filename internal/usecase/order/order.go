package order

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"ecommerce-api/pkg/notification"
	"github.com/google/uuid"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrCartEmpty              = errors.New("cart is empty")
	ErrInsufficientStock      = errors.New("one or more items have insufficient stock")
	ErrAddressNotFound        = errors.New("address not found")
	ErrOrderNotFound          = errors.New("order not found")
	ErrUnauthorized           = errors.New("unauthorized to access this order")
	ErrCannotCancel           = errors.New("order can only be cancelled when status is pending_payment")
	ErrShipmentNotFound       = errors.New("shipment not found")
	ErrInvalidShipmentStatus  = errors.New("invalid shipment status")
)

// ─── DTOs ────────────────────────────────────────────────────────────────────

type CreateOrderInput struct {
	UserID         uuid.UUID
	AddressID      uuid.UUID
	Courier        string
	CourierService string
	Notes          string
	ShippingCost   int64
}

// ─── Interface ────────────────────────────────────────────────────────────────

type CreateShipmentInput struct {
	OrderID        uuid.UUID
	TrackingNumber string
	Courier        string
	CourierService string
}

type UseCase interface {
	Checkout(ctx context.Context, input CreateOrderInput) (*domain.Order, error)
	GetByID(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error)
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Order, int, error)
	ListAll(ctx context.Context, filter repository.OrderFilter) ([]domain.Order, int, error)
	UpdateStatus(ctx context.Context, orderID uuid.UUID, status domain.OrderStatus, actorID uuid.UUID, actorRole string) error
	CreateShipment(ctx context.Context, input CreateShipmentInput, actorID uuid.UUID) (*domain.Shipment, error)
	GetShipment(ctx context.Context, orderID, userID uuid.UUID) (*domain.Shipment, error)
	UpdateShipmentStatus(ctx context.Context, orderID uuid.UUID, status domain.ShipmentStatus, actorID uuid.UUID) (*domain.Shipment, error)
	Cancel(ctx context.Context, orderID, userID uuid.UUID) error
}

// ─── Implementation ───────────────────────────────────────────────────────────

type useCase struct {
	orderRepo    repository.OrderRepository
	cartRepo     repository.CartRepository
	variantRepo  repository.ProductVariantRepository
	addressRepo  repository.AddressRepository
	paymentRepo  repository.PaymentRepository
	shipmentRepo repository.ShipmentRepository
	auditRepo    repository.AuditLogRepository
	userRepo     repository.UserRepository
	notifier     notification.Sender
	adminPhone   string
}

func NewUseCase(
	orderRepo repository.OrderRepository,
	cartRepo repository.CartRepository,
	variantRepo repository.ProductVariantRepository,
	addressRepo repository.AddressRepository,
	paymentRepo repository.PaymentRepository,
	shipmentRepo repository.ShipmentRepository,
	auditRepo repository.AuditLogRepository,
	userRepo repository.UserRepository,
	notifier notification.Sender,
	adminPhone string,
) UseCase {
	return &useCase{
		orderRepo:    orderRepo,
		cartRepo:     cartRepo,
		variantRepo:  variantRepo,
		addressRepo:  addressRepo,
		paymentRepo:  paymentRepo,
		shipmentRepo: shipmentRepo,
		auditRepo:    auditRepo,
		userRepo:     userRepo,
		notifier:     notifier,
		adminPhone:   adminPhone,
	}
}

// Checkout adalah inti dari flow belanja:
// validasi cart → snapshot harga → decrement stok → create order → create payment → clear cart
func (uc *useCase) Checkout(ctx context.Context, input CreateOrderInput) (*domain.Order, error) {
	// 1. Ambil cart
	cart, err := uc.cartRepo.FindByUserID(ctx, input.UserID)
	if err != nil || cart == nil || len(cart.Items) == 0 {
		return nil, ErrCartEmpty
	}

	// 2. Validasi alamat milik user ini
	address, err := uc.addressRepo.FindByID(ctx, input.AddressID)
	if err != nil || address == nil || address.UserID != input.UserID {
		return nil, ErrAddressNotFound
	}

	// 3. Snapshot alamat — disimpan langsung di order agar history tidak berubah
	snapshotAddress := map[string]any{
		"recipient_name": address.RecipientName,
		"phone":          address.Phone,
		"street":         address.Street,
		"city":           address.City,
		"province":       address.Province,
		"postal_code":    address.PostalCode,
	}

	// 4. Build order items + validasi stok + hitung subtotal
	var orderItems []domain.OrderItem
	var decrements []repository.StockDecrement
	var subtotal int64

	for _, cartItem := range cart.Items {
		variant := cartItem.Variant
		if variant == nil || variant.Stock < cartItem.Quantity {
			return nil, fmt.Errorf("%w: %s", ErrInsufficientStock, cartItem.Product.Name)
		}

		itemSubtotal := variant.Price * int64(cartItem.Quantity)
		subtotal += itemSubtotal

		orderItems = append(orderItems, domain.OrderItem{
			ID:          uuid.New(),
			VariantID:   &variant.ID,
			ProductName: cartItem.Product.Name,
			VariantName: variant.Name,
			Quantity:    cartItem.Quantity,
			UnitPrice:   variant.Price,
			Subtotal:    itemSubtotal,
			CreatedAt:   time.Now(),
		})

		decrements = append(decrements, repository.StockDecrement{
			VariantID: cartItem.VariantID,
			Quantity:  cartItem.Quantity,
		})
	}

	total := subtotal + input.ShippingCost

	// 5. Buat order
	now := time.Now()
	order := &domain.Order{
		ID:              uuid.New(),
		UserID:          input.UserID,
		AddressID:       input.AddressID,
		SnapshotAddress: snapshotAddress,
		Status:          domain.OrderStatusPendingPayment,
		Subtotal:        subtotal,
		ShippingCost:    input.ShippingCost,
		Total:           total,
		Courier:         input.Courier,
		CourierService:  input.CourierService,
		Notes:           input.Notes,
		Items:           orderItems,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// order + order_items + stock decrement dalam satu transaksi DB
	if err := uc.orderRepo.Create(ctx, order, decrements); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// 7. Buat payment record awal (status: pending)
	payment := &domain.Payment{
		ID:        uuid.New(),
		OrderID:   order.ID,
		Provider:  "midtrans",
		Status:    domain.PaymentStatusPending,
		Amount:    float64(total),
		CreatedAt: now,
		UpdatedAt: now,
	}
	uc.paymentRepo.Create(ctx, payment)

	// 8. Bersihkan cart
	uc.cartRepo.ClearCart(ctx, cart.ID)

	// 9. Notifikasi WA
	if buyer, err := uc.userRepo.FindByID(ctx, input.UserID); err == nil && buyer != nil {
		uc.notify(buyer.Phone, notification.MsgOrderCreated(order.ID.String(), order.Total))
		uc.notify(uc.adminPhone, notification.MsgNewOrderAdmin(buyer.Name, order.ID.String(), order.Total))
	}

	// 10. Audit log
	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &input.UserID,
		ActorRole:  "buyer",
		Action:     domain.AuditCreate,
		EntityType: "orders",
		EntityID:   &order.ID,
		NewData:    map[string]any{"status": order.Status, "total": order.Total},
		CreatedAt:  now,
	})

	return order, nil
}

func (uc *useCase) GetByID(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error) {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrOrderNotFound
	}
	// buyer hanya bisa lihat order miliknya
	if order.UserID != userID {
		return nil, ErrUnauthorized
	}
	return order, nil
}

func (uc *useCase) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Order, int, error) {
	return uc.orderRepo.FindAll(ctx, repository.OrderFilter{
		UserID: &userID,
		Page:   page,
		Limit:  limit,
	})
}

func (uc *useCase) ListAll(ctx context.Context, filter repository.OrderFilter) ([]domain.Order, int, error) {
	return uc.orderRepo.FindAll(ctx, filter)
}

func (uc *useCase) CreateShipment(ctx context.Context, input CreateShipmentInput, actorID uuid.UUID) (*domain.Shipment, error) {
	order, err := uc.orderRepo.FindByID(ctx, input.OrderID)
	if err != nil || order == nil {
		return nil, ErrOrderNotFound
	}

	// Pastikan order sudah dibayar sebelum bisa dikirim
	if order.Status != domain.OrderStatusPaid && order.Status != domain.OrderStatusProcessing {
		return nil, fmt.Errorf("order must be paid or processing before creating shipment")
	}

	now := time.Now()
	shipment := &domain.Shipment{
		ID:             uuid.New(),
		OrderID:        input.OrderID,
		Courier:        input.Courier,
		CourierService: input.CourierService,
		TrackingNumber: input.TrackingNumber,
		Status:         domain.ShipmentStatusPickedUp,
		ShippedAt:      &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := uc.shipmentRepo.Create(ctx, shipment); err != nil {
		return nil, fmt.Errorf("create shipment: %w", err)
	}

	// Update order status ke shipped
	uc.orderRepo.UpdateStatus(ctx, input.OrderID, domain.OrderStatusShipped)

	// Notifikasi buyer
	if buyer, err := uc.userRepo.FindByID(ctx, order.UserID); err == nil && buyer != nil {
		uc.notify(buyer.Phone, notification.MsgOrderShipped(input.OrderID.String(), input.Courier, input.TrackingNumber))
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &actorID,
		ActorRole:  "admin",
		Action:     domain.AuditCreate,
		EntityType: "shipments",
		EntityID:   &shipment.ID,
		NewData:    map[string]any{"tracking_number": input.TrackingNumber, "courier": input.Courier},
		CreatedAt:  now,
	})

	return shipment, nil
}

// Cancel membatalkan order milik buyer — hanya boleh saat status pending_payment.
// Stok dikembalikan secara atomic bersama perubahan status.
func (uc *useCase) Cancel(ctx context.Context, orderID, userID uuid.UUID) error {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return ErrOrderNotFound
	}
	if order.UserID != userID {
		return ErrUnauthorized
	}
	if order.Status != domain.OrderStatusPendingPayment {
		return ErrCannotCancel
	}

	if err := uc.orderRepo.Cancel(ctx, orderID); err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &userID,
		ActorRole:  "buyer",
		Action:     domain.AuditUpdate,
		EntityType: "orders",
		EntityID:   &orderID,
		OldData:    map[string]any{"status": order.Status},
		NewData:    map[string]any{"status": domain.OrderStatusCancelled},
		CreatedAt:  time.Now(),
	})

	return nil
}

func (uc *useCase) GetShipment(ctx context.Context, orderID, userID uuid.UUID) (*domain.Shipment, error) {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrUnauthorized
	}
	shipment, err := uc.shipmentRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("get shipment: %w", err)
	}
	if shipment == nil {
		return nil, ErrShipmentNotFound
	}
	return shipment, nil
}

var validShipmentTransitions = map[domain.ShipmentStatus]bool{
	domain.ShipmentStatusInTransit: true,
	domain.ShipmentStatusDelivered: true,
	domain.ShipmentStatusReturned:  true,
}

func (uc *useCase) UpdateShipmentStatus(ctx context.Context, orderID uuid.UUID, status domain.ShipmentStatus, actorID uuid.UUID) (*domain.Shipment, error) {
	if !validShipmentTransitions[status] {
		return nil, ErrInvalidShipmentStatus
	}

	shipment, err := uc.shipmentRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("find shipment: %w", err)
	}
	if shipment == nil {
		return nil, ErrShipmentNotFound
	}

	now := time.Now()
	shipment.Status = status
	if status == domain.ShipmentStatusDelivered {
		shipment.DeliveredAt = &now
	}

	if err := uc.shipmentRepo.Update(ctx, shipment); err != nil {
		return nil, fmt.Errorf("update shipment: %w", err)
	}

	// Sinkronisasi status order saat barang diterima
	if status == domain.ShipmentStatusDelivered {
		uc.orderRepo.UpdateStatus(ctx, orderID, domain.OrderStatusDelivered)

		// Notifikasi buyer
		if order, err := uc.orderRepo.FindByID(ctx, orderID); err == nil && order != nil {
			if buyer, err := uc.userRepo.FindByID(ctx, order.UserID); err == nil && buyer != nil {
				uc.notify(buyer.Phone, notification.MsgOrderDelivered(orderID.String()))
			}
		}
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &actorID,
		ActorRole:  "admin",
		Action:     domain.AuditUpdate,
		EntityType: "shipments",
		EntityID:   &shipment.ID,
		OldData:    map[string]any{"status": shipment.Status},
		NewData:    map[string]any{"status": status},
		CreatedAt:  now,
	})

	return shipment, nil
}

// notify kirim WA secara async — tidak pernah block atau gagalkan operasi utama
func (uc *useCase) notify(phone, message string) {
	if uc.notifier == nil || phone == "" {
		return
	}
	go func() {
		if err := uc.notifier.Send(context.Background(), phone, message); err != nil {
			slog.Warn("notification failed", "phone", phone, "error", err)
		}
	}()
}

func (uc *useCase) UpdateStatus(ctx context.Context, orderID uuid.UUID, status domain.OrderStatus, actorID uuid.UUID, actorRole string) error {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return ErrOrderNotFound
	}

	oldStatus := order.Status
	if err := uc.orderRepo.UpdateStatus(ctx, orderID, status); err != nil {
		return err
	}

	// Audit setiap perubahan status
	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		ActorID:    &actorID,
		ActorRole:  actorRole,
		Action:     domain.AuditUpdate,
		EntityType: "orders",
		EntityID:   &orderID,
		OldData:    map[string]any{"status": oldStatus},
		NewData:    map[string]any{"status": status},
		CreatedAt:  time.Now(),
	})

	return nil
}
