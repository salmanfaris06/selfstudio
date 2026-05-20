package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
)

func TestProcessingQueueEndpointReturnsDataWrapperAndFilters(t *testing.T) {
	store := photos.NewStore()
	base := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	if err := store.ReplaceAll([]photos.Photo{
		apiQueuePhoto("p1", "station-a", "session-1", photos.GradedStatusPending, base),
		apiQueuePhoto("p2", "station-a", "session-1", photos.GradedStatusFailed, base.Add(time.Minute)),
		apiQueuePhoto("p3", "station-b", "session-2", photos.GradedStatusProcessed, base.Add(2*time.Minute)),
	}); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}
	handler := NewProcessingQueueHandler(store)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/processing/queue?station_id=station-a&status=failed&limit=5", nil)

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response DataResponse[processing.QueueStatus]
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Summary.Total != 1 || response.Data.Summary.Failed != 1 || len(response.Data.Items) != 1 || response.Data.Items[0].PhotoID != "p2" {
		t.Fatalf("unexpected queue response: %+v", response.Data)
	}
}

func TestProcessingQueueEndpointSummaryIgnoresLimit(t *testing.T) {
	store := photos.NewStore()
	base := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	if err := store.ReplaceAll([]photos.Photo{
		apiQueuePhoto("p1", "station-a", "session-1", photos.GradedStatusPending, base),
		apiQueuePhoto("p2", "station-a", "session-1", photos.GradedStatusProcessing, base.Add(time.Minute)),
		apiQueuePhoto("p3", "station-a", "session-1", photos.GradedStatusProcessed, base.Add(2*time.Minute)),
		apiQueuePhoto("p4", "station-a", "session-1", photos.GradedStatusFailed, base.Add(3*time.Minute)),
	}); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}
	handler := NewProcessingQueueHandler(store)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/processing/queue?station_id=station-a&limit=2", nil)

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response DataResponse[processing.QueueStatus]
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.Items) != 2 {
		t.Fatalf("expected limited items, got %d", len(response.Data.Items))
	}
	if response.Data.Summary.Total != 4 || response.Data.Summary.Pending != 1 || response.Data.Summary.Processing != 1 || response.Data.Summary.Processed != 1 || response.Data.Summary.Failed != 1 {
		t.Fatalf("summary should count all matching photos before limit, got %+v", response.Data.Summary)
	}
}

func TestProcessingQueueEndpointRejectsInvalidFiltersWithErrorShape(t *testing.T) {
	handler := NewProcessingQueueHandler(photos.NewStore())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/processing/queue?station_id=../secret&limit=0", nil)

	handler.Get(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var response ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if response.Error.Code != "INVALID_PROCESSING_QUEUE_FILTER" || response.Error.Message == "" || response.Error.Action == "" || response.Error.Details == nil {
		t.Fatalf("unexpected error shape: %+v", response.Error)
	}
}

func apiQueuePhoto(id, stationID, sessionID, gradedStatus string, routedAt time.Time) photos.Photo {
	originalSavedAt := routedAt.Add(5 * time.Second)
	startedAt := routedAt.Add(10 * time.Second)
	processedAt := routedAt.Add(15 * time.Second)
	photo := photos.Photo{PhotoID: id, StationID: stationID, SessionID: sessionID, SourcePath: "C:/input/" + id + ".jpg", SourceSizeBytes: int64(len(id)), DetectedAt: routedAt, StableAt: routedAt, RoutedAt: routedAt, Status: photos.StatusRouted, LocalOriginalPath: "C:/output/original/" + id + ".jpg", OriginalSaveStatus: photos.OriginalStatusSaved, ProcessingStatus: photos.ProcessingStatusEligible, GradedProcessingStatus: gradedStatus, GradedAttemptCount: 1, OriginalSavedAt: &originalSavedAt}
	if gradedStatus == photos.GradedStatusProcessing || gradedStatus == photos.GradedStatusFailed || gradedStatus == photos.GradedStatusProcessed {
		photo.GradedProcessingStartedAt = &startedAt
	}
	if gradedStatus == photos.GradedStatusProcessed {
		photo.LocalGradedPath = "C:/output/graded/" + id + ".jpg"
		photo.GradedProcessedAt = &processedAt
	}
	if gradedStatus == photos.GradedStatusFailed {
		photo.GradedLastError = "LUT_PROCESSING_FAILED"
	}
	return photo
}
