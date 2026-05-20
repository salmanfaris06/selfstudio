package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/upload"
)

type SessionUploadsHandler struct {
	Sessions *sessions.Store
	Targets  *upload.Store
	Jobs     *upload.JobsStore
	Worker   *upload.Worker
	Activity *activity.Store
	Broker   *events.Broker
}
type SessionUploadsData struct {
	SessionID    string         `json:"session_id"`
	UploadStatus string         `json:"upload_status"`
	Jobs         []UploadJobDTO `json:"jobs"`
}

type UploadJobDTO struct {
	JobID                string     `json:"job_id"`
	SessionID            string     `json:"session_id"`
	StationID            string     `json:"station_id"`
	PhotoID              string     `json:"photo_id"`
	AssetKind            string     `json:"asset_kind"`
	LocalFileName        string     `json:"local_file_name,omitempty"`
	BucketName           string     `json:"bucket_name,omitempty"`
	ObjectKey            string     `json:"object_key,omitempty"`
	DriveFolderID        string     `json:"drive_folder_id,omitempty"`
	DriveFileID          string     `json:"drive_file_id,omitempty"`
	RemoteIdentity       string     `json:"remote_identity,omitempty"`
	RemoteGeneration     int64      `json:"remote_generation,omitempty"`
	RemoteMetageneration int64      `json:"remote_metageneration,omitempty"`
	RemoteETag           string     `json:"remote_etag,omitempty"`
	DedupeKey            string     `json:"dedupe_key,omitempty"`
	Status               string     `json:"status"`
	AttemptCount         int        `json:"attempt_count"`
	MaxAttempts          int        `json:"max_attempts,omitempty"`
	LastErrorCode        string     `json:"last_error_code,omitempty"`
	LastErrorAction      string     `json:"last_error_action,omitempty"`
	LastAttemptAt        *time.Time `json:"last_attempt_at,omitempty"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	RetryAfterSeconds    int        `json:"retry_after,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	UploadedAt           *time.Time `json:"uploaded_at,omitempty"`
}

func NewSessionUploadsHandler(sessionStore *sessions.Store, targetStore *upload.Store, jobs *upload.JobsStore, worker *upload.Worker, activityStore *activity.Store, broker *events.Broker) SessionUploadsHandler {
	h := SessionUploadsHandler{Sessions: sessionStore, Targets: targetStore, Jobs: jobs, Worker: worker, Activity: activityStore, Broker: broker}
	if worker != nil {
		h.consumeWorkerEvents(worker.Events)
	}
	return h
}

func (h SessionUploadsHandler) consumeWorkerEvents(ch <-chan upload.FileUploadJob) {
	if ch == nil {
		return
	}
	go func() {
		for j := range ch {
			name := "upload.session_updated"
			action := "cloud.upload_session_failed"
			result := activity.ResultSuccess
			message := "Google Drive upload status updated"
			switch j.Status {
			case upload.JobStatusUploaded:
				name = "upload.file_uploaded"
				action = "cloud.upload_file_uploaded"
				message = "Google Drive file uploaded"
			case upload.JobStatusRetrying:
				name = "upload.retry_started"
				action = "cloud.upload_retry_started"
				message = "Google Drive upload retry started"
			case upload.JobStatusRetryScheduled:
				name = "upload.retry_scheduled"
				action = "cloud.upload_retry_scheduled"
				message = "Google Drive upload retry scheduled"
			case upload.JobStatusFailed:
				name = "upload.retry_failed"
				action = "cloud.upload_retry_failed"
				if j.AttemptCount >= upload.MaxAutoUploadAttempts {
					name = "upload.retry_exhausted"
					action = "cloud.upload_retry_exhausted"
				}
				result = activity.ResultFailure
				message = "Google Drive upload failed"
			}
			s := sessions.Session{SessionID: j.SessionID, StationID: j.StationID}
			h.record(action, result, message)
			h.publish(name, s, &j)
			h.publish("upload.session_updated", s, &j)
		}
	}()
}

func (h SessionUploadsHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, ok := h.getSession(w, r)
	if !ok {
		return
	}
	jobs := []upload.FileUploadJob{}
	if h.Jobs != nil {
		jobs = h.Jobs.ListBySession(s.SessionID)
	}
	target := upload.SessionCloudTarget{SessionID: s.SessionID, StationID: s.StationID}
	if h.Targets != nil {
		if t, found := h.Targets.Get(s.SessionID); found {
			target = t
		}
	}
	writeData(w, http.StatusOK, SessionUploadsData{SessionID: s.SessionID, UploadStatus: upload.AggregateUploadStatus(target, jobs), Jobs: toUploadJobDTOs(jobs)})
}
func (h SessionUploadsHandler) Start(w http.ResponseWriter, r *http.Request) {
	s, ok := h.getSession(w, r)
	if !ok {
		return
	}
	if h.Worker == nil {
		writeAPIError(w, http.StatusInternalServerError, "UPLOAD_WORKER_UNAVAILABLE", "Google Drive upload service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	res, err := h.Worker.StartSession(r.Context(), s.SessionID)
	if err != nil {
		var se upload.SafeUploadError
		code, action := upload.ErrorUploadFailed, upload.ActionRetryCloudUpload
		if upload.AsSafeUploadError(err, &se) {
			code, action = se.Code, se.Action
		}
		status := http.StatusConflict
		if code == upload.ErrorUploadStateSaveFailed {
			status = http.StatusInternalServerError
		}
		writeAPIErrorWithDetails(w, status, code, "Google Drive upload belum berhasil dimulai.", action, map[string]any{"session_id": s.SessionID, "session_upload_status": res.SessionStatus})
		return
	}
	h.record("cloud.upload_started", activity.ResultSuccess, "Google Drive upload started")
	h.publish("upload.started", s, nil)
	writeData(w, http.StatusAccepted, SessionUploadsData{SessionID: s.SessionID, UploadStatus: res.SessionStatus, Jobs: toUploadJobDTOs(res.Jobs)})
}

func (h SessionUploadsHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	if h.Worker == nil {
		writeAPIError(w, http.StatusInternalServerError, "UPLOAD_WORKER_UNAVAILABLE", "Google Drive upload retry service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	res, err := h.Worker.RetryJob(r.Context(), r.PathValue("job_id"), true)
	if err != nil {
		h.writeRetryError(w, err, res)
		return
	}
	h.record("cloud.upload_retry_started", activity.ResultSuccess, "Google Drive upload retry requested")
	writeData(w, http.StatusAccepted, toRetryResultDTO(res))
}

func (h SessionUploadsHandler) RetrySession(w http.ResponseWriter, r *http.Request) {
	s, ok := h.getSession(w, r)
	if !ok {
		return
	}
	if h.Worker == nil {
		writeAPIError(w, http.StatusInternalServerError, "UPLOAD_WORKER_UNAVAILABLE", "Google Drive upload retry service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	res, err := h.Worker.RetrySession(r.Context(), s.SessionID, true)
	if err != nil {
		h.writeRetryError(w, err, res)
		return
	}
	h.record("cloud.upload_retry_started", activity.ResultSuccess, "Google Drive upload session retry requested")
	writeData(w, http.StatusAccepted, toRetryResultDTO(res))
}

type RetryResultDTO struct {
	SessionID     string         `json:"session_id"`
	UploadStatus  string         `json:"upload_status"`
	SessionStatus string         `json:"session_upload_status"`
	Jobs          []UploadJobDTO `json:"jobs"`
	Accepted      bool           `json:"accepted"`
	NoopReason    string         `json:"noop_reason,omitempty"`
}

func toRetryResultDTO(res upload.RetryResult) RetryResultDTO {
	jobs := toUploadJobDTOs(res.Jobs)
	sessionID := ""
	if len(jobs) > 0 {
		sessionID = jobs[0].SessionID
	}
	return RetryResultDTO{SessionID: sessionID, UploadStatus: res.SessionStatus, SessionStatus: res.SessionStatus, Jobs: jobs, Accepted: res.Accepted, NoopReason: res.NoopReason}
}

func toUploadJobDTOs(jobs []upload.FileUploadJob) []UploadJobDTO {
	out := make([]UploadJobDTO, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, UploadJobDTO{JobID: j.JobID, SessionID: j.SessionID, StationID: j.StationID, PhotoID: j.PhotoID, AssetKind: j.AssetKind, LocalFileName: filepath.Base(j.LocalPath), BucketName: j.BucketName, ObjectKey: j.ObjectKey, DriveFolderID: j.DriveFolderID, DriveFileID: j.DriveFileID, RemoteIdentity: j.RemoteIdentity, RemoteGeneration: j.RemoteGeneration, RemoteMetageneration: j.RemoteMetageneration, RemoteETag: j.RemoteETag, DedupeKey: j.DedupeKey, Status: j.Status, AttemptCount: j.AttemptCount, MaxAttempts: j.MaxAttempts, LastErrorCode: j.LastErrorCode, LastErrorAction: j.LastErrorAction, LastAttemptAt: j.LastAttemptAt, NextRetryAt: j.NextRetryAt, RetryAfterSeconds: j.RetryAfterSeconds, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt, UploadedAt: j.UploadedAt})
	}
	return out
}

func (h SessionUploadsHandler) writeRetryError(w http.ResponseWriter, err error, res upload.RetryResult) {
	var se upload.SafeUploadError
	code, action := upload.ErrorUploadFailed, upload.ActionRetryCloudUpload
	if upload.AsSafeUploadError(err, &se) {
		code, action = se.Code, se.Action
	}
	status := http.StatusConflict
	if code == upload.ErrorUploadJobNotFound {
		status = http.StatusNotFound
	} else if code == upload.ErrorUploadRetryStateSave {
		status = http.StatusInternalServerError
	}
	writeAPIErrorWithDetails(w, status, code, "Google Drive upload retry belum berhasil dimulai.", action, map[string]any{"session_upload_status": res.SessionStatus, "noop_reason": res.NoopReason})
}

func (h SessionUploadsHandler) getSession(w http.ResponseWriter, r *http.Request) (sessions.Session, bool) {
	if h.Sessions == nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session service belum siap.", "Restart aplikasi lalu coba lagi.")
		return sessions.Session{}, false
	}
	s, err := h.Sessions.Get(r.PathValue("session_id"))
	if err != nil {
		if errors.Is(err, sessions.ErrSessionNotFound) {
			writeAPIError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session tidak ditemukan.", "Refresh dashboard lalu pilih session yang tersedia.")
		} else {
			writeAPIError(w, http.StatusInternalServerError, "SESSION_LOAD_FAILED", "Session gagal dibaca.", "Coba lagi.")
		}
		return sessions.Session{}, false
	}
	return s, true
}
func (h SessionUploadsHandler) record(t string, r activity.Result, m string) {
	if h.Activity != nil {
		h.Activity.Record(t, r, m)
	}
}
func (h SessionUploadsHandler) publish(name string, s sessions.Session, j *upload.FileUploadJob) {
	if h.Broker == nil {
		return
	}
	payload := map[string]any{"session_id": s.SessionID, "station_id": s.StationID}
	if j != nil {
		payload["photo_id"] = j.PhotoID
		payload["asset_kind"] = j.AssetKind
		payload["status"] = j.Status
		payload["drive_folder_id"] = j.DriveFolderID
		payload["drive_file_id"] = j.DriveFileID
		payload["last_error_code"] = j.LastErrorCode
		payload["last_error_action"] = j.LastErrorAction
		payload["attempt_count"] = j.AttemptCount
		payload["max_attempts"] = j.MaxAttempts
		payload["retry_after"] = j.RetryAfterSeconds
		payload["next_retry_at"] = j.NextRetryAt
		payload["dedupe_key"] = j.DedupeKey
	}
	ev, err := events.New(name, "upload", s.SessionID, payload)
	if err == nil {
		h.Broker.Publish(ev)
	}
}
