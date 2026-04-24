package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	domain "ecommerce-api/internal"
	"ecommerce-api/internal/middleware"
	cartUC "ecommerce-api/internal/usecase/cart"
	"ecommerce-api/pkg/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CartHandler struct {
	cartUC cartUC.UseCase
}

func NewCartHandler(cartUC cartUC.UseCase) *CartHandler {
	return &CartHandler{cartUC: cartUC}
}

// GET /api/v1/cart
func (h *CartHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	cart, err := h.cartUC.GetCart(r.Context(), userID)
	if err != nil {
		response.InternalError(w)
		return
	}

	response.OK(w, "Berhasil mendapatkan cart", toCartResponse(cart))
}

// POST /api/v1/cart/items
func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	var req struct {
		VariantID string `json:"variant_id"`
		Quantity  int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	variantID, err := uuid.Parse(req.VariantID)
	if err != nil {
		response.BadRequest(w, "invalid variant_id")
		return
	}

	if req.Quantity <= 0 {
		response.BadRequest(w, "quantity must be greater than 0")
		return
	}

	if err := h.cartUC.AddItem(r.Context(), userID, variantID, req.Quantity); err != nil {
		switch {
		case errors.Is(err, cartUC.ErrVariantNotFound):
			response.NotFound(w, "variant")
		case errors.Is(err, cartUC.ErrVariantOutOfStock):
			response.UnprocessableEntity(w, err.Error())
		default:
			response.InternalError(w)
		}
		return
	}

	response.OK(w, "Item berhasil ditambahkan ke cart", nil)
}

// PUT /api/v1/cart/items/{itemID}
func (h *CartHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		response.BadRequest(w, "invalid item id")
		return
	}

	var req struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if err := h.cartUC.UpdateItem(r.Context(), userID, itemID, req.Quantity); err != nil {
		switch {
		case errors.Is(err, cartUC.ErrCartItemNotFound):
			response.NotFound(w, "cart item")
		case errors.Is(err, cartUC.ErrUnauthorized):
			response.Forbidden(w)
		default:
			response.InternalError(w)
		}
		return
	}

	response.OK(w, "Item cart berhasil diperbarui", nil)
}

// DELETE /api/v1/cart/items/{itemID}
func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		response.BadRequest(w, "invalid item id")
		return
	}

	if err := h.cartUC.RemoveItem(r.Context(), userID, itemID); err != nil {
		switch {
		case errors.Is(err, cartUC.ErrCartItemNotFound):
			response.NotFound(w, "cart item")
		case errors.Is(err, cartUC.ErrUnauthorized):
			response.Forbidden(w)
		default:
			response.InternalError(w)
		}
		return
	}

	response.OK(w, "Item berhasil dihapus dari cart", nil)
}

// ─── Response Mapper ─────────────────────────────────────────────────────────

func toCartResponse(cart *domain.Cart) map[string]any {
	items := make([]map[string]any, 0, len(cart.Items))
	var totalPrice int64

	for _, item := range cart.Items {
		itemData := map[string]any{
			"id":         item.ID,
			"variant_id": item.VariantID,
			"quantity":   item.Quantity,
			"updated_at": item.UpdatedAt,
		}
		if item.Variant != nil {
			itemData["variant"] = map[string]any{
				"id":    item.Variant.ID,
				"name":  item.Variant.Name,
				"price": item.Variant.Price,
				"stock": item.Variant.Stock,
			}
			totalPrice += item.Variant.Price * int64(item.Quantity)
		}
		if item.Product != nil {
			itemData["product"] = map[string]any{
				"id":   item.Product.ID,
				"name": item.Product.Name,
			}
		}
		items = append(items, itemData)
	}

	return map[string]any{
		"id":          cart.ID,
		"user_id":     cart.UserID,
		"items":       items,
		"total_price": totalPrice,
		"item_count":  len(cart.Items),
	}
}
