package handler

import (
	"encoding/json"
	"net/http"

	"ecommerce-api/internal/repository"
	orderUC "ecommerce-api/internal/usecase/order"
	"ecommerce-api/pkg/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminHandler struct {
	userRepo  repository.UserRepository
	auditRepo repository.AuditLogRepository
	orderUC   orderUC.UseCase
}

func NewAdminHandler(
	userRepo repository.UserRepository,
	auditRepo repository.AuditLogRepository,
	orderUC orderUC.UseCase,
) *AdminHandler {
	return &AdminHandler{
		userRepo:  userRepo,
		auditRepo: auditRepo,
		orderUC:   orderUC,
	}
}

// GET /api/v1/admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	users, total, err := h.userRepo.FindAll(r.Context(), page, limit)
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]map[string]any, 0, len(users))
	for _, u := range users {
		result = append(result, toUserResponse(&u))
	}

	totalPages := (total + limit - 1) / limit
	response.OKWithMeta(w, "Berhasil mendapatkan daftar user", result, response.Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// PUT /api/v1/admin/users/{id}/status
func (h *AdminHandler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid user id")
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	// Pastikan user ada dulu
	user, err := h.userRepo.FindByID(r.Context(), id)
	if err != nil || user == nil {
		response.NotFound(w, "user")
		return
	}

	if err := h.userRepo.UpdateStatus(r.Context(), id, req.IsActive); err != nil {
		response.InternalError(w)
		return
	}

	response.OK(w, "Status user berhasil diperbarui", nil)
}

// GET /api/v1/admin/audit-logs
func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	filter := repository.AuditFilter{
		Page:  page,
		Limit: limit,
	}

	if et := r.URL.Query().Get("entity_type"); et != "" {
		filter.EntityType = et
	}
	if actorStr := r.URL.Query().Get("actor_id"); actorStr != "" {
		if id, err := uuid.Parse(actorStr); err == nil {
			filter.ActorID = &id
		}
	}

	logs, total, err := h.auditRepo.FindAll(r.Context(), filter)
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]map[string]any, 0, len(logs))
	for _, l := range logs {
		result = append(result, map[string]any{
			"id":          l.ID,
			"actor_id":    l.ActorID,
			"actor_role":  l.ActorRole,
			"action":      l.Action,
			"entity_type": l.EntityType,
			"entity_id":   l.EntityID,
			"old_data":    l.OldData,
			"new_data":    l.NewData,
			"ip_address":  l.IPAddress,
			"notes":       l.Notes,
			"created_at":  l.CreatedAt,
		})
	}

	totalPages := (total + limit - 1) / limit
	response.OKWithMeta(w, "Berhasil mendapatkan audit log", result, response.Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GET /api/v1/admin/orders
func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	filter := repository.OrderFilter{
		Page:  page,
		Limit: limit,
	}

	orders, total, err := h.orderUC.ListAll(r.Context(), filter)
	if err != nil {
		response.InternalError(w)
		return
	}

	totalPages := (total + limit - 1) / limit
	response.OKWithMeta(w, "Berhasil mendapatkan daftar order", toOrderListResponse(orders), response.Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

