package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response DataResponse[HealthData]
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Service != "selfstudio-agent" {
		t.Fatalf("service = %q, want selfstudio-agent", response.Data.Service)
	}
	if response.Data.Status != "ok" {
		t.Fatalf("status = %q, want ok", response.Data.Status)
	}
	if response.Data.Database.Status != "placeholder" {
		t.Fatalf("database status = %q, want placeholder", response.Data.Database.Status)
	}
	if response.Data.Worker.Label == "" || response.Data.Worker.Action == "" {
		t.Fatalf("worker status missing label/action: %+v", response.Data.Worker)
	}
	if response.Data.Disk.Label == "" || response.Data.Disk.Action == "" {
		t.Fatalf("disk status missing label/action: %+v", response.Data.Disk)
	}
}
