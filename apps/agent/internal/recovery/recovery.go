package recovery

import (
	"context"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/ingestion"
)

type Summary struct {
	RecoveredRoutedPhotos    int `json:"recovered_routed_photos"`
	RecoveredQuarantineItems int `json:"recovered_quarantine_items"`
	SkippedDuplicates        int `json:"skipped_duplicates"`
	UnresolvedConflicts      int `json:"unresolved_conflicts"`
	Errors                   int `json:"errors"`
}

type Service struct {
	scanner       *ingestion.Scanner
	router        *ingestion.Router
	activityStore *activity.Store
	broker        *events.Broker
}

func NewService(scanner *ingestion.Scanner, router *ingestion.Router, activityStore *activity.Store, broker *events.Broker) Service {
	return Service{scanner: scanner, router: router, activityStore: activityStore, broker: broker}
}

func (s Service) Run(ctx context.Context, now time.Time) Summary {
	summary := Summary{}
	if s.scanner == nil || s.router == nil {
		s.record(summary)
		return summary
	}
	result, err := s.scanner.Scan(ctx)
	if err != nil {
		summary.Errors = len(result.Errors)
		if summary.Errors == 0 {
			summary.Errors = 1
		}
		s.record(summary)
		return summary
	}
	summary.Errors = len(result.Errors)
	for _, photo := range result.Photos {
		routed := s.router.Route(photo, now)
		if routed.Duplicate {
			summary.SkippedDuplicates++
			continue
		}
		switch routed.Status {
		case ingestion.PhotoStatus("routed"):
			summary.RecoveredRoutedPhotos++
			s.publish("photo.routed", "photo", routed.PhotoID, map[string]any{"photo_id": routed.PhotoID, "station_id": routed.StationID, "session_id": routed.SessionID, "status": routed.Status})
		case ingestion.PhotoStatus("quarantined"):
			summary.RecoveredQuarantineItems++
			s.publish("photo.quarantined", "quarantine", routed.QuarantineID, map[string]any{"quarantine_id": routed.QuarantineID, "station_id": routed.StationID, "related_session_id": routed.RelatedSessionID, "status": routed.Status, "reason": routed.Reason})
		default:
			summary.UnresolvedConflicts++
		}
	}
	s.record(summary)
	s.publish("ingestion.recovered", "ingestion", "startup", map[string]any{"recovered_routed_photos": summary.RecoveredRoutedPhotos, "recovered_quarantine_items": summary.RecoveredQuarantineItems, "skipped_duplicates": summary.SkippedDuplicates, "unresolved_conflicts": summary.UnresolvedConflicts, "errors": summary.Errors})
	return summary
}

func (s Service) record(summary Summary) {
	if s.activityStore == nil {
		return
	}
	msg := "Recovery ingestion selesai: routed, quarantine, duplicate, conflict, error dicatat untuk operator."
	s.activityStore.Record("ingestion.recovered", activity.ResultSuccess, msg)
}

func (s Service) publish(name string, entityType string, entityID string, data map[string]any) {
	if s.broker == nil || entityID == "" {
		return
	}
	if event, err := events.New(name, entityType, entityID, data); err == nil {
		s.broker.Publish(event)
	}
}
