package response

import (
	"encoding/json"
	"math"
	"net/http"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorData  `json:"error,omitempty"`
}

type ErrorData struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type PaginatedResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Meta    *Meta       `json:"meta"`
}

type Meta struct {
	Total      int `json:"total"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
	TotalPages int `json:"total_pages"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := Response{
		Success: status >= 200 && status < 300,
		Data:    data,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := Response{
		Success: false,
		Error: &ErrorData{
			Code:    code,
			Message: message,
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func JSONWithPagination(w http.ResponseWriter, r *http.Request, status int, data interface{}, total, limit, offset int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	resp := PaginatedResponse{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Total:      total,
			Limit:      limit,
			Offset:     offset,
			TotalPages: totalPages,
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}
