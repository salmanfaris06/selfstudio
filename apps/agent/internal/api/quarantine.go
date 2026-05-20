package api

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/sessions"
)

type QuarantineHandler struct {
	quarantineStore *quarantine.Store
	sessionStore    *sessions.Store
	photoStore      *photos.Store
	activityStore   *activity.Store
	broker          *events.Broker
	saveAssignment  func() error
	rollbackAssign  func() error
	originalSaver   *processing.OriginalSaver
	gradedProcessor *processing.GradedProcessor
	processingGuard *processing.ProcessingGuard
}

type QuarantineListData struct {
	Items []quarantine.Record `json:"items"`
}

type EligibleSession struct {
	SessionID            string `json:"session_id"`
	StationID            string `json:"station_id"`
	Status               string `json:"status"`
	CustomerName         string `json:"customer_name"`
	OrderNumber          string `json:"order_number"`
	Eligible             bool   `json:"eligible"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	EligibilityReason    string `json:"eligibility_reason"`
}

type EligibleSessionsData struct {
	QuarantineID string            `json:"quarantine_id"`
	Sessions     []EligibleSession `json:"sessions"`
}

type AssignQuarantineRequest struct {
	SessionID string `json:"session_id"`
}

type AssignQuarantineData struct {
	Quarantine quarantine.Record `json:"quarantine"`
	Photo      photos.Photo      `json:"photo"`
}

func NewQuarantineHandler(quarantineStore *quarantine.Store, sessionStore *sessions.Store, photoStore *photos.Store, activityStore *activity.Store, broker *events.Broker) QuarantineHandler {
	return QuarantineHandler{quarantineStore: quarantineStore, sessionStore: sessionStore, photoStore: photoStore, activityStore: activityStore, broker: broker}
}

func NewPersistentQuarantineHandler(quarantineStore *quarantine.Store, sessionStore *sessions.Store, photoStore *photos.Store, activityStore *activity.Store, broker *events.Broker, saveAssignment func() error, rollbackAssign func() error) QuarantineHandler {
	return NewPersistentQuarantineHandlerWithOriginalSaver(quarantineStore, sessionStore, photoStore, activityStore, broker, saveAssignment, rollbackAssign, nil)
}

func NewPersistentQuarantineHandlerWithOriginalSaver(quarantineStore *quarantine.Store, sessionStore *sessions.Store, photoStore *photos.Store, activityStore *activity.Store, broker *events.Broker, saveAssignment func() error, rollbackAssign func() error, originalSaver *processing.OriginalSaver) QuarantineHandler {
	return NewPersistentQuarantineHandlerWithProcessors(quarantineStore, sessionStore, photoStore, activityStore, broker, saveAssignment, rollbackAssign, originalSaver, nil)
}

func NewPersistentQuarantineHandlerWithProcessors(quarantineStore *quarantine.Store, sessionStore *sessions.Store, photoStore *photos.Store, activityStore *activity.Store, broker *events.Broker, saveAssignment func() error, rollbackAssign func() error, originalSaver *processing.OriginalSaver, gradedProcessor *processing.GradedProcessor) QuarantineHandler {
	return NewPersistentQuarantineHandlerWithProcessingGuard(quarantineStore, sessionStore, photoStore, activityStore, broker, saveAssignment, rollbackAssign, originalSaver, gradedProcessor, nil)
}

func NewPersistentQuarantineHandlerWithProcessingGuard(quarantineStore *quarantine.Store, sessionStore *sessions.Store, photoStore *photos.Store, activityStore *activity.Store, broker *events.Broker, saveAssignment func() error, rollbackAssign func() error, originalSaver *processing.OriginalSaver, gradedProcessor *processing.GradedProcessor, guard *processing.ProcessingGuard) QuarantineHandler {
	return QuarantineHandler{quarantineStore: quarantineStore, sessionStore: sessionStore, photoStore: photoStore, activityStore: activityStore, broker: broker, saveAssignment: saveAssignment, rollbackAssign: rollbackAssign, originalSaver: originalSaver, gradedProcessor: gradedProcessor, processingGuard: guard}
}

func (h QuarantineHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.quarantineStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "QUARANTINE_UNAVAILABLE", "Quarantine service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 100)
	items := h.quarantineStore.List(quarantine.ListFilter{Status: r.URL.Query().Get("status"), StationID: r.URL.Query().Get("station_id"), Limit: limit})
	writeData(w, http.StatusOK, QuarantineListData{Items: items})
}

func (h QuarantineHandler) EligibleSessions(w http.ResponseWriter, r *http.Request) {
	record, ok := h.loadRecordOrError(w, r.PathValue("quarantine_id"))
	if !ok {
		return
	}
	writeData(w, http.StatusOK, EligibleSessionsData{QuarantineID: record.QuarantineID, Sessions: h.eligibleSessions(record)})
}

func (h QuarantineHandler) Assign(w http.ResponseWriter, r *http.Request) {
	record, ok := h.loadRecordOrError(w, r.PathValue("quarantine_id"))
	if !ok {
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type harus application/json.", "Kirim request JSON dari dashboard.")
			return
		}
	}
	var request AssignQuarantineRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.SessionID) == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_QUARANTINE_ASSIGN_REQUEST", "Target session assignment tidak valid.", "Pilih session tujuan dari daftar eligible lalu coba lagi.")
		return
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_QUARANTINE_ASSIGN_REQUEST", "Request assignment tidak valid.", "Kirim satu object JSON saja.")
		return
	}
	target, err := h.sessionStore.Get(request.SessionID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "QUARANTINE_SESSION_NOT_FOUND", "Session tujuan tidak ditemukan.", "Refresh daftar eligible session lalu pilih ulang.")
		return
	}
	if !isEligibleQuarantineTarget(record, target) {
		writeAPIError(w, http.StatusConflict, "QUARANTINE_SESSION_INELIGIBLE", "Session tujuan tidak eligible untuk item quarantine ini.", "Pilih session aktif dari station yang sama, atau related locked session khusus recovery late-photo.")
		return
	}
	if existing, exists := h.photoStore.GetBySourceIdentity(record.StationID, record.SourcePath, record.SourceSizeBytes); exists {
		if existing.SessionID != target.SessionID {
			writeAPIError(w, http.StatusConflict, "QUARANTINE_PHOTO_CONFLICT", "Foto ini sudah routed ke session lain.", "Buka session terkait atau minta supervisor cek konflik foto.")
			return
		}
		assigned, err := h.quarantineStore.Assign(record.QuarantineID, target.SessionID, existing.PhotoID, time.Now().UTC())
		if err != nil {
			h.writeAssignStoreError(w, err)
			return
		}
		if err := h.persistAssignment(); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "QUARANTINE_PERSIST_FAILED", "Assignment quarantine tidak bisa disimpan permanen.", "Periksa akses local-data/state lalu ulangi assignment. Tidak ada perubahan runtime yang dianggap berhasil.")
			return
		}
		writeData(w, http.StatusOK, AssignQuarantineData{Quarantine: assigned, Photo: existing})
		return
	}
	now := time.Now().UTC()
	photo := h.photoStore.Route(record.StationID, target.SessionID, record.SourcePath, record.SourceSizeBytes, record.DetectedAt, record.StableAt, now)
	if photo.SessionID != target.SessionID {
		writeAPIError(w, http.StatusConflict, "QUARANTINE_PHOTO_CONFLICT", "Foto ini sudah routed ke session lain.", "Buka session terkait atau minta supervisor cek konflik foto.")
		return
	}
	assigned, err := h.quarantineStore.Assign(record.QuarantineID, target.SessionID, photo.PhotoID, now)
	if err != nil {
		h.writeAssignStoreError(w, err)
		return
	}
	if err := h.persistAssignment(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "QUARANTINE_PERSIST_FAILED", "Assignment quarantine tidak bisa disimpan permanen.", "Periksa akses local-data/state lalu ulangi assignment. Tidak ada perubahan runtime yang dianggap berhasil.")
		return
	}
	updatedPhoto, err := h.saveAssignedOriginal(photo)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "ORIGINAL_SAVE_FAILED", "Original JPG assignment belum berhasil disimpan lokal.", "Periksa source photo dan output folder lalu ulangi assignment.")
		return
	}
	h.recordAndPublishAssigned(assigned, updatedPhoto)
	writeData(w, http.StatusOK, AssignQuarantineData{Quarantine: assigned, Photo: updatedPhoto})
}

func (h QuarantineHandler) saveAssignedOriginal(photo photos.Photo) (photos.Photo, error) {
	if h.originalSaver == nil {
		return photo, nil
	}
	result := h.originalSaver.Save(photo.PhotoID)
	if result.Err != nil {
		if h.activityStore != nil {
			stationID := photo.StationID
			sessionID := photo.SessionID
			h.activityStore.RecordWithRefs("photo.original_save_failed", activity.ResultFailure, "Original JPG assignment gagal disimpan lokal: "+result.Err.Error(), &stationID, &sessionID)
		}
		if result.Photo.PhotoID != "" {
			return result.Photo, result.Err
		}
		return photo, result.Err
	}
	if h.activityStore != nil {
		stationID := result.Photo.StationID
		sessionID := result.Photo.SessionID
		h.activityStore.RecordWithRefs("photo.original_saved", activity.ResultSuccess, "Original JPG assignment berhasil disimpan lokal.", &stationID, &sessionID)
	}
	if h.broker != nil {
		if event, err := events.New("photo.original_saved", "photo", result.Photo.PhotoID, map[string]any{"photo_id": result.Photo.PhotoID, "station_id": result.Photo.StationID, "session_id": result.Photo.SessionID, "status": "saved_original"}); err == nil {
			h.broker.Publish(event)
		}
		if event, err := events.New("queue.updated", "photo", result.Photo.PhotoID, map[string]any{"photo_id": result.Photo.PhotoID, "station_id": result.Photo.StationID, "session_id": result.Photo.SessionID, "graded_processing_status": result.Photo.GradedProcessingStatus}); err == nil {
			h.broker.Publish(event)
		}
	}
	h.enqueueAssignedGraded(result.Photo)
	return result.Photo, nil
}

func (h QuarantineHandler) enqueueAssignedGraded(photo photos.Photo) {
	if h.gradedProcessor == nil || photo.PhotoID == "" {
		return
	}
	newProcessingRunner(h.gradedProcessor, h.processingGuard, h.activityStore, h.broker).enqueue(photo.PhotoID, false)
}

func (h QuarantineHandler) persistAssignment() error {
	if h.saveAssignment == nil {
		return nil
	}
	if err := h.saveAssignment(); err != nil {
		if h.rollbackAssign != nil {
			_ = h.rollbackAssign()
		}
		return err
	}
	return nil
}

func (h QuarantineHandler) loadRecordOrError(w http.ResponseWriter, quarantineID string) (quarantine.Record, bool) {
	if h.quarantineStore == nil || h.sessionStore == nil || h.photoStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "QUARANTINE_UNAVAILABLE", "Quarantine assignment service belum siap.", "Restart aplikasi lalu coba lagi.")
		return quarantine.Record{}, false
	}
	record, err := h.quarantineStore.Get(quarantineID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "QUARANTINE_NOT_FOUND", "Item quarantine tidak ditemukan.", "Refresh daftar quarantine lalu pilih item yang tersedia.")
		return quarantine.Record{}, false
	}
	return record, true
}

func (h QuarantineHandler) eligibleSessions(record quarantine.Record) []EligibleSession {
	items := []EligibleSession{}
	for _, session := range h.sessionStore.List() {
		if !isEligibleQuarantineTarget(record, session) {
			continue
		}
		reason := "same_station_manual_recovery"
		if isRelatedLockedLatePhotoRecovery(record, session) {
			reason = "related_locked_late_photo_recovery"
		}
		items = append(items, EligibleSession{SessionID: session.SessionID, StationID: session.StationID, Status: session.Status, CustomerName: session.CustomerName, OrderNumber: session.OrderNumber, Eligible: true, RequiresConfirmation: true, EligibilityReason: reason})
	}
	return items
}

func isEligibleQuarantineTarget(record quarantine.Record, session sessions.Session) bool {
	if session.StationID != record.StationID {
		return false
	}
	if session.Status == sessions.StatusLocked {
		return isRelatedLockedLatePhotoRecovery(record, session)
	}
	return session.Status == sessions.StatusActive
}

func isRelatedLockedLatePhotoRecovery(record quarantine.Record, session sessions.Session) bool {
	return record.Reason == quarantine.ReasonLatePhoto && record.RelatedSessionID == session.SessionID && session.Status == sessions.StatusLocked
}

func (h QuarantineHandler) writeAssignStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, quarantine.ErrAlreadyAssignedDifferentSession) {
		writeAPIError(w, http.StatusConflict, "QUARANTINE_ALREADY_ASSIGNED", "Item quarantine sudah di-assign ke session lain.", "Refresh daftar quarantine sebelum melanjutkan.")
		return
	}
	writeAPIError(w, http.StatusConflict, "QUARANTINE_NOT_ASSIGNABLE", "Item quarantine tidak bisa di-assign.", "Refresh daftar quarantine dan pilih item berstatus quarantined.")
}

func (h QuarantineHandler) recordAndPublishAssigned(record quarantine.Record, photo photos.Photo) {
	if h.activityStore != nil {
		stationID := record.StationID
		sessionID := record.AssignedSessionID
		h.activityStore.RecordWithRefs("quarantine.assigned", activity.ResultSuccess, "Photo quarantine berhasil di-assign manual ke session tujuan.", &stationID, &sessionID)
	}
	if h.broker != nil {
		if event, err := events.New("quarantine.assigned", "quarantine", record.QuarantineID, map[string]any{"quarantine_id": record.QuarantineID, "photo_id": photo.PhotoID, "station_id": record.StationID, "assigned_session_id": record.AssignedSessionID, "status": record.Status}); err == nil {
			h.broker.Publish(event)
		}
		if event, err := events.New("photo.routed", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": photo.Status}); err == nil {
			h.broker.Publish(event)
		}
	}
}

func (h QuarantineHandler) publishAssignedProcessingStarted(photo photos.Photo) {
	if h.broker != nil {
		if event, err := events.New("photo.processing_started", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "status": "processing", "graded_processing_status": "processing"}); err == nil {
			h.broker.Publish(event)
		}
		if event, err := events.New("queue.updated", "photo", photo.PhotoID, map[string]any{"photo_id": photo.PhotoID, "station_id": photo.StationID, "session_id": photo.SessionID, "graded_processing_status": "processing"}); err == nil {
			h.broker.Publish(event)
		}
	}
}

func (h QuarantineHandler) recordAndPublishAssignedProcessed(photo photos.Photo) {
	if h.activityStore != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activityStore.RecordWithRefs("photo.processed", activity.ResultSuccess, "Graded JPG assignment berhasil dibuat.", &stationID, &sessionID)
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

func (h QuarantineHandler) recordAndPublishAssignedProcessingFailed(photo photos.Photo) {
	if h.activityStore != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activityStore.RecordWithRefs("photo.processing_failed", activity.ResultFailure, "Graded JPG assignment gagal dibuat: "+photo.GradedLastError, &stationID, &sessionID)
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

func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
