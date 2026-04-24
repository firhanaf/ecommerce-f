package handler

import (
	"encoding/json"
	"net/http"
	"time"

	domain "ecommerce-api/internal"
	"ecommerce-api/internal/middleware"
	"ecommerce-api/internal/repository"
	"ecommerce-api/pkg/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AddressHandler struct {
	addressRepo repository.AddressRepository
}

func NewAddressHandler(addressRepo repository.AddressRepository) *AddressHandler {
	return &AddressHandler{addressRepo: addressRepo}
}

// GET /api/v1/addresses
func (h *AddressHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	addrs, err := h.addressRepo.FindByUserID(r.Context(), userID)
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]map[string]any, 0, len(addrs))
	for _, a := range addrs {
		result = append(result, toAddressResponse(&a))
	}

	response.OK(w, "Berhasil mendapatkan daftar alamat", result)
}

// POST /api/v1/addresses
func (h *AddressHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	var req struct {
		RecipientName string `json:"recipient_name"`
		Phone         string `json:"phone"`
		Street        string `json:"street"`
		City          string `json:"city"`
		Province      string `json:"province"`
		PostalCode    string `json:"postal_code"`
		IsDefault     bool   `json:"is_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.RecipientName == "" || req.Phone == "" || req.Street == "" ||
		req.City == "" || req.Province == "" || req.PostalCode == "" {
		response.BadRequest(w, "semua field alamat wajib diisi")
		return
	}

	// Kalau ini alamat pertama atau diminta jadi default
	if req.IsDefault {
		h.addressRepo.SetDefault(r.Context(), userID, uuid.Nil)
	} else {
		existing, _ := h.addressRepo.FindByUserID(r.Context(), userID)
		if len(existing) == 0 {
			req.IsDefault = true
		}
	}

	now := time.Now()
	addr := &domain.Address{
		ID:            uuid.New(),
		UserID:        userID,
		RecipientName: req.RecipientName,
		Phone:         req.Phone,
		Street:        req.Street,
		City:          req.City,
		Province:      req.Province,
		PostalCode:    req.PostalCode,
		IsDefault:     req.IsDefault,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.addressRepo.Create(r.Context(), addr); err != nil {
		response.InternalError(w)
		return
	}

	if req.IsDefault {
		h.addressRepo.SetDefault(r.Context(), userID, addr.ID)
	}

	response.Created(w, "Alamat berhasil ditambahkan", toAddressResponse(addr))
}

// PUT /api/v1/addresses/{id}
func (h *AddressHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid address id")
		return
	}

	addr, err := h.addressRepo.FindByID(r.Context(), id)
	if err != nil || addr == nil {
		response.NotFound(w, "address")
		return
	}
	if addr.UserID != userID {
		response.Forbidden(w)
		return
	}

	var req struct {
		RecipientName string `json:"recipient_name"`
		Phone         string `json:"phone"`
		Street        string `json:"street"`
		City          string `json:"city"`
		Province      string `json:"province"`
		PostalCode    string `json:"postal_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.RecipientName != "" {
		addr.RecipientName = req.RecipientName
	}
	if req.Phone != "" {
		addr.Phone = req.Phone
	}
	if req.Street != "" {
		addr.Street = req.Street
	}
	if req.City != "" {
		addr.City = req.City
	}
	if req.Province != "" {
		addr.Province = req.Province
	}
	if req.PostalCode != "" {
		addr.PostalCode = req.PostalCode
	}

	if err := h.addressRepo.Update(r.Context(), addr); err != nil {
		response.InternalError(w)
		return
	}

	response.OK(w, "Alamat berhasil diperbarui", toAddressResponse(addr))
}

// DELETE /api/v1/addresses/{id}
func (h *AddressHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid address id")
		return
	}

	addr, err := h.addressRepo.FindByID(r.Context(), id)
	if err != nil || addr == nil {
		response.NotFound(w, "address")
		return
	}
	if addr.UserID != userID {
		response.Forbidden(w)
		return
	}

	if err := h.addressRepo.Delete(r.Context(), id); err != nil {
		response.InternalError(w)
		return
	}

	response.OK(w, "Alamat berhasil dihapus", nil)
}

// PUT /api/v1/addresses/{id}/default
func (h *AddressHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid address id")
		return
	}

	addr, err := h.addressRepo.FindByID(r.Context(), id)
	if err != nil || addr == nil {
		response.NotFound(w, "address")
		return
	}
	if addr.UserID != userID {
		response.Forbidden(w)
		return
	}

	if err := h.addressRepo.SetDefault(r.Context(), userID, id); err != nil {
		response.InternalError(w)
		return
	}

	response.OK(w, "Alamat default berhasil diubah", nil)
}

func toAddressResponse(a *domain.Address) map[string]any {
	return map[string]any{
		"id":             a.ID,
		"recipient_name": a.RecipientName,
		"phone":          a.Phone,
		"street":         a.Street,
		"city":           a.City,
		"province":       a.Province,
		"postal_code":    a.PostalCode,
		"is_default":     a.IsDefault,
		"created_at":     a.CreatedAt,
		"updated_at":     a.UpdatedAt,
	}
}
