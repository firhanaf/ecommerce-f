package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	domain "ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"ecommerce-api/pkg/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CategoryHandler struct {
	categoryRepo repository.CategoryRepository
}

func NewCategoryHandler(categoryRepo repository.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{categoryRepo: categoryRepo}
}

// GET /api/v1/categories
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	cats, err := h.categoryRepo.FindAll(r.Context())
	if err != nil {
		response.InternalError(w)
		return
	}

	result := make([]map[string]any, 0, len(cats))
	for _, c := range cats {
		result = append(result, toCategoryResponse(&c))
	}

	response.OK(w, "Berhasil mendapatkan daftar kategori", result)
}

// POST /api/v1/admin/categories
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Name == "" {
		response.BadRequest(w, "name is required")
		return
	}

	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		id, err := uuid.Parse(*req.ParentID)
		if err != nil {
			response.BadRequest(w, "invalid parent_id")
			return
		}
		parent, _ := h.categoryRepo.FindByID(r.Context(), id)
		if parent == nil {
			response.NotFound(w, "parent category")
			return
		}
		parentID = &id
	}

	cat := &domain.Category{
		ID:        uuid.New(),
		Name:      req.Name,
		Slug:      generateCategorySlug(req.Name),
		ParentID:  parentID,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if err := h.categoryRepo.Create(r.Context(), cat); err != nil {
		response.InternalError(w)
		return
	}

	response.Created(w, "Kategori berhasil dibuat", toCategoryResponse(cat))
}

// PUT /api/v1/admin/categories/{id}
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid category id")
		return
	}

	cat, err := h.categoryRepo.FindByID(r.Context(), id)
	if err != nil || cat == nil {
		response.NotFound(w, "category")
		return
	}

	var req struct {
		Name     *string `json:"name"`
		ParentID *string `json:"parent_id"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Name != nil && *req.Name != "" {
		cat.Name = *req.Name
		cat.Slug = generateCategorySlug(*req.Name)
	}
	if req.IsActive != nil {
		cat.IsActive = *req.IsActive
	}
	if req.ParentID != nil {
		if *req.ParentID == "" {
			cat.ParentID = nil
		} else {
			pid, err := uuid.Parse(*req.ParentID)
			if err != nil {
				response.BadRequest(w, "invalid parent_id")
				return
			}
			cat.ParentID = &pid
		}
	}

	if err := h.categoryRepo.Update(r.Context(), cat); err != nil {
		response.InternalError(w)
		return
	}

	response.OK(w, "Kategori berhasil diperbarui", toCategoryResponse(cat))
}

// DELETE /api/v1/admin/categories/{id}
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid category id")
		return
	}

	cat, err := h.categoryRepo.FindByID(r.Context(), id)
	if err != nil || cat == nil {
		response.NotFound(w, "category")
		return
	}

	if err := h.categoryRepo.Delete(r.Context(), id); err != nil {
		response.InternalError(w)
		return
	}

	response.OK(w, "Kategori berhasil dihapus", nil)
}

func toCategoryResponse(c *domain.Category) map[string]any {
	return map[string]any{
		"id":         c.ID,
		"name":       c.Name,
		"slug":       c.Slug,
		"parent_id":  c.ParentID,
		"is_active":  c.IsActive,
		"created_at": c.CreatedAt,
	}
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func generateCategorySlug(name string) string {
	slug := strings.ToLower(name)
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
