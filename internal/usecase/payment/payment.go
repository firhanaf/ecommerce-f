package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"ecommerce-api/pkg/notification"
	"ecommerce-api/pkg/payment"
	"github.com/google/uuid"
)

var (
	ErrInvalidWebhook = errors.New("invalid webhook signature")
	ErrOrderNotFound  = errors.New("order not found")
	ErrPaymentExists  = errors.New("payment already initiated for this order")
)

type InitiateResult struct {
	SnapToken   string
	RedirectURL string
}

type UseCase interface {
	InitiatePayment(ctx context.Context, orderID, userID uuid.UUID) (*InitiateResult, error)
	HandleWebhook(ctx context.Context, payload []byte) error
	GetByOrderID(ctx context.Context, orderID, userID uuid.UUID) (*domain.Payment, error)
}

type useCase struct {
	paymentRepo repository.PaymentRepository
	orderRepo   repository.OrderRepository
	userRepo    repository.UserRepository
	gateway     payment.PaymentGateway
	auditRepo   repository.AuditLogRepository
	notifier    notification.Sender
	adminPhone  string
}

func NewUseCase(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	userRepo repository.UserRepository,
	gateway payment.PaymentGateway,
	auditRepo repository.AuditLogRepository,
	notifier notification.Sender,
	adminPhone string,
) UseCase {
	return &useCase{
		paymentRepo: paymentRepo,
		orderRepo:   orderRepo,
		userRepo:    userRepo,
		gateway:     gateway,
		auditRepo:   auditRepo,
		notifier:    notifier,
		adminPhone:  adminPhone,
	}
}

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

// InitiatePayment membuat charge ke Midtrans Snap dan mengembalikan snap token
func (uc *useCase) InitiatePayment(ctx context.Context, orderID, userID uuid.UUID) (*InitiateResult, error) {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrOrderNotFound
	}

	// Verifikasi order milik user ini
	if order.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	// Cek payment record yang sudah ada
	pmnt, err := uc.paymentRepo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if pmnt == nil {
		return nil, fmt.Errorf("payment record not found for order %s", orderID)
	}

	// Build charge items dari order items
	chargeItems := make([]payment.ChargeItem, 0, len(order.Items))
	for _, item := range order.Items {
		chargeItems = append(chargeItems, payment.ChargeItem{
			Name:     fmt.Sprintf("%s - %s", item.ProductName, item.VariantName),
			Price:    float64(item.UnitPrice),
			Quantity: item.Quantity,
		})
	}

	// Tambahkan shipping cost sebagai item
	if order.ShippingCost > 0 {
		chargeItems = append(chargeItems, payment.ChargeItem{
			Name:     fmt.Sprintf("Shipping (%s %s)", order.Courier, order.CourierService),
			Price:    float64(order.ShippingCost),
			Quantity: 1,
		})
	}

	expiredAt := time.Now().Add(24 * time.Hour)
	resp, err := uc.gateway.CreateCharge(ctx, payment.CreateChargeRequest{
		OrderID:   order.ID.String(),
		Amount:    float64(order.Total),
		Items:     chargeItems,
		ExpiredAt: expiredAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create charge: %w", err)
	}

	// Update payment record dengan transaction_id dari gateway
	if resp.TransactionID != "" {
		rawResp := map[string]any{
			"snap_token":    resp.Token,
			"redirect_url":  resp.RedirectURL,
			"transaction_id": resp.TransactionID,
		}
		uc.paymentRepo.UpdateStatus(ctx, pmnt.ID, domain.PaymentStatusPending, rawResp)
	}

	return &InitiateResult{
		SnapToken:   resp.Token,
		RedirectURL: resp.RedirectURL,
	}, nil
}

// HandleWebhook menerima notifikasi dari Midtrans dan update status order
func (uc *useCase) HandleWebhook(ctx context.Context, payload []byte) error {
	webhookData, err := uc.gateway.ParseWebhook(payload)
	if err != nil {
		return fmt.Errorf("parse webhook: %w", err)
	}

	// Verifikasi signature — keamanan pertama
	if !uc.gateway.VerifyWebhookSignature(webhookData.RawPayload) {
		return ErrInvalidWebhook
	}

	// Cari payment berdasarkan transaction_id dari Midtrans
	pmnt, err := uc.paymentRepo.FindByTransactionID(ctx, webhookData.TransactionID)
	if err != nil || pmnt == nil {
		return fmt.Errorf("payment not found for tx %s", webhookData.TransactionID)
	}

	// Map status Midtrans ke internal status
	newPaymentStatus := mapMidtransStatus(webhookData.TransactionStatus)

	// Update payment
	if err := uc.paymentRepo.UpdateStatus(ctx, pmnt.ID, newPaymentStatus, webhookData.RawPayload); err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}

	// Update order status berdasarkan payment status
	newOrderStatus := mapPaymentToOrderStatus(newPaymentStatus)
	if newOrderStatus != "" {
		uc.orderRepo.UpdateStatus(ctx, pmnt.OrderID, newOrderStatus)
	}

	// Notifikasi WA saat pembayaran berhasil
	if newPaymentStatus == domain.PaymentStatusSettlement {
		if order, err := uc.orderRepo.FindByID(ctx, pmnt.OrderID); err == nil && order != nil {
			if buyer, err := uc.userRepo.FindByID(ctx, order.UserID); err == nil && buyer != nil {
				uc.notify(buyer.Phone, notification.MsgPaymentConfirmed(pmnt.OrderID.String(), int64(pmnt.Amount)))
			}
			uc.notify(uc.adminPhone, notification.MsgPaymentReceivedAdmin(pmnt.OrderID.String(), int64(pmnt.Amount)))
		}
	}

	// Audit log
	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		Action:     domain.AuditUpdate,
		EntityType: "payments",
		EntityID:   &pmnt.ID,
		OldData:    map[string]any{"status": pmnt.Status},
		NewData:    map[string]any{"status": newPaymentStatus, "tx_id": webhookData.TransactionID},
		Notes:      "midtrans webhook",
		CreatedAt:  time.Now(),
	})

	return nil
}

func (uc *useCase) GetByOrderID(ctx context.Context, orderID, userID uuid.UUID) (*domain.Payment, error) {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}
	return uc.paymentRepo.FindByOrderID(ctx, orderID)
}

// mapMidtransStatus konversi status string Midtrans ke domain type
func mapMidtransStatus(midtransStatus string) domain.PaymentStatus {
	switch midtransStatus {
	case "settlement", "capture":
		return domain.PaymentStatusSettlement
	case "cancel":
		return domain.PaymentStatusCancel
	case "expire":
		return domain.PaymentStatusExpire
	case "refund":
		return domain.PaymentStatusRefund
	default:
		return domain.PaymentStatusPending
	}
}

// mapPaymentToOrderStatus update order saat payment berhasil/gagal
func mapPaymentToOrderStatus(ps domain.PaymentStatus) domain.OrderStatus {
	switch ps {
	case domain.PaymentStatusSettlement:
		return domain.OrderStatusPaid
	case domain.PaymentStatusCancel, domain.PaymentStatusExpire:
		return domain.OrderStatusCancelled
	default:
		return ""
	}
}
