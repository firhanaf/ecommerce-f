package middleware

import (
	"context"
	"net/http"
	"strings"

	"ecommerce-api/pkg/jwt"
	"ecommerce-api/pkg/response"
	"github.com/google/uuid"
)

// ─── Context Keys ─────────────────────────────────────────────────────────────

type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyRole   contextKey = "role"
)

// ─── Auth Middleware ─────────────────────────────────────────────────────────

func Auth(tokenSvc jwt.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				response.Unauthorized(w)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := tokenSvc.ValidateToken(tokenStr)
			if err != nil {
				response.Unauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ─── Role Guard ───────────────────────────────────────────────────────────────

// RequireRole memastikan hanya role tertentu yang boleh akses endpoint
// Contoh: RequireRole("admin", "seller")
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowedRoles := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowedRoles[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(ContextKeyRole).(string)
			if !ok || !allowedRoles[role] {
				response.Forbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Request Logger ───────────────────────────────────────────────────────────

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bisa diganti ke structured logger (slog, zap) tanpa ubah middleware lain
		// log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// ─── Recoverer ────────────────────────────────────────────────────────────────

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				response.InternalError(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─── Context Helpers ─────────────────────────────────────────────────────────

func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(uuid.UUID)
	return userID, ok
}

func GetRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(ContextKeyRole).(string)
	return role, ok
}
