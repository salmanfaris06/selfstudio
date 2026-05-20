package api

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
	"selfstudio/agent/internal/upload"
)

type SessionsHandler struct {
	stationStore    *stations.Store
	sessionStore    *sessions.Store
	persistence     sessions.Persistence
	activityStore   *activity.Store
	broker          *events.Broker
	photoStore      *photos.Store
	quarantineStore *quarantine.Store
	uploadTargets   *upload.Store
	uploadJobs      *upload.JobsStore
	validator       stations.ReadinessValidator
	outputRoot      string
}

type SessionData struct {
	Session sessions.Session `json:"session"`
}

type SessionListData struct {
	Sessions  []sessions.Session `json:"sessions"`
	Recovered bool               `json:"recovered"`
}

type EndSessionData struct {
	Session sessions.Session `json:"session"`
}

type SessionDetailData struct {
	Session sessions.Session `json:"session"`
	Summary SessionSummary   `json:"summary"`
	Photos  []photos.Photo   `json:"photos"`
}

type StationQuarantineSummaryData struct {
	Summary StationQuarantineSummary `json:"summary"`
}

type StationQuarantineSummary struct {
	StationID              string `json:"station_id"`
	StationQuarantineCount int    `json:"station_quarantine_count"`
	LatestQuarantineReason string `json:"latest_quarantine_reason,omitempty"`
}

type SessionSummary struct {
	LocalOutputFolder      string `json:"local_output_folder"`
	PhotoCount             int    `json:"photo_count"`
	Failures               int    `json:"failures"`
	QuarantineCount        int    `json:"quarantine_count"`
	StationQuarantineCount int    `json:"station_quarantine_count"`
	LatestQuarantineReason string `json:"latest_quarantine_reason,omitempty"`
	UploadStatus           string `json:"upload_status"`
	DriveTargetStatus      string `json:"drive_target_status"`
	DriveTargetIdentity    string `json:"drive_target_identity"`
	DriveSessionFolderID   string `json:"drive_session_folder_id"`
	DriveFolderPath        string `json:"drive_folder_path"`
	DriveRootFolderID      string `json:"drive_root_folder_id"`
	DriveRootFolderName    string `json:"drive_root_folder_name"`
	DriveLastErrorCode     string `json:"drive_last_error_code"`
	DriveLastErrorAction   string `json:"drive_last_error_action"`
}

func NewSessionsHandler(stationStore *stations.Store, sessionStore *sessions.Store, persistence sessions.Persistence, activityStore *activity.Store, broker *events.Broker, outputRoot string) SessionsHandler {
	return NewSessionsHandlerWithPhotos(stationStore, sessionStore, persistence, activityStore, broker, outputRoot, nil)
}

func NewSessionsHandlerWithPhotos(stationStore *stations.Store, sessionStore *sessions.Store, persistence sessions.Persistence, activityStore *activity.Store, broker *events.Broker, outputRoot string, photoStore *photos.Store) SessionsHandler {
	return NewSessionsHandlerWithPhotosAndQuarantine(stationStore, sessionStore, persistence, activityStore, broker, outputRoot, photoStore, nil)
}

func NewSessionsHandlerWithPhotosAndQuarantine(stationStore *stations.Store, sessionStore *sessions.Store, persistence sessions.Persistence, activityStore *activity.Store, broker *events.Broker, outputRoot string, photoStore *photos.Store, quarantineStore *quarantine.Store) SessionsHandler {
	return NewSessionsHandlerWithPhotosQuarantineAndUpload(stationStore, sessionStore, persistence, activityStore, broker, outputRoot, photoStore, quarantineStore, nil)
}

func NewSessionsHandlerWithPhotosQuarantineAndUpload(stationStore *stations.Store, sessionStore *sessions.Store, persistence sessions.Persistence, activityStore *activity.Store, broker *events.Broker, outputRoot string, photoStore *photos.Store, quarantineStore *quarantine.Store, uploadTargets *upload.Store) SessionsHandler {
	return NewSessionsHandlerWithPhotosQuarantineUploadJobs(stationStore, sessionStore, persistence, activityStore, broker, outputRoot, photoStore, quarantineStore, uploadTargets, nil)
}

func NewSessionsHandlerWithPhotosQuarantineUploadJobs(stationStore *stations.Store, sessionStore *sessions.Store, persistence sessions.Persistence, activityStore *activity.Store, broker *events.Broker, outputRoot string, photoStore *photos.Store, quarantineStore *quarantine.Store, uploadTargets *upload.Store, uploadJobs *upload.JobsStore) SessionsHandler {
	if outputRoot == "" {
		outputRoot = filepath.Join("local-data", "output")
	}
	return SessionsHandler{stationStore: stationStore, sessionStore: sessionStore, persistence: persistence, activityStore: activityStore, broker: broker, photoStore: photoStore, quarantineStore: quarantineStore, uploadTargets: uploadTargets, uploadJobs: uploadJobs, validator: stations.NewReadinessValidator(outputRoot), outputRoot: outputRoot}
}

func (h SessionsHandler) WithReadinessValidator(validator stations.ReadinessValidator) SessionsHandler {
	h.validator = validator
	return h
}

func (h SessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.sessionStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	h.persistAndPublishExpired(h.sessionStore.LockExpired(time.Now().UTC()))
	writeData(w, http.StatusOK, SessionListData{Sessions: h.sessionStore.List(), Recovered: true})
}

func (h SessionsHandler) End(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if h.sessionStore == nil {
		h.recordEndFailure(nil, sessionID, "Session end gagal karena service session belum siap.")
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type harus application/json.", "Kirim request JSON dari dashboard.")
			return
		}
	}
	request := sessions.EndSessionRequest{Reason: sessions.EndReasonManual}
	if r.Body != nil {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_SESSION_END_REQUEST", "Data end session tidak valid.", "Kirim reason manual atau timer.")
			return
		}
		if err := decoder.Decode(&struct{}{}); err == nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_SESSION_END_REQUEST", "Data end session tidak valid.", "Kirim satu object JSON saja.")
			return
		}
	}
	session, err := h.sessionStore.End(sessionID, request.Reason, time.Now().UTC())
	if err != nil {
		h.recordEndFailure(nil, sessionID, "Session end gagal.")
		if errors.Is(err, sessions.ErrSessionNotFound) {
			writeAPIError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session tidak ditemukan.", "Refresh dashboard lalu pilih session aktif.")
			return
		}
		writeAPIError(w, http.StatusBadRequest, "INVALID_SESSION_END_REQUEST", "Session tidak bisa diakhiri.", "Refresh dashboard lalu coba lagi.")
		return
	}
	if err := h.persistence.Save(h.sessionStore); err != nil {
		h.recordEndFailure(&session.StationID, sessionID, "Session end gagal disimpan.")
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session end gagal disimpan.", "Coba lagi atau restart aplikasi sebelum event.")
		return
	}
	h.recordEnded(session)
	h.publishEnded(session)
	writeData(w, http.StatusOK, EndSessionData{Session: session})
}

func (h SessionsHandler) StationQuarantineSummary(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if !knownStationID(stationID) {
		writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
		return
	}
	summary := StationQuarantineSummary{StationID: stationID}
	if h.quarantineStore != nil {
		summary.StationQuarantineCount = h.quarantineStore.CountByStation(stationID)
		if latest := h.quarantineStore.ListByStation(stationID, 1); len(latest) > 0 {
			summary.LatestQuarantineReason = latest[0].Reason
		}
	}
	writeData(w, http.StatusOK, StationQuarantineSummaryData{Summary: summary})
}

func (h SessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.sessionStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	session, err := h.sessionStore.Get(r.PathValue("session_id"))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session tidak ditemukan.", "Refresh dashboard lalu pilih session yang tersedia.")
		return
	}
	routedPhotos := []photos.Photo{}
	photoCount := 0
	quarantineCount := 0
	stationQuarantineCount := 0
	latestReason := ""
	failures := 0
	if h.photoStore != nil {
		routedPhotos = h.photoStore.ListBySession(session.SessionID, 20)
		photoCount = h.photoStore.CountBySession(session.SessionID)
		for _, photo := range h.photoStore.ListBySession(session.SessionID, 0) {
			if photo.GradedProcessingStatus == photos.GradedStatusFailed {
				failures++
			}
		}
	}
	if h.quarantineStore != nil {
		quarantineCount = h.quarantineStore.CountByRelatedSession(session.SessionID)
		stationQuarantineCount = h.quarantineStore.CountByStation(session.StationID)
		if latest := h.quarantineStore.ListByStation(session.StationID, 1); len(latest) > 0 {
			latestReason = latest[0].Reason
		}
	}
	uploadStatus := upload.SessionUploadNotConfigured
	driveTarget := upload.SessionCloudTarget{SessionID: session.SessionID, StationID: session.StationID, Status: upload.StatusPending}
	if h.uploadTargets != nil {
		if existing, ok := h.uploadTargets.Get(session.SessionID); ok {
			driveTarget = existing
		}
		jobs := []upload.FileUploadJob{}
		if h.uploadJobs != nil {
			jobs = h.uploadJobs.ListBySession(session.SessionID)
		}
		uploadStatus = upload.AggregateUploadStatus(driveTarget, jobs)
	}
	writeData(w, http.StatusOK, SessionDetailData{Session: session, Summary: SessionSummary{LocalOutputFolder: session.StationSnapshot.OutputFolder, PhotoCount: photoCount, Failures: failures, QuarantineCount: quarantineCount, StationQuarantineCount: stationQuarantineCount, LatestQuarantineReason: latestReason, UploadStatus: uploadStatus, DriveTargetStatus: driveTarget.PublicStatus(), DriveTargetIdentity: driveTarget.RemoteIdentity, DriveSessionFolderID: driveTarget.DriveSessionFolderID, DriveFolderPath: driveTarget.DriveFolderPath, DriveRootFolderID: driveTarget.DriveRootFolderID, DriveRootFolderName: driveTarget.DriveRootFolderName, DriveLastErrorCode: driveTarget.LastErrorCode, DriveLastErrorAction: driveTarget.LastErrorAction}, Photos: routedPhotos})
}

func (h SessionsHandler) Start(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if h.stationStore == nil || h.sessionStore == nil {
		h.recordStartFailure(stationID, "Session start gagal karena service session belum siap.")
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type harus application/json.", "Kirim request JSON dari dashboard.")
		return
	}
	var request sessions.StartSessionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		h.recordStartFailure(stationID, "Session start gagal karena request tidak valid.")
		writeAPIError(w, http.StatusBadRequest, "INVALID_SESSION_REQUEST", "Data session tidak valid.", "Isi customer name, order number, dan durasi timer dengan benar.")
		return
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		h.recordStartFailure(stationID, "Session start gagal karena request tidak valid.")
		writeAPIError(w, http.StatusBadRequest, "INVALID_SESSION_REQUEST", "Data session tidak valid.", "Isi customer name, order number, dan durasi timer dengan benar.")
		return
	}
	station, err := h.stationStore.Get(stationID)
	if err != nil {
		h.recordStartFailure(stationID, "Session start gagal karena station tidak ditemukan.")
		if errors.Is(err, stations.ErrStationNotFound) {
			writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Station tidak bisa dibaca.", "Restart aplikasi lalu coba lagi.")
		return
	}
	readiness := h.validator.Check(station)
	h.persistAndPublishExpired(h.sessionStore.LockExpired(time.Now().UTC()))
	if readiness.Status == stations.ReadinessFailed {
		h.recordStartFailure(stationID, "Session start diblokir karena readiness station failed.")
		writeAPIError(w, http.StatusConflict, "SESSION_READINESS_BLOCKED", readiness.Label, readiness.Action)
		return
	}
	outputFolder, err := h.deriveSessionOutputFolder(station, request)
	if err != nil {
		h.recordStartFailure(stationID, "Session start gagal karena output rule tidak aman.")
		writeAPIError(w, http.StatusConflict, "SESSION_READINESS_BLOCKED", "Output folder session tidak aman.", "Perbaiki output rule station lalu coba lagi.")
		return
	}
	session, err := h.sessionStore.Start(station, request, outputFolder, time.Now().UTC())
	if err != nil {
		h.recordStartFailure(stationID, "Session start gagal.")
		if errors.Is(err, sessions.ErrSessionAlreadyActive) {
			writeAPIError(w, http.StatusConflict, "SESSION_ALREADY_ACTIVE", "Station sudah memiliki session aktif.", "Akhiri session aktif sebelum memulai session baru.")
			return
		}
		if errors.Is(err, sessions.ErrInvalidSession) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_SESSION_REQUEST", "Data session tidak valid.", "Isi customer name, order number, dan timer minimal 60 detik.")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session gagal dibuat.", "Coba lagi atau restart aplikasi.")
		return
	}
	if err := h.persistence.Save(h.sessionStore); err != nil {
		h.sessionStore.Remove(session.SessionID)
		h.recordStartFailure(stationID, "Session start gagal disimpan.")
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session gagal disimpan.", "Coba lagi atau restart aplikasi sebelum event.")
		return
	}
	h.recordStarted(session)
	h.publishStarted(session)
	writeData(w, http.StatusCreated, SessionData{Session: session})
}

func (h SessionsHandler) deriveSessionOutputFolder(station stations.Station, request sessions.StartSessionRequest) (string, error) {
	relative := strings.NewReplacer(
		"{station_id}", station.StationID,
		"{customer_name}", request.CustomerName,
		"{order_number}", request.OrderNumber,
		"{session_id}", "session",
	).Replace(station.OutputRule)
	relative = filepath.Clean(filepath.FromSlash(strings.ReplaceAll(relative, "\\", "/")))
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", stations.ErrInvalidStationConfig
	}
	root, err := filepath.Abs(filepath.Clean(h.outputRoot))
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, relative))
	if err != nil {
		return "", err
	}
	if candidate != root && !strings.HasPrefix(candidate, root+string(filepath.Separator)) {
		return "", stations.ErrInvalidStationConfig
	}
	return candidate, nil
}

func (h SessionsHandler) persistAndPublishExpired(expired []sessions.Session) {
	if len(expired) == 0 {
		return
	}
	if err := h.persistence.Save(h.sessionStore); err != nil {
		for _, session := range expired {
			h.recordEndFailure(&session.StationID, session.SessionID, "Timer session lock gagal disimpan.")
		}
		return
	}
	for _, session := range expired {
		h.recordEnded(session)
		h.publishEnded(session)
	}
}

func (h SessionsHandler) recordEnded(session sessions.Session) {
	if h.activityStore == nil {
		return
	}
	stationID := session.StationID
	sessionID := session.SessionID
	h.activityStore.RecordWithRefs("session.ended", activity.ResultSuccess, "Session berhasil diakhiri.", &stationID, &sessionID)
}

func (h SessionsHandler) recordEndFailure(stationID *string, sessionID string, message string) {
	if h.activityStore == nil {
		return
	}
	h.activityStore.RecordWithRefs("session.end_failed", activity.ResultFailure, message, stationID, &sessionID)
}

func (h SessionsHandler) publishEnded(session sessions.Session) {
	if h.broker == nil {
		return
	}
	event, err := events.New("session.ended", "session", session.SessionID, map[string]any{"session": session})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}

func (h SessionsHandler) recordStarted(session sessions.Session) {
	if h.activityStore == nil {
		return
	}
	stationID := session.StationID
	sessionID := session.SessionID
	h.activityStore.RecordWithRefs("session.started", activity.ResultSuccess, "Session berhasil dimulai.", &stationID, &sessionID)
}

func (h SessionsHandler) recordStartFailure(stationID string, message string) {
	if h.activityStore == nil {
		return
	}
	var stationRef *string
	if knownStationID(stationID) {
		stationRef = &stationID
	}
	h.activityStore.RecordWithRefs("session.start_failed", activity.ResultFailure, message, stationRef, nil)
}

func (h SessionsHandler) publishStarted(session sessions.Session) {
	if h.broker == nil {
		return
	}
	event, err := events.New("session.started", "session", session.SessionID, map[string]any{"session": session})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}
