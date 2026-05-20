package ingestion

import (
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/sessions"
)

const PhotoUnassignedPendingQuarantine PhotoStatus = "unassigned_pending_quarantine"

type RouteResult struct {
	PhotoID                   string      `json:"photo_id,omitempty"`
	QuarantineID              string      `json:"quarantine_id,omitempty"`
	StationID                 string      `json:"station_id"`
	SessionID                 string      `json:"session_id,omitempty"`
	RelatedSessionID          string      `json:"related_session_id,omitempty"`
	SourcePath                string      `json:"source_path"`
	SourceSizeBytes           int64       `json:"source_size_bytes"`
	DetectedAt                time.Time   `json:"detected_at"`
	StableAt                  time.Time   `json:"stable_at"`
	RoutedAt                  *time.Time  `json:"routed_at,omitempty"`
	QuarantinedAt             *time.Time  `json:"quarantined_at,omitempty"`
	Reason                    string      `json:"reason,omitempty"`
	Status                    PhotoStatus `json:"status"`
	Duplicate                 bool        `json:"duplicate"`
	LocalOriginalPath         string      `json:"local_original_path,omitempty"`
	OriginalSaveStatus        string      `json:"original_save_status,omitempty"`
	LastError                 string      `json:"last_error,omitempty"`
	AttemptCount              int         `json:"attempt_count,omitempty"`
	OriginalSaveStartedAt     *time.Time  `json:"original_save_started_at,omitempty"`
	OriginalSavedAt           *time.Time  `json:"original_saved_at,omitempty"`
	ProcessingStatus          string      `json:"processing_status,omitempty"`
	LocalGradedPath           string      `json:"local_graded_path,omitempty"`
	GradedProcessingStatus    string      `json:"graded_processing_status,omitempty"`
	GradedLastError           string      `json:"graded_last_error,omitempty"`
	GradedAttemptCount        int         `json:"graded_attempt_count,omitempty"`
	GradedProcessingStartedAt *time.Time  `json:"graded_processing_started_at,omitempty"`
	GradedProcessedAt         *time.Time  `json:"graded_processed_at,omitempty"`
	LUTSnapshotPath           string      `json:"lut_snapshot_path,omitempty"`
}

type Router struct {
	sessions   *sessions.Store
	photos     *photos.Store
	quarantine *quarantine.Store
}

func NewRouter(sessionStore *sessions.Store, photoStore *photos.Store) *Router {
	return NewRouterWithQuarantine(sessionStore, photoStore, nil)
}

func NewRouterWithQuarantine(sessionStore *sessions.Store, photoStore *photos.Store, quarantineStore *quarantine.Store) *Router {
	return &Router{sessions: sessionStore, photos: photoStore, quarantine: quarantineStore}
}

func (r *Router) Route(photo DetectedPhoto, now time.Time) RouteResult {
	result := RouteResult{StationID: photo.StationID, SourcePath: photo.SourcePath, SourceSizeBytes: photo.SizeBytes, DetectedAt: photo.DetectedAt, StableAt: photo.StableAt, Status: PhotoUnassignedPendingQuarantine}
	if r == nil || r.sessions == nil || r.photos == nil {
		return result
	}
	now = now.UTC()
	session, ok := r.sessions.ActiveForStation(photo.StationID, now)
	if !ok {
		return r.quarantinePhoto(photo, now, result)
	}
	routedAt := now
	record := r.photos.Route(photo.StationID, session.SessionID, photo.SourcePath, photo.SizeBytes, photo.DetectedAt, photo.StableAt, routedAt)
	return RouteResultFromPhoto(record, &routedAt)
}

func RouteResultFromPhoto(record photos.Photo, routedAt *time.Time) RouteResult {
	return RouteResult{PhotoID: record.PhotoID, StationID: record.StationID, SessionID: record.SessionID, SourcePath: record.SourcePath, SourceSizeBytes: record.SourceSizeBytes, DetectedAt: record.DetectedAt, StableAt: record.StableAt, RoutedAt: routedAt, Status: PhotoStatus(record.Status), Duplicate: record.Duplicate, LocalOriginalPath: record.LocalOriginalPath, OriginalSaveStatus: record.OriginalSaveStatus, LastError: record.LastError, AttemptCount: record.AttemptCount, OriginalSaveStartedAt: record.OriginalSaveStartedAt, OriginalSavedAt: record.OriginalSavedAt, ProcessingStatus: record.ProcessingStatus, LocalGradedPath: record.LocalGradedPath, GradedProcessingStatus: record.GradedProcessingStatus, GradedLastError: record.GradedLastError, GradedAttemptCount: record.GradedAttemptCount, GradedProcessingStartedAt: record.GradedProcessingStartedAt, GradedProcessedAt: record.GradedProcessedAt, LUTSnapshotPath: record.LUTSnapshotPath}
}

func (r *Router) quarantinePhoto(photo DetectedPhoto, now time.Time, fallback RouteResult) RouteResult {
	if r.quarantine == nil {
		return fallback
	}
	reason := quarantine.ReasonNoActiveSession
	relatedSessionID := ""
	if last, ok := r.sessions.LastSessionForStation(photo.StationID, now); ok && last.Status == sessions.StatusLocked {
		boundary := last.EndsAt
		if last.EndedAt != nil {
			boundary = *last.EndedAt
		}
		if !boundary.IsZero() && !boundary.After(photo.DetectedAt.UTC()) {
			reason = quarantine.ReasonLatePhoto
			relatedSessionID = last.SessionID
		}
	}
	record := r.quarantine.Quarantine(photo.StationID, relatedSessionID, photo.SourcePath, photo.SizeBytes, photo.DetectedAt, photo.StableAt, now, reason)
	quarantinedAt := record.QuarantinedAt
	return RouteResult{QuarantineID: record.QuarantineID, StationID: record.StationID, RelatedSessionID: record.RelatedSessionID, SourcePath: record.SourcePath, SourceSizeBytes: record.SourceSizeBytes, DetectedAt: record.DetectedAt, StableAt: record.StableAt, QuarantinedAt: &quarantinedAt, Reason: record.Reason, Status: PhotoStatus(record.Status), Duplicate: record.Duplicate}
}
