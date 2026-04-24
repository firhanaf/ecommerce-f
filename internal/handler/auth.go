package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	domain "ecommerce-api/internal"
	authUC "ecommerce-api/internal/usecase/auth"
	"ecommerce-api/pkg/response"
	"github.com/google/uuid"
)

type AuthHandler struct {
	authUC authUC.UseCase
}

func NewAuthHandler(authUC authUC.UseCase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Phone    string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Name == "" || req.Email == "" || len(req.Password) < 8 {
		response.BadRequest(w, "name, email, and password (min 8 chars) are required")
		return
	}
	if req.Phone == "" {
		response.BadRequest(w, "phone number is required")
		return
	}

	result, err := h.authUC.Register(r.Context(), authUC.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Phone:    req.Phone,
	})
	if err != nil {
		switch {
		case errors.Is(err, authUC.ErrEmailAlreadyExists):
			response.Conflict(w, err.Error())
		case errors.Is(err, authUC.ErrPhoneAlreadyExists):
			response.Conflict(w, err.Error())
		default:
			response.InternalError(w)
		}
		return
	}

	response.Created(w, "Registrasi berhasil, silakan kirim OTP untuk verifikasi nomor WhatsApp Anda", map[string]any{
		"user_id": result.UserID,
	})
}

// POST /api/v1/auth/verify-otp
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(w, "invalid user_id")
		return
	}

	if len(req.Code) != 6 {
		response.BadRequest(w, "OTP code must be 6 digits")
		return
	}

	result, err := h.authUC.VerifyOTP(r.Context(), userID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, authUC.ErrInvalidOTP):
			response.UnprocessableEntity(w, err.Error())
		case errors.Is(err, authUC.ErrOTPMaxAttempts):
			response.JSON(w, http.StatusTooManyRequests, response.Response{
				Code:    http.StatusTooManyRequests,
				Message: err.Error(),
				Data:    nil,
			})
		default:
			response.InternalError(w)
		}
		return
	}

	response.OK(w, "Verifikasi OTP berhasil", map[string]any{
		"user":          toUserResponse(result.User),
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
}

// POST /api/v1/auth/resend-otp
func (h *AuthHandler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(w, "invalid user_id")
		return
	}

	if err := h.authUC.ResendOTP(r.Context(), userID); err != nil {
		if errors.Is(err, authUC.ErrOTPRateLimited) {
			response.TooManyRequests(w, err.Error())
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.OK(w, "OTP berhasil dikirim ulang ke WhatsApp Anda", nil)
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"` // email atau nomor HP
		Password   string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Identifier == "" || req.Password == "" {
		response.BadRequest(w, "identifier and password are required")
		return
	}

	result, err := h.authUC.Login(r.Context(), authUC.LoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
	})
	if err != nil {
		if errors.Is(err, authUC.ErrPhoneNotVerified) {
			parts := strings.SplitN(err.Error(), ":", 2)
			userID := ""
			if len(parts) == 2 {
				userID = parts[1]
			}
			response.JSON(w, http.StatusForbidden, response.Response{
				Code:    http.StatusForbidden,
				Message: "Silakan verifikasi nomor WhatsApp Anda terlebih dahulu",
				Data: map[string]any{
					"user_id": userID,
					"reason":  "phone_not_verified",
				},
			})
			return
		}
		if errors.Is(err, authUC.ErrInvalidCredentials) || errors.Is(err, authUC.ErrUserInactive) {
			response.JSON(w, http.StatusUnauthorized, response.Response{
				Code:    http.StatusUnauthorized,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}
		response.InternalError(w)
		return
	}

	response.OK(w, "Login berhasil", map[string]any{
		"user":          toUserResponse(result.User),
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
}

// POST /api/v1/auth/forgot-password
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"` // email atau nomor HP
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}
	if req.Identifier == "" {
		response.BadRequest(w, "identifier (email or phone) is required")
		return
	}

	userID, err := h.authUC.ForgotPassword(r.Context(), req.Identifier)
	if err != nil {
		switch {
		case errors.Is(err, authUC.ErrUserNotFound):
			// Sengaja generic — jangan bocorkan apakah email/phone terdaftar
			response.OK(w, "Jika akun ditemukan, OTP akan dikirim ke WhatsApp terdaftar", nil)
		case errors.Is(err, authUC.ErrOTPRateLimited):
			response.TooManyRequests(w, "Permintaan reset password hanya bisa dilakukan 1x per 10 menit")
		case errors.Is(err, authUC.ErrUserInactive):
			response.JSON(w, http.StatusForbidden, response.Response{
				Code:    http.StatusForbidden,
				Message: "Akun tidak aktif",
			})
		default:
			response.InternalError(w)
		}
		return
	}

	response.OK(w, "OTP reset password telah dikirim ke WhatsApp Anda", map[string]any{
		"user_id": userID,
	})
}

// POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID      string `json:"user_id"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(w, "invalid user_id")
		return
	}
	if len(req.Code) != 6 {
		response.BadRequest(w, "OTP code must be 6 digits")
		return
	}
	if req.NewPassword == "" {
		response.BadRequest(w, "new_password is required")
		return
	}

	if err := h.authUC.ResetPassword(r.Context(), userID, req.Code, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, authUC.ErrInvalidOTP):
			response.UnprocessableEntity(w, err.Error())
		case errors.Is(err, authUC.ErrOTPMaxAttempts):
			response.TooManyRequests(w, err.Error())
		case errors.Is(err, authUC.ErrPasswordTooShort):
			response.BadRequest(w, err.Error())
		default:
			response.InternalError(w)
		}
		return
	}

	response.OK(w, "Password berhasil direset, silakan login dengan password baru", nil)
}

// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	result, err := h.authUC.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(w)
		return
	}

	response.OK(w, "Token berhasil diperbarui", map[string]any{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
}

// ─── Response Mapper ─────────────────────────────────────────────────────────

func toUserResponse(u *domain.User) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"name":           u.Name,
		"email":          u.Email,
		"role":           u.Role,
		"phone":          u.Phone,
		"phone_verified": u.PhoneVerified,
		"avatar_url":     u.AvatarURL,
		"is_active":      u.IsActive,
		"created_at":     u.CreatedAt,
	}
}
