package otp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ─── Interface ───────────────────────────────────────────────────────────────
// Mau ganti ke Twilio atau provider lain? Implement interface ini.

type Sender interface {
	SendOTP(ctx context.Context, phone, code string) error
}

// ─── Fonnte Implementation ────────────────────────────────────────────────────
// Fonnte adalah WhatsApp gateway lokal Indonesia dengan free tier
// Docs: https://fonnte.com/docs

type fonnteClient struct {
	token      string
	httpClient *http.Client
}

func NewFonnteClient(token string) Sender {
	return &fonnteClient{
		token:      token,
		httpClient: &http.Client{},
	}
}

func (f *fonnteClient) SendOTP(ctx context.Context, phone, code string) error {
	phone = NormalizePhone(phone)

	message := fmt.Sprintf(
		"Welcome to Floweys Project \n\nKode OTP Anda adalah: *%s*\n\nBerlaku 5 menit. Jangan bagikan kode ini ke siapapun.",
		code,
	)

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
		return fmt.Errorf("fonnte: build request: %w", err)
	}
	req.Header.Set("Authorization", f.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fonnte: send request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status  bool   `json:"status"`
		Process string `json:"process"`
		Reason  string `json:"reason"`
		Detail  string `json:"detail"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("fonnte: decode response: %w", err)
	}

	if !result.Status {
		return fmt.Errorf("fonnte: send failed: %s %s", result.Reason, result.Detail)
	}

	return nil
}

// NormalizePhone konversi berbagai format nomor HP Indonesia ke format Fonnte (62xxx)
// Contoh: "08123456789" → "628123456789"
//
//	"+628123456789" → "628123456789"
//	"8123456789" → "628123456789"
func NormalizePhone(phone string) string {
	// Hapus karakter non-digit kecuali leading +
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.TrimPrefix(phone, "+")

	// 08xxx → 628xxx
	if strings.HasPrefix(phone, "0") {
		return "62" + phone[1:]
	}

	// Sudah ada 62 prefix
	if strings.HasPrefix(phone, "62") {
		return phone
	}

	// 8xxx (tanpa 0 dan tanpa 62)
	return "62" + phone
}
