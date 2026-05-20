package processing

import (
	"sort"
	"strings"
	"time"

	"selfstudio/agent/internal/photos"
)

type QueueFilter struct {
	StationID string
	SessionID string
	Status    string
	Limit     int
}

type QueueStatus struct {
	Summary QueueSummary `json:"summary"`
	Items   []QueueItem  `json:"items"`
}

type QueueSummary struct {
	Total         int        `json:"total"`
	NotEligible   int        `json:"not_eligible"`
	Pending       int        `json:"pending"`
	Processing    int        `json:"processing"`
	Processed     int        `json:"processed"`
	Failed        int        `json:"failed"`
	Retrying      int        `json:"retrying"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
	CurrentJob    *QueueItem `json:"current_job,omitempty"`
}

type QueueItem struct {
	PhotoID                   string     `json:"photo_id"`
	StationID                 string     `json:"station_id"`
	SessionID                 string     `json:"session_id"`
	SourcePath                string     `json:"source_path"`
	LocalOriginalPath         string     `json:"local_original_path,omitempty"`
	LocalGradedPath           string     `json:"local_graded_path,omitempty"`
	OriginalSaveStatus        string     `json:"original_save_status"`
	ProcessingStatus          string     `json:"processing_status"`
	GradedProcessingStatus    string     `json:"graded_processing_status"`
	GradedLastError           string     `json:"graded_last_error,omitempty"`
	GradedAttemptCount        int        `json:"graded_attempt_count"`
	GradedProcessingStartedAt *time.Time `json:"graded_processing_started_at,omitempty"`
	GradedProcessedAt         *time.Time `json:"graded_processed_at,omitempty"`
	LastUpdatedAt             time.Time  `json:"last_updated_at"`
}

func BuildQueueStatus(store *photos.Store, filter QueueFilter) QueueStatus {
	if store == nil {
		return QueueStatus{Items: []QueueItem{}}
	}
	records := store.ListAll()
	items := make([]QueueItem, 0, len(records))
	for _, photo := range records {
		item := QueueItemFromPhoto(photo)
		if !matchesQueueFilter(item, filter) {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LastUpdatedAt.After(items[j].LastUpdatedAt)
	})
	summary := summarizeQueue(items)
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return QueueStatus{Summary: summary, Items: items}
}

func QueueItemFromPhoto(photo photos.Photo) QueueItem {
	return QueueItem{
		PhotoID:                   photo.PhotoID,
		StationID:                 photo.StationID,
		SessionID:                 photo.SessionID,
		SourcePath:                photo.SourcePath,
		LocalOriginalPath:         photo.LocalOriginalPath,
		LocalGradedPath:           photo.LocalGradedPath,
		OriginalSaveStatus:        photo.OriginalSaveStatus,
		ProcessingStatus:          photo.ProcessingStatus,
		GradedProcessingStatus:    photo.GradedProcessingStatus,
		GradedLastError:           photo.GradedLastError,
		GradedAttemptCount:        photo.GradedAttemptCount,
		GradedProcessingStartedAt: cloneTimePtr(photo.GradedProcessingStartedAt),
		GradedProcessedAt:         cloneTimePtr(photo.GradedProcessedAt),
		LastUpdatedAt:             queueLastUpdatedAt(photo),
	}
}

func matchesQueueFilter(item QueueItem, filter QueueFilter) bool {
	if filter.StationID != "" && item.StationID != filter.StationID {
		return false
	}
	if filter.SessionID != "" && item.SessionID != filter.SessionID {
		return false
	}
	if filter.Status != "" && queueBucket(item) != filter.Status {
		return false
	}
	return true
}

func summarizeQueue(items []QueueItem) QueueSummary {
	summary := QueueSummary{Total: len(items)}
	for index := range items {
		item := items[index]
		switch queueBucket(item) {
		case photos.GradedStatusNotEligible:
			summary.NotEligible++
		case photos.GradedStatusPending:
			summary.Pending++
		case photos.GradedStatusProcessing:
			summary.Processing++
			if summary.CurrentJob == nil {
				copy := item
				summary.CurrentJob = &copy
			}
		case photos.GradedStatusProcessed:
			summary.Processed++
		case photos.GradedStatusFailed:
			summary.Failed++
		case "retrying":
			summary.Retrying++
		}
		if summary.LastUpdatedAt == nil || item.LastUpdatedAt.After(*summary.LastUpdatedAt) {
			updated := item.LastUpdatedAt
			summary.LastUpdatedAt = &updated
		}
	}
	return summary
}

func queueBucket(item QueueItem) string {
	status := strings.TrimSpace(item.GradedProcessingStatus)
	if status == "" {
		status = photos.GradedStatusNotEligible
	}
	return status
}

func queueLastUpdatedAt(photo photos.Photo) time.Time {
	candidates := []time.Time{}
	if photo.GradedProcessedAt != nil {
		candidates = append(candidates, photo.GradedProcessedAt.UTC())
	}
	if photo.GradedProcessingStartedAt != nil {
		candidates = append(candidates, photo.GradedProcessingStartedAt.UTC())
	}
	if photo.OriginalSavedAt != nil {
		candidates = append(candidates, photo.OriginalSavedAt.UTC())
	}
	if photo.OriginalSaveStartedAt != nil {
		candidates = append(candidates, photo.OriginalSaveStartedAt.UTC())
	}
	candidates = append(candidates, photo.RoutedAt.UTC(), photo.StableAt.UTC(), photo.DetectedAt.UTC())
	latest := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.After(latest) {
			latest = candidate
		}
	}
	return latest
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
