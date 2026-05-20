package api

import (
	"net/http"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/ingestion"
	"selfstudio/agent/internal/processing"
)

type IngestionHandler struct {
	scanner         *ingestion.Scanner
	router          *ingestion.Router
	activityStore   *activity.Store
	broker          *events.Broker
	saveRoute       func() error
	rollbackRoute   func() error
	originalSaver   *processing.OriginalSaver
	gradedProcessor *processing.GradedProcessor
	processingGuard *processing.ProcessingGuard
}

type IngestionScanData struct {
	Photos            []ingestion.DetectedPhoto    `json:"photos"`
	RoutedPhotos      []ingestion.RouteResult      `json:"routed_photos"`
	QuarantinedPhotos []ingestion.RouteResult      `json:"quarantined_photos"`
	Errors            []ingestion.StationScanError `json:"errors"`
}

func NewIngestionHandler(scanner *ingestion.Scanner, activityStore *activity.Store, broker *events.Broker) IngestionHandler {
	return NewIngestionHandlerWithRouter(scanner, nil, activityStore, broker)
}

func NewIngestionHandlerWithRouter(scanner *ingestion.Scanner, router *ingestion.Router, activityStore *activity.Store, broker *events.Broker) IngestionHandler {
	return IngestionHandler{scanner: scanner, router: router, activityStore: activityStore, broker: broker}
}

func NewPersistentIngestionHandler(scanner *ingestion.Scanner, router *ingestion.Router, activityStore *activity.Store, broker *events.Broker, saveRoute func() error, rollbackRoute func() error) IngestionHandler {
	return NewPersistentIngestionHandlerWithOriginalSaver(scanner, router, activityStore, broker, saveRoute, rollbackRoute, nil)
}

func NewPersistentIngestionHandlerWithOriginalSaver(scanner *ingestion.Scanner, router *ingestion.Router, activityStore *activity.Store, broker *events.Broker, saveRoute func() error, rollbackRoute func() error, originalSaver *processing.OriginalSaver) IngestionHandler {
	return NewPersistentIngestionHandlerWithProcessors(scanner, router, activityStore, broker, saveRoute, rollbackRoute, originalSaver, nil)
}

func NewPersistentIngestionHandlerWithProcessors(scanner *ingestion.Scanner, router *ingestion.Router, activityStore *activity.Store, broker *events.Broker, saveRoute func() error, rollbackRoute func() error, originalSaver *processing.OriginalSaver, gradedProcessor *processing.GradedProcessor) IngestionHandler {
	return NewPersistentIngestionHandlerWithProcessingGuard(scanner, router, activityStore, broker, saveRoute, rollbackRoute, originalSaver, gradedProcessor, nil)
}

func NewPersistentIngestionHandlerWithProcessingGuard(scanner *ingestion.Scanner, router *ingestion.Router, activityStore *activity.Store, broker *events.Broker, saveRoute func() error, rollbackRoute func() error, originalSaver *processing.OriginalSaver, gradedProcessor *processing.GradedProcessor, guard *processing.ProcessingGuard) IngestionHandler {
	return IngestionHandler{scanner: scanner, router: router, activityStore: activityStore, broker: broker, saveRoute: saveRoute, rollbackRoute: rollbackRoute, originalSaver: originalSaver, gradedProcessor: gradedProcessor, processingGuard: guard}
}

func (h IngestionHandler) Scan(w http.ResponseWriter, r *http.Request) {
	if h.scanner == nil {
		writeAPIError(w, http.StatusInternalServerError, "INGESTION_UNAVAILABLE", "Ingestion scanner belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	result, err := h.scanner.Scan(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INGESTION_SCAN_FAILED", "Scan input folder gagal.", "Periksa folder station lalu coba lagi.")
		return
	}
	routed := []ingestion.RouteResult{}
	quarantined := []ingestion.RouteResult{}
	for _, photo := range result.Photos {
		if h.router != nil {
			routeResult := h.router.Route(photo, time.Now().UTC())
			if routeResult.Duplicate {
				if routeResult.Status == ingestion.PhotoStatus("routed") {
					routed = append(routed, routeResult)
				} else if routeResult.Status == ingestion.PhotoStatus("quarantined") {
					quarantined = append(quarantined, routeResult)
				}
				continue
			}
			if err := h.persistRouteMutation(); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "INGESTION_PERSIST_FAILED", "Perubahan ingestion tidak bisa disimpan permanen.", "Periksa akses local-data/state lalu ulangi scan. Tidak ada perubahan runtime yang dianggap berhasil.")
				return
			}
			h.recordAndPublishDetected(photo)
			if routeResult.Status == ingestion.PhotoStatus("routed") {
				updatedRouteResult, err := h.saveOriginal(routeResult)
				if err != nil {
					writeAPIError(w, http.StatusInternalServerError, "ORIGINAL_SAVE_FAILED", "Original JPG belum berhasil disimpan lokal.", "Periksa source photo dan output folder lalu ulangi scan.")
					return
				}
				routed = append(routed, updatedRouteResult)
				h.recordAndPublishRouted(updatedRouteResult)
			} else if routeResult.Status == ingestion.PhotoStatus("quarantined") {
				quarantined = append(quarantined, routeResult)
				h.recordAndPublishQuarantined(routeResult)
			} else {
				routed = append(routed, routeResult)
			}
		}
	}
	writeData(w, http.StatusOK, IngestionScanData{Photos: result.Photos, RoutedPhotos: routed, QuarantinedPhotos: quarantined, Errors: result.Errors})
}

func (h IngestionHandler) saveOriginal(photo ingestion.RouteResult) (ingestion.RouteResult, error) {
	if h.originalSaver == nil || photo.PhotoID == "" {
		return photo, nil
	}
	result := h.originalSaver.Save(photo.PhotoID)
	if result.Err != nil {
		h.recordAndPublishOriginalFailed(photo, result.Err.Error())
		if result.Photo.PhotoID != "" {
			routedAt := result.Photo.RoutedAt
			return ingestion.RouteResultFromPhoto(result.Photo, &routedAt), result.Err
		}
		return photo, result.Err
	}
	routedAt := result.Photo.RoutedAt
	updated := ingestion.RouteResultFromPhoto(result.Photo, &routedAt)
	h.recordAndPublishOriginalSaved(updated)
	h.enqueueGraded(updated)
	return updated, nil
}

func (h IngestionHandler) enqueueGraded(photo ingestion.RouteResult) {
	if h.gradedProcessor == nil || photo.PhotoID == "" {
		return
	}
	newProcessingRunner(h.gradedProcessor, h.processingGuard, h.activityStore, h.broker).enqueue(photo.PhotoID, false)
}

func (h IngestionHandler) persistRouteMutation() error {
	if h.saveRoute == nil {
		return nil
	}
	if err := h.saveRoute(); err != nil {
		if h.rollbackRoute != nil {
			_ = h.rollbackRoute()
		}
		return err
	}
	return nil
}

func (h IngestionHandler) recordAndPublishDetected(photo ingestion.DetectedPhoto) {
	if h.activityStore != nil {
		stationID := photo.StationID
		h.activityStore.RecordWithRefs("photo.detected", activity.ResultSuccess, "Stable JPG terdeteksi.", &stationID, nil)
	}
	if h.broker != nil {
		if event, err := events.New("photo.detected", "photo", photo.StationID, map[string]any{"station_id": photo.StationID, "status": photo.Status, "detected_at": photo.DetectedAt}); err == nil {
			h.broker.Publish(event)
		}
	}
}

func (h IngestionHandler) recordAndPublishRouted(photo ingestion.RouteResult) {
	if h.activityStore != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activityStore.RecordWithRefs("photo.routed", activity.ResultSuccess, "Photo berhasil diarahkan ke session aktif.", &stationID, &sessionID)
	}
	if h.broker != nil {
		if event, err := events.New("photo.routed", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": photo.Status}); err == nil {
			h.broker.Publish(event)
		}
	}
}

func (h IngestionHandler) recordAndPublishOriginalSaved(photo ingestion.RouteResult) {
	if h.activityStore != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activityStore.RecordWithRefs("photo.original_saved", activity.ResultSuccess, "Original JPG berhasil disimpan lokal.", &stationID, &sessionID)
	}
	if h.broker != nil {
		if event, err := events.New("photo.original_saved", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": "saved_original"}); err == nil {
			h.broker.Publish(event)
		}
		if event, err := events.New("queue.updated", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID}); err == nil {
			h.broker.Publish(event)
		}
	}
}

func (h IngestionHandler) recordAndPublishOriginalFailed(photo ingestion.RouteResult, message string) {
	if h.activityStore != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activityStore.RecordWithRefs("photo.original_save_failed", activity.ResultFailure, "Original JPG gagal disimpan lokal: "+message, &stationID, &sessionID)
	}
}

func (h IngestionHandler) recordAndPublishProcessingStarted(photo ingestion.RouteResult) {
	if h.broker != nil {
		if event, err := events.New("photo.processing_started", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": "processing", "graded_processing_status": "processing"}); err == nil {
			h.broker.Publish(event)
		}
		if event, err := events.New("queue.updated", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "graded_processing_status": "processing"}); err == nil {
			h.broker.Publish(event)
		}
	}
}
func (h IngestionHandler) recordAndPublishProcessed(photo ingestion.RouteResult) {
	if h.activityStore != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activityStore.RecordWithRefs("photo.processed", activity.ResultSuccess, "Graded JPG berhasil dibuat.", &stationID, &sessionID)
	}
	if h.broker != nil {
		if event, err := events.New("photo.processed", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": photo.GradedProcessingStatus, "graded_processing_status": photo.GradedProcessingStatus}); err == nil {
			h.broker.Publish(event)
		}
		if event, err := events.New("queue.updated", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "graded_processing_status": photo.GradedProcessingStatus}); err == nil {
			h.broker.Publish(event)
		}
	}
}
func (h IngestionHandler) recordAndPublishProcessingFailed(photo ingestion.RouteResult) {
	if h.activityStore != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activityStore.RecordWithRefs("photo.processing_failed", activity.ResultFailure, "Graded JPG gagal dibuat: "+photo.GradedLastError, &stationID, &sessionID)
	}
	if h.broker != nil {
		if event, err := events.New("photo.processing_failed", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": photo.GradedProcessingStatus, "graded_processing_status": photo.GradedProcessingStatus}); err == nil {
			h.broker.Publish(event)
		}
		if event, err := events.New("queue.updated", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "graded_processing_status": photo.GradedProcessingStatus}); err == nil {
			h.broker.Publish(event)
		}
	}
}

func (h IngestionHandler) recordAndPublishQuarantined(photo ingestion.RouteResult) {
	if h.activityStore != nil {
		stationID := photo.StationID
		var sessionRef *string
		if photo.RelatedSessionID != "" {
			sessionRef = &photo.RelatedSessionID
		}
		h.activityStore.RecordWithRefs("photo.quarantined", activity.ResultSuccess, "Photo dikarantina agar tidak salah masuk session. Reason: "+photo.Reason+".", &stationID, sessionRef)
	}
	if h.broker != nil {
		if event, err := events.New("photo.quarantined", "quarantine", photo.QuarantineID, map[string]any{"quarantine_id": photo.QuarantineID, "station_id": photo.StationID, "related_session_id": photo.RelatedSessionID, "status": photo.Status, "reason": photo.Reason}); err == nil {
			h.broker.Publish(event)
		}
	}
}
