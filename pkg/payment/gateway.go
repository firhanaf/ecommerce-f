package payment

import (
	"context"
	"fmt"
	"time"
)

// ─── Interface ───────────────────────────────────────────────────────────────
// Mau ganti ke Xendit, Stripe, atau gateway lain?
// Cukup buat struct baru yang implement PaymentGateway.

type CreateChargeRequest struct {
	OrderID     string
	Amount      float64
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Items       []ChargeItem
	ExpiredAt   time.Time
}

type ChargeItem struct {
	Name     string
	Price    float64
	Quantity int
}

type CreateChargeResponse struct {
	TransactionID string
	RedirectURL   string // URL untuk pembayaran (Midtrans Snap)
	Token         string // Snap token
	ExpiredAt     time.Time
	RawResponse   map[string]any
}

type WebhookPayload struct {
	TransactionID     string
	OrderID           string
	TransactionStatus string // "settlement", "cancel", "expire", dll
	PaymentType       string
	RawPayload        map[string]any
}

type PaymentGateway interface {
	CreateCharge(ctx context.Context, req CreateChargeRequest) (*CreateChargeResponse, error)
	ParseWebhook(payload []byte) (*WebhookPayload, error)
	VerifyWebhookSignature(payload map[string]any, signature string) bool
}

// ─── Midtrans Implementation ─────────────────────────────────────────────────
// Midtrans Snap API (pembayaran via popup atau redirect)

type midtransGateway struct {
	serverKey  string
	production bool
	baseURL    string
}

func NewMidtransGateway(serverKey string, production bool) PaymentGateway {
	baseURL := "https://app.sandbox.midtrans.com"
	if production {
		baseURL = "https://app.midtrans.com"
	}

	return &midtransGateway{
		serverKey:  serverKey,
		production: production,
		baseURL:    baseURL,
	}
}

func (m *midtransGateway) CreateCharge(ctx context.Context, req CreateChargeRequest) (*CreateChargeResponse, error) {
	// Catatan: di implementasi nyata, gunakan HTTP client untuk hit Midtrans Snap API
	// https://snap-docs.midtrans.com/#create-transaction
	//
	// Contoh payload:
	// {
	//   "transaction_details": { "order_id": req.OrderID, "gross_amount": req.Amount },
	//   "customer_details": { "first_name": req.CustomerName, "email": req.CustomerEmail },
	//   "item_details": [...],
	//   "expiry": { "duration": 24, "unit": "hours" }
	// }

	// Placeholder — implementasi HTTP call ke Midtrans
	_ = fmt.Sprintf("%s/snap/v1/transactions", m.baseURL)

	return &CreateChargeResponse{}, nil
}

func (m *midtransGateway) ParseWebhook(payload []byte) (*WebhookPayload, error) {
	// Parse JSON payload dari Midtrans webhook
	// Field penting: transaction_id, order_id, transaction_status, payment_type
	return &WebhookPayload{}, nil
}

func (m *midtransGateway) VerifyWebhookSignature(payload map[string]any, signature string) bool {
	// Midtrans signature: SHA512(order_id + status_code + gross_amount + server_key)
	// Verifikasi sebelum proses webhook untuk keamanan
	return true
}
