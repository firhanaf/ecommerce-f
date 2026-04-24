package payment

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ─── Interface ───────────────────────────────────────────────────────────────

type CreateChargeRequest struct {
	OrderID       string
	Amount        float64
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Items         []ChargeItem
	ExpiredAt     time.Time
}

type ChargeItem struct {
	Name     string
	Price    float64
	Quantity int
}

type CreateChargeResponse struct {
	TransactionID string
	RedirectURL   string
	Token         string
	ExpiredAt     time.Time
	RawResponse   map[string]any
}

type WebhookPayload struct {
	TransactionID     string
	OrderID           string
	TransactionStatus string
	PaymentType       string
	RawPayload        map[string]any
}

type PaymentGateway interface {
	CreateCharge(ctx context.Context, req CreateChargeRequest) (*CreateChargeResponse, error)
	ParseWebhook(payload []byte) (*WebhookPayload, error)
	VerifyWebhookSignature(payload map[string]any) bool
}

// ─── Midtrans Implementation ─────────────────────────────────────────────────

type midtransGateway struct {
	serverKey  string
	production bool
	baseURL    string
	httpClient *http.Client
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
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateCharge membuat transaksi Snap di Midtrans dan mengembalikan snap_token
func (m *midtransGateway) CreateCharge(ctx context.Context, req CreateChargeRequest) (*CreateChargeResponse, error) {
	items := make([]map[string]any, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, map[string]any{
			"id":       item.Name,
			"name":     item.Name,
			"price":    int64(item.Price),
			"quantity": item.Quantity,
		})
	}

	payload := map[string]any{
		"transaction_details": map[string]any{
			"order_id":     req.OrderID,
			"gross_amount": int64(req.Amount),
		},
		"item_details": items,
		"expiry": map[string]any{
			"duration": 24,
			"unit":     "hours",
		},
	}

	if req.CustomerName != "" || req.CustomerEmail != "" {
		payload["customer_details"] = map[string]any{
			"first_name": req.CustomerName,
			"email":      req.CustomerEmail,
			"phone":      req.CustomerPhone,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("midtrans: marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.baseURL+"/snap/v1/transactions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("midtrans: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(m.serverKey, "")

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("midtrans: send request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirect_url"`
		// Error response
		ErrorMessages []string `json:"error_messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("midtrans: decode response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("midtrans: unexpected status %d: %v", resp.StatusCode, result.ErrorMessages)
	}

	return &CreateChargeResponse{
		Token:       result.Token,
		RedirectURL: result.RedirectURL,
	}, nil
}

// ParseWebhook mem-parse payload JSON dari Midtrans notification
func (m *midtransGateway) ParseWebhook(payload []byte) (*WebhookPayload, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("midtrans: parse webhook: %w", err)
	}

	getString := func(key string) string {
		if v, ok := raw[key].(string); ok {
			return v
		}
		return ""
	}

	return &WebhookPayload{
		TransactionID:     getString("transaction_id"),
		OrderID:           getString("order_id"),
		TransactionStatus: getString("transaction_status"),
		PaymentType:       getString("payment_type"),
		RawPayload:        raw,
	}, nil
}

// VerifyWebhookSignature memverifikasi signature dari Midtrans
// Formula: SHA512(order_id + status_code + gross_amount + server_key)
func (m *midtransGateway) VerifyWebhookSignature(payload map[string]any) bool {
	getString := func(key string) string {
		if v, ok := payload[key].(string); ok {
			return v
		}
		return ""
	}

	orderID := getString("order_id")
	statusCode := getString("status_code")
	grossAmount := getString("gross_amount")
	signatureKey := getString("signature_key")

	if signatureKey == "" {
		return false
	}

	raw := orderID + statusCode + grossAmount + m.serverKey
	hash := sha512.Sum512([]byte(raw))
	expected := hex.EncodeToString(hash[:])

	return expected == signatureKey
}
