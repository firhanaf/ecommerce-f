package handler

import (
	"errors"
	"io"
	"net/http"

	"ecommerce-api/internal/middleware"
	paymentUC "ecommerce-api/internal/usecase/payment"
	"ecommerce-api/pkg/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	paymentUC paymentUC.UseCase
}

func NewPaymentHandler(paymentUC paymentUC.UseCase) *PaymentHandler {
	return &PaymentHandler{paymentUC: paymentUC}
}

// POST /api/v1/orders/{id}/pay — Inisiasi pembayaran, dapatkan Snap token
func (h *PaymentHandler) InitiateForOrder(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.paymentUC.InitiatePayment(r.Context(), orderID, userID)
	if err != nil {
		if errors.Is(err, paymentUC.ErrOrderNotFound) {
			response.NotFound(w, "order")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}

	response.OK(w, "Pembayaran berhasil diinisiasi", map[string]any{
		"snap_token":   result.SnapToken,
		"redirect_url": result.RedirectURL,
	})
}

// POST /api/v1/payments/webhook — Callback dari Midtrans
// Tidak perlu auth — Midtrans yang hit endpoint ini
func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.BadRequest(w, "failed to read request body")
		return
	}
	defer r.Body.Close()

	if err := h.paymentUC.HandleWebhook(r.Context(), body); err != nil {
		if errors.Is(err, paymentUC.ErrInvalidWebhook) {
			response.JSON(w, http.StatusForbidden, response.Response{
				Code:    http.StatusForbidden,
				Message: "invalid webhook signature",
				Data:    nil,
			})
			return
		}
		// Webhook error tidak boleh return 5xx karena Midtrans akan retry
		response.OK(w, "ignored", nil)
		return
	}

	response.OK(w, "ok", nil)
}
