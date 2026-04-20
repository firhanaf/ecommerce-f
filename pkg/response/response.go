package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
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

func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Response{Success: true, Data: data})
}

func OKWithMeta(w http.ResponseWriter, data any, meta Meta) {
	JSON(w, http.StatusOK, Response{Success: true, Data: data, Meta: &meta})
}

func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, Response{Success: true, Data: data})
}

func BadRequest(w http.ResponseWriter, err string) {
	JSON(w, http.StatusBadRequest, Response{Success: false, Error: err})
}

func Unauthorized(w http.ResponseWriter) {
	JSON(w, http.StatusUnauthorized, Response{Success: false, Error: "unauthorized"})
}

func Forbidden(w http.ResponseWriter) {
	JSON(w, http.StatusForbidden, Response{Success: false, Error: "forbidden"})
}

func NotFound(w http.ResponseWriter, resource string) {
	JSON(w, http.StatusNotFound, Response{Success: false, Error: resource + " not found"})
}

func InternalError(w http.ResponseWriter) {
	JSON(w, http.StatusInternalServerError, Response{Success: false, Error: "internal server error"})
}

func Conflict(w http.ResponseWriter, err string) {
	JSON(w, http.StatusConflict, Response{Success: false, Error: err})
}
