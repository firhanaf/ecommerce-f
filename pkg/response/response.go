package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Meta    *Meta  `json:"meta,omitempty"`
}

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func JSON(w http.ResponseWriter, statusCode int, res Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(res)
}

func OK(w http.ResponseWriter, message string, data any) {
	JSON(w, http.StatusOK, Response{Code: http.StatusOK, Message: message, Data: data})
}

func OKWithMeta(w http.ResponseWriter, message string, data any, meta Meta) {
	JSON(w, http.StatusOK, Response{Code: http.StatusOK, Message: message, Data: data, Meta: &meta})
}

func Created(w http.ResponseWriter, message string, data any) {
	JSON(w, http.StatusCreated, Response{Code: http.StatusCreated, Message: message, Data: data})
}

func BadRequest(w http.ResponseWriter, message string) {
	JSON(w, http.StatusBadRequest, Response{Code: http.StatusBadRequest, Message: message, Data: nil})
}

func Unauthorized(w http.ResponseWriter) {
	JSON(w, http.StatusUnauthorized, Response{Code: http.StatusUnauthorized, Message: "unauthorized", Data: nil})
}

func Forbidden(w http.ResponseWriter) {
	JSON(w, http.StatusForbidden, Response{Code: http.StatusForbidden, Message: "forbidden", Data: nil})
}

func NotFound(w http.ResponseWriter, resource string) {
	JSON(w, http.StatusNotFound, Response{Code: http.StatusNotFound, Message: resource + " not found", Data: nil})
}

func Conflict(w http.ResponseWriter, message string) {
	JSON(w, http.StatusConflict, Response{Code: http.StatusConflict, Message: message, Data: nil})
}

func UnprocessableEntity(w http.ResponseWriter, message string) {
	JSON(w, http.StatusUnprocessableEntity, Response{Code: http.StatusUnprocessableEntity, Message: message, Data: nil})
}

func TooManyRequests(w http.ResponseWriter, message string) {
	JSON(w, http.StatusTooManyRequests, Response{Code: http.StatusTooManyRequests, Message: message, Data: nil})
}

func InternalError(w http.ResponseWriter) {
	JSON(w, http.StatusInternalServerError, Response{Code: http.StatusInternalServerError, Message: "internal server error", Data: nil})
}
