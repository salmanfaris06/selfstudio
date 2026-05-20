package api

import (
	"context"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/ingestion"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
)

type processingRunner struct {
	processor *processing.GradedProcessor
	guard     *processing.ProcessingGuard
	activity  *activity.Store
	broker    *events.Broker
	backoff   time.Duration
}

func newProcessingRunner(processor *processing.GradedProcessor, guard *processing.ProcessingGuard, activityStore *activity.Store, broker *events.Broker) processingRunner {
	if guard == nil {
		guard = processing.NewProcessingGuard()
	}
	return processingRunner{processor: processor, guard: guard, activity: activityStore, broker: broker, backoff: 25 * time.Millisecond}
}

func EnqueueProcessing(processor *processing.GradedProcessor, guard *processing.ProcessingGuard, activityStore *activity.Store, broker *events.Broker, photoID string, manual bool) bool {
	return newProcessingRunner(processor, guard, activityStore, broker).enqueue(photoID, manual)
}

func (r processingRunner) enqueue(photoID string, manual bool) bool {
	if r.processor == nil || photoID == "" || !r.guard.TryStart(photoID) {
		return false
	}
	photo, _ := r.processor.Photos.Get(photoID)
	r.publishStarted(photo)
	go func() {
		defer r.guard.Done(photoID)
		r.runAttempts(context.Background(), photoID, manual)
	}()
	return true
}

func (r processingRunner) runAttempts(ctx context.Context, photoID string, manual bool) processing.GradedResult {
	result := r.processor.Process(ctx, photoID)
	r.publishTerminal(result.Photo, result.Err)
	for result.Err != nil && !manual {
		photo := result.Photo
		if photo.PhotoID == "" {
			break
		}
		if !processing.IsAutomaticRetryEligible(photo) {
			r.recordTerminalAutoFailure(photo)
			break
		}
		timer := time.NewTimer(r.backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result
		case <-timer.C:
		}
		r.publishStarted(photo)
		result = r.processor.Process(ctx, photoID)
		r.publishTerminal(result.Photo, result.Err)
	}
	return result
}

func (r processingRunner) publishStarted(photo photos.Photo) {
	if r.broker == nil || photo.PhotoID == "" {
		return
	}
	payload := map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": "processing", "graded_processing_status": "processing"}
	if event, err := events.New("photo.processing_started", "photo", photo.PhotoID, payload); err == nil {
		r.broker.Publish(event)
	}
	if event, err := events.New("queue.updated", "photo", photo.PhotoID, payload); err == nil {
		r.broker.Publish(event)
	}
}

func (r processingRunner) publishTerminal(photo photos.Photo, err error) {
	if photo.PhotoID == "" {
		return
	}
	if err != nil {
		if r.activity != nil {
			stationID := photo.StationID
			sessionID := photo.SessionID
			r.activity.RecordWithRefs("photo.processing_failed", activity.ResultFailure, "Graded JPG gagal dibuat: "+photo.GradedLastError, &stationID, &sessionID)
		}
		r.publishEvent("photo.processing_failed", photo)
		return
	}
	if r.activity != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		r.activity.RecordWithRefs("photo.processed", activity.ResultSuccess, "Graded JPG berhasil dibuat.", &stationID, &sessionID)
	}
	r.publishEvent("photo.processed", photo)
}

func (r processingRunner) publishEvent(name string, photo photos.Photo) {
	if r.broker == nil {
		return
	}
	payload := map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": photo.GradedProcessingStatus, "graded_processing_status": photo.GradedProcessingStatus}
	if event, err := events.New(name, "photo", photo.PhotoID, payload); err == nil {
		r.broker.Publish(event)
	}
	if event, err := events.New("queue.updated", "photo", photo.PhotoID, payload); err == nil {
		r.broker.Publish(event)
	}
}

func (r processingRunner) recordTerminalAutoFailure(photo photos.Photo) {
	if r.activity == nil || photo.PhotoID == "" {
		return
	}
	stationID := photo.StationID
	sessionID := photo.SessionID
	r.activity.RecordWithRefs("photo.processing_retry_exhausted", activity.ResultFailure, "Automatic retry processing mencapai batas. Perbaiki LUT/ImageMagick lalu retry manual.", &stationID, &sessionID)
}

func routeResultPhotoID(photo ingestion.RouteResult) string { return photo.PhotoID }
