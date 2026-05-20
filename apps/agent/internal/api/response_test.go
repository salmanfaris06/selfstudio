package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteAPIErrorShape(t *testing.T) {
	rec := httptest.NewRecorder()

	writeAPIError(rec, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi operator tidak aktif.", "Login ulang.")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("code = %q", response.Error.Code)
	}
	if response.Error.Message == "" || response.Error.Action == "" || response.Error.Details == nil {
		t.Fatalf("error response missing required fields: %+v", response.Error)
	}
}
