package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	domain "ecommerce-api/internal"
	"ecommerce-api/internal/middleware"
	productUC "ecommerce-api/internal/usecase/product"
	"ecommerce-api/pkg/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ProductHandler struct {
	productUC productUC.UseCase
}

func NewProductHandler(productUC productUC.UseCase) *ProductHandler {
	return &ProductHandler{productUC: productUC}
}

// GET /api/v1/products
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := productUC.ListProductFilter{
		Search: r.URL.Query().Get("search"),
		Page:   1,
		Limit:  20,
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			filter.Page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			filter.Limit = v
		}
	}
	if cat := r.URL.Query().Get("category_id"); cat != "" {
		if id, err := uuid.Parse(cat); err == nil {
			filter.CategoryID = &id
		}
	}

	// Public endpoint: default hanya tampilkan produk aktif
	isActive := true
	filter.IsActive = &isActive

	products, total, err := h.productUC.ListAll(r.Context(), filter)
	if err != nil {
		response.InternalError(w)
		return
	}

	totalPages := (total + filter.Limit - 1) / filter.Limit
	response.OKWithMeta(w, toProductListResponse(products), response.Meta{
		Page:       filter.Page,
		Limit:      filter.Limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GET /api/v1/products/{slug}
func (h *ProductHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	product, err := h.productUC.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, productUC.ErrProductNotFound) {
			response.NotFound(w, "product")
			return
		}
		response.InternalError(w)
		return
	}

	response.OK(w, toProductResponse(product))
}

// POST /api/v1/seller/products
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req productUC.CreateProductInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	// Ambil actor ID dari context untuk audit
	actorID, _ := middleware.GetUserIDFromContext(r.Context())
	_ = actorID

	product, err := h.productUC.CreateProduct(r.Context(), req)
	if err != nil {
		if errors.Is(err, productUC.ErrSlugAlreadyExists) {
			response.Conflict(w, err.Error())
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.Created(w, toProductResponse(product))
}

// PUT /api/v1/seller/products/{id}
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid product id")
		return
	}

	var req productUC.UpdateProductInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	product, err := h.productUC.UpdateByID(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, productUC.ErrProductNotFound) {
			response.NotFound(w, "product")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.OK(w, toProductResponse(product))
}

// DELETE /api/v1/seller/products/{id}
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid product id")
		return
	}

	if err := h.productUC.DeleteByID(r.Context(), id); err != nil {
		if errors.Is(err, productUC.ErrProductNotFound) {
			response.NotFound(w, "product")
			return
		}
		response.InternalError(w)
		return
	}

	response.OK(w, map[string]any{"message": "product deleted"})
}

// POST /api/v1/seller/products/{id}/images
// Multipart form: field "image" + optional field "is_primary" = "true"
func (h *ProductHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid product id")
		return
	}

	// Batas upload 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		response.BadRequest(w, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		response.BadRequest(w, "image field is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		response.InternalError(w)
		return
	}

	isPrimary := r.FormValue("is_primary") == "true"

	image, err := h.productUC.UploadImage(r.Context(), productUC.UploadImageInput{
		ProductID: productID,
		Filename:  header.Filename,
		Data:      data,
		Size:      header.Size,
		IsPrimary: isPrimary,
	})
	if err != nil {
		if errors.Is(err, productUC.ErrProductNotFound) {
			response.NotFound(w, "product")
			return
		}
		response.InternalError(w)
		return
	}

	response.Created(w, toProductImageResponse(image))
}

// ─── Response Mappers ─────────────────────────────────────────────────────────

func toProductResponse(p domain.Product) map[string]any {
	variants := make([]map[string]any, 0, len(p.Variants))
	for _, v := range p.Variants {
		variants = append(variants, map[string]any{
			"id":         v.ID,
			"name":       v.Name,
			"sku":        v.SKU,
			"price":      v.Price,
			"stock":      v.Stock,
			"is_active":  v.IsActive,
			"created_at": v.CreatedAt,
		})
	}

	images := make([]map[string]any, 0, len(p.Images))
	for _, img := range p.Images {
		images = append(images, toProductImageResponse(img))
	}

	return map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"slug":        p.Slug,
		"description": p.Description,
		"category_id": p.CategoryID,
		"is_active":   p.IsActive,
		"variants":    variants,
		"images":      images,
		"created_at":  p.CreatedAt,
		"updated_at":  p.UpdatedAt,
	}
}

func toProductListResponse(products []domain.Product) []map[string]any {
	result := make([]map[string]any, 0, len(products))
	for _, p := range products {
		result = append(result, toProductResponse(p))
	}
	return result
}

func toProductImageResponse(img domain.ProductImage) map[string]any {
	return map[string]any{
		"id":         img.ID,
		"product_id": img.ProductID,
		"url":        img.URL,
		"is_primary": img.IsPrimary,
		"sort_order": img.SortOrder,
		"created_at": img.CreatedAt,
	}
}
