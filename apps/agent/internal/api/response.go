package api

import (
	"encoding/json"
	"net/http"
)

type DataResponse[T any] struct {
	Data T `json:"data"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Action  string         `json:"action"`
	Details map[string]any `json:"details"`
}

func writeData[T any](w http.ResponseWriter, status int, data T) {
	writeJSON(w, status, DataResponse[T]{Data: data})
}

func writeAPIError(w http.ResponseWriter, status int, code string, message string, action string) {
	writeAPIErrorWithDetails(w, status, code, message, action, map[string]any{})
}

func writeAPIErrorWithDetails(w http.ResponseWriter, status int, code string, message string, action string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	writeJSON(w, status, ErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
			Action:  action,
			Details: details,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
