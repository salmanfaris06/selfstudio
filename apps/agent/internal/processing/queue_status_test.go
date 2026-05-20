package processing

import (
	"testing"
	"time"

	"selfstudio/agent/internal/photos"
)

func TestBuildQueueStatusSummarizesMixedPhotoStates(t *testing.T) {
	base := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := photos.NewStore()
	if err := store.ReplaceAll([]photos.Photo{
		photo("p-not", "station-a", "session-1", photos.OriginalStatusPending, photos.ProcessingStatusNotEligible, photos.GradedStatusNotEligible, base),
		photo("p-pending", "station-a", "session-1", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusPending, base.Add(time.Minute)),
		photo("p-processing", "station-a", "session-1", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusProcessing, base.Add(2*time.Minute)),
		photo("p-processed", "station-b", "session-2", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusProcessed, base.Add(3*time.Minute)),
		failedPhoto("p-failed", "station-b", "session-2", base.Add(4*time.Minute)),
	}); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}

	status := BuildQueueStatus(store, QueueFilter{})

	if status.Summary.Total != 5 || status.Summary.NotEligible != 1 || status.Summary.Pending != 1 || status.Summary.Processing != 1 || status.Summary.Processed != 1 || status.Summary.Failed != 1 || status.Summary.Retrying != 0 {
		t.Fatalf("unexpected summary: %+v", status.Summary)
	}
	if status.Summary.CurrentJob == nil || status.Summary.CurrentJob.PhotoID != "p-processing" {
		t.Fatalf("expected processing current job, got %+v", status.Summary.CurrentJob)
	}
	if status.Items[0].PhotoID != "p-failed" {
		t.Fatalf("items should be newest first, got %s", status.Items[0].PhotoID)
	}
	failed := status.Items[0]
	if failed.GradedLastError != "LUT_PROCESSING_FAILED" || failed.GradedAttemptCount != 2 {
		t.Fatalf("failed item missing error/attempt count: %+v", failed)
	}
}

func TestBuildQueueStatusFiltersAndLimits(t *testing.T) {
	base := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := photos.NewStore()
	if err := store.ReplaceAll([]photos.Photo{
		photo("p1", "station-a", "session-1", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusPending, base),
		photo("p2", "station-a", "session-2", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusProcessed, base.Add(time.Minute)),
		photo("p3", "station-b", "session-3", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusProcessed, base.Add(2*time.Minute)),
	}); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}

	status := BuildQueueStatus(store, QueueFilter{StationID: "station-a", Status: photos.GradedStatusProcessed, Limit: 1})
	if status.Summary.Total != 1 || len(status.Items) != 1 || status.Items[0].PhotoID != "p2" {
		t.Fatalf("unexpected filtered queue: %+v", status)
	}
}

func TestBuildQueueStatusSummaryIgnoresLimit(t *testing.T) {
	base := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := photos.NewStore()
	if err := store.ReplaceAll([]photos.Photo{
		photo("p-pending", "station-a", "session-1", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusPending, base),
		photo("p-processing", "station-a", "session-1", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusProcessing, base.Add(time.Minute)),
		photo("p-processed", "station-a", "session-1", photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusProcessed, base.Add(2*time.Minute)),
		failedPhoto("p-failed", "station-a", "session-1", base.Add(3*time.Minute)),
		photo("p-not", "station-a", "session-1", photos.OriginalStatusPending, photos.ProcessingStatusNotEligible, photos.GradedStatusNotEligible, base.Add(4*time.Minute)),
	}); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}

	status := BuildQueueStatus(store, QueueFilter{StationID: "station-a", Limit: 2})

	if len(status.Items) != 2 {
		t.Fatalf("expected items to honor limit, got %d", len(status.Items))
	}
	if status.Summary.Total != 5 || status.Summary.NotEligible != 1 || status.Summary.Pending != 1 || status.Summary.Processing != 1 || status.Summary.Processed != 1 || status.Summary.Failed != 1 {
		t.Fatalf("summary should count all filtered photos before limit, got %+v", status.Summary)
	}
	if status.Summary.CurrentJob == nil || status.Summary.CurrentJob.PhotoID != "p-processing" {
		t.Fatalf("current job should be derived from all filtered photos before limit, got %+v", status.Summary.CurrentJob)
	}
}

func TestQueueLastUpdatedAtPrefersProcessingTimestamps(t *testing.T) {
	routed := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	saved := routed.Add(time.Minute)
	started := routed.Add(2 * time.Minute)
	processed := routed.Add(3 * time.Minute)
	item := QueueItemFromPhoto(photos.Photo{PhotoID: "p1", StationID: "station-a", SessionID: "session-1", SourcePath: "C:/in/a.jpg", RoutedAt: routed, StableAt: routed, DetectedAt: routed, OriginalSavedAt: &saved, GradedProcessingStartedAt: &started, GradedProcessedAt: &processed})
	if !item.LastUpdatedAt.Equal(processed) {
		t.Fatalf("expected processed timestamp, got %s", item.LastUpdatedAt)
	}
}

func photo(id, stationID, sessionID, originalStatus, processingStatus, gradedStatus string, routedAt time.Time) photos.Photo {
	var originalSavedAt *time.Time
	var gradedStartedAt *time.Time
	var gradedProcessedAt *time.Time
	if originalStatus == photos.OriginalStatusSaved {
		saved := routedAt.Add(10 * time.Second)
		originalSavedAt = &saved
	}
	if gradedStatus == photos.GradedStatusProcessing || gradedStatus == photos.GradedStatusFailed || gradedStatus == photos.GradedStatusProcessed {
		started := routedAt.Add(20 * time.Second)
		gradedStartedAt = &started
	}
	if gradedStatus == photos.GradedStatusProcessed {
		processed := routedAt.Add(30 * time.Second)
		gradedProcessedAt = &processed
	}
	return photos.Photo{PhotoID: id, StationID: stationID, SessionID: sessionID, SourcePath: "C:/input/" + id + ".jpg", SourceSizeBytes: int64(len(id)), DetectedAt: routedAt, StableAt: routedAt, RoutedAt: routedAt, Status: photos.StatusRouted, LocalOriginalPath: "C:/output/original/" + id + ".jpg", LocalGradedPath: "C:/output/graded/" + id + ".jpg", OriginalSaveStatus: originalStatus, ProcessingStatus: processingStatus, GradedProcessingStatus: gradedStatus, OriginalSavedAt: originalSavedAt, GradedProcessingStartedAt: gradedStartedAt, GradedProcessedAt: gradedProcessedAt}
}

func failedPhoto(id, stationID, sessionID string, routedAt time.Time) photos.Photo {
	p := photo(id, stationID, sessionID, photos.OriginalStatusSaved, photos.ProcessingStatusEligible, photos.GradedStatusFailed, routedAt)
	p.GradedLastError = "LUT_PROCESSING_FAILED"
	p.GradedAttemptCount = 2
	return p
}
