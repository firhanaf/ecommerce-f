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
		if errors.Is(err, authUC.ErrEmailAlreadyExists) {
			response.Conflict(w, err.Error())
			return
		}
		response.InternalError(w)
		return
	}

	response.Created(w, map[string]any{
		"user_id": result.UserID,
		"message": result.Message,
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
		if errors.Is(err, authUC.ErrInvalidOTP) {
			response.JSON(w, http.StatusUnprocessableEntity, response.Response{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		response.InternalError(w)
		return
	}

	response.OK(w, map[string]any{
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
			response.JSON(w, http.StatusTooManyRequests, response.Response{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.OK(w, map[string]any{
		"message": "OTP berhasil dikirim ulang ke WhatsApp Anda",
	})
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	result, err := h.authUC.Login(r.Context(), authUC.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		// Cek apakah error phone_not_verified (format: "err:userID")
		if errors.Is(err, authUC.ErrPhoneNotVerified) {
			// Extract user_id dari error message untuk keperluan resend OTP di frontend
			parts := strings.SplitN(err.Error(), ":", 2)
			userID := ""
			if len(parts) == 2 {
				userID = parts[1]
			}
			response.JSON(w, http.StatusForbidden, response.Response{
				Success: false,
				Error:   "phone_not_verified",
				Data: map[string]any{
					"user_id": userID,
					"message": "Silakan verifikasi nomor WhatsApp Anda terlebih dahulu",
				},
			})
			return
		}
		if errors.Is(err, authUC.ErrInvalidCredentials) || errors.Is(err, authUC.ErrUserInactive) {
			response.JSON(w, http.StatusUnauthorized, response.Response{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		response.InternalError(w)
		return
	}

	response.OK(w, map[string]any{
		"user":          toUserResponse(result.User),
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
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

	response.OK(w, map[string]any{
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
