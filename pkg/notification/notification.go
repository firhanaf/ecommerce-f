package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	pkgotp "ecommerce-api/pkg/otp"
)

// Sender mengirim pesan WA ke nomor tujuan.
// Implementasi bisa diganti (Twilio, Zenziva, dll) tanpa ubah usecase.
type Sender interface {
	Send(ctx context.Context, phone, message string) error
}

// ─── Fonnte Implementation ────────────────────────────────────────────────────

type fonnteNotifier struct {
	token      string
	httpClient *http.Client
}

func NewFonnteNotifier(token string) Sender {
	return &fonnteNotifier{
		token:      token,
		httpClient: &http.Client{},
	}
}

func (f *fonnteNotifier) Send(ctx context.Context, phone, message string) error {
	phone = pkgotp.NormalizePhone(phone)

	formData := url.Values{}
	formData.Set("target", phone)
	formData.Set("message", message)
	formData.Set("countryCode", "62")

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.fonnte.com/send",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return fmt.Errorf("notification: build request: %w", err)
	}
	req.Header.Set("Authorization", f.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notification: send request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status bool   `json:"status"`
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("notification: decode response: %w", err)
	}
	if !result.Status {
		return fmt.Errorf("notification: send failed: %s %s", result.Reason, result.Detail)
	}

	return nil
}

// ─── Message Templates ────────────────────────────────────────────────────────

func MsgOrderCreated(orderID string, total int64) string {
	return fmt.Sprintf(
		"✅ *Pesanan Berhasil Dibuat*\n\nID Pesanan: #%s\nTotal: Rp%s\n\nSilakan lakukan pembayaran untuk memproses pesananmu.",
		shortID(orderID), formatRupiah(total),
	)
}

func MsgPaymentConfirmed(orderID string, total int64) string {
	return fmt.Sprintf(
		"💰 *Pembayaran Diterima*\n\nID Pesanan: #%s\nTotal: Rp%s\n\nPesananmu sedang kami proses. Terima kasih!",
		shortID(orderID), formatRupiah(total),
	)
}

func MsgOrderShipped(orderID, courier, trackingNumber string) string {
	return fmt.Sprintf(
		"🚚 *Pesananmu Sedang Dikirim*\n\nID Pesanan: #%s\nKurir: %s\nNomor Resi: *%s*\n\nKamu bisa cek status pengiriman di website kurir.",
		shortID(orderID), courier, trackingNumber,
	)
}

func MsgOrderDelivered(orderID string) string {
	return fmt.Sprintf(
		"📦 *Pesanan Telah Diterima*\n\nID Pesanan: #%s\n\nPesananmu telah sampai. Terima kasih sudah berbelanja! 🙏",
		shortID(orderID),
	)
}

func MsgNewOrderAdmin(buyerName, orderID string, total int64) string {
	return fmt.Sprintf(
		"🛍️ *Pesanan Baru Masuk*\n\nDari: %s\nID Pesanan: #%s\nTotal: Rp%s\n\nSegera proses pesanan ini.",
		buyerName, shortID(orderID), formatRupiah(total),
	)
}

func MsgPaymentReceivedAdmin(orderID string, total int64) string {
	return fmt.Sprintf(
		"💳 *Pembayaran Diterima*\n\nID Pesanan: #%s\nTotal: Rp%s\n\nPesanan siap diproses.",
		shortID(orderID), formatRupiah(total),
	)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// shortID ambil 8 karakter pertama UUID untuk tampilan lebih ringkas
func shortID(id string) string {
	if len(id) >= 8 {
		return strings.ToUpper(id[:8])
	}
	return strings.ToUpper(id)
}

func formatRupiah(amount int64) string {
	s := fmt.Sprintf("%d", amount)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += "."
		}
		result += string(c)
	}
	return result
}
