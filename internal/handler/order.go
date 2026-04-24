package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	domain "ecommerce-api/internal"
	"ecommerce-api/internal/middleware"
	"ecommerce-api/internal/repository"
	orderUC "ecommerce-api/internal/usecase/order"
	"ecommerce-api/pkg/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrderHandler struct {
	orderUC orderUC.UseCase
}

func NewOrderHandler(orderUC orderUC.UseCase) *OrderHandler {
	return &OrderHandler{orderUC: orderUC}
}

// POST /api/v1/orders — Checkout: buat order dari cart
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	var req struct {
		AddressID      string `json:"address_id"`
		Courier        string `json:"courier"`
		CourierService string `json:"courier_service"`
		Notes          string `json:"notes"`
		ShippingCost   int64  `json:"shipping_cost"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	addressID, err := uuid.Parse(req.AddressID)
	if err != nil {
		response.BadRequest(w, "invalid address_id")
		return
	}

	order, err := h.orderUC.Checkout(r.Context(), orderUC.CreateOrderInput{
		UserID:         userID,
		AddressID:      addressID,
		Courier:        req.Courier,
		CourierService: req.CourierService,
		Notes:          req.Notes,
		ShippingCost:   req.ShippingCost,
	})
	if err != nil {
		switch {
		case errors.Is(err, orderUC.ErrCartEmpty):
			response.BadRequest(w, err.Error())
		case errors.Is(err, orderUC.ErrAddressNotFound):
			response.NotFound(w, "address")
		case errors.Is(err, orderUC.ErrInsufficientStock):
			response.UnprocessableEntity(w, err.Error())
		default:
			response.InternalError(w)
		}
		return
	}

	response.Created(w, "Order berhasil dibuat", toOrderResponse(order))
}

// GET /api/v1/orders — Daftar order milik buyer
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	page, limit := parsePagination(r)
	orders, total, err := h.orderUC.ListByUser(r.Context(), userID, page, limit)
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

// GET /api/v1/orders/{id}
func (h *OrderHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid order id")
		return
	}

	order, err := h.orderUC.GetByID(r.Context(), orderID, userID)
	if err != nil {
		if errors.Is(err, orderUC.ErrOrderNotFound) {
			response.NotFound(w, "order")
			return
		}
		if errors.Is(err, orderUC.ErrUnauthorized) {
			response.Forbidden(w)
			return
		}
		response.InternalError(w)
		return
	}

	response.OK(w, "Berhasil mendapatkan detail order", toOrderResponse(order))
}

// GET /api/v1/seller/orders — Daftar semua order (untuk seller/admin)
func (h *OrderHandler) ListSeller(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	filter := repository.OrderFilter{
		Page:  page,
		Limit: limit,
	}

	if s := r.URL.Query().Get("status"); s != "" {
		status := domain.OrderStatus(s)
		filter.Status = &status
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

// PUT /api/v1/seller/orders/{id}/status
func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}
	actorRole, _ := middleware.GetRoleFromContext(r.Context())

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid order id")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.Status == "" {
		response.BadRequest(w, "status is required")
		return
	}

	if err := h.orderUC.UpdateStatus(
		r.Context(),
		orderID,
		domain.OrderStatus(req.Status),
		actorID,
		actorRole,
	); err != nil {
		if errors.Is(err, orderUC.ErrOrderNotFound) {
			response.NotFound(w, "order")
			return
		}
		response.InternalError(w)
		return
	}

	response.OK(w, "Status order berhasil diperbarui", nil)
}

// POST /api/v1/seller/orders/{id}/shipment
func (h *OrderHandler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Unauthorized(w)
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "invalid order id")
		return
	}

	var req struct {
		TrackingNumber string `json:"tracking_number"`
		Courier        string `json:"courier"`
		CourierService string `json:"courier_service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body")
		return
	}

	if req.TrackingNumber == "" {
		response.BadRequest(w, "tracking_number is required")
		return
	}

	shipment, err := h.orderUC.CreateShipment(r.Context(), orderUC.CreateShipmentInput{
		OrderID:        orderID,
		TrackingNumber: req.TrackingNumber,
		Courier:        req.Courier,
		CourierService: req.CourierService,
	}, actorID)
	if err != nil {
		if errors.Is(err, orderUC.ErrOrderNotFound) {
			response.NotFound(w, "order")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.Created(w, "Shipment berhasil dibuat", toShipmentResponse(shipment))
}

// ─── Response Mappers ─────────────────────────────────────────────────────────

func toOrderResponse(o *domain.Order) map[string]any {
	items := make([]map[string]any, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, map[string]any{
			"id":            item.ID,
			"variant_id":    item.VariantID,
			"product_name":  item.ProductName,
			"variant_name":  item.VariantName,
			"product_image": item.ProductImage,
			"quantity":      item.Quantity,
			"unit_price":    item.UnitPrice,
			"subtotal":      item.Subtotal,
		})
	}

	return map[string]any{
		"id":               o.ID,
		"user_id":          o.UserID,
		"status":           o.Status,
		"subtotal":         o.Subtotal,
		"shipping_cost":    o.ShippingCost,
		"total":            o.Total,
		"courier":          o.Courier,
		"courier_service":  o.CourierService,
		"notes":            o.Notes,
		"snapshot_address": o.SnapshotAddress,
		"items":            items,
		"created_at":       o.CreatedAt,
		"updated_at":       o.UpdatedAt,
	}
}

func toOrderListResponse(orders []domain.Order) []map[string]any {
	result := make([]map[string]any, 0, len(orders))
	for _, o := range orders {
		result = append(result, toOrderResponse(&o))
	}
	return result
}

func toShipmentResponse(s *domain.Shipment) map[string]any {
	return map[string]any{
		"id":              s.ID,
		"order_id":        s.OrderID,
		"courier":         s.Courier,
		"courier_service": s.CourierService,
		"tracking_number": s.TrackingNumber,
		"status":          s.Status,
		"shipped_at":      s.ShippedAt,
		"created_at":      s.CreatedAt,
	}
}

func parsePagination(r *http.Request) (page, limit int) {
	page, limit = 1, 20
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return
}
