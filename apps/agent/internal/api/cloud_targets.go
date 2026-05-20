package api

import (
	"errors"
	"net/http"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/upload"
)

type CloudTargetHandler struct {
	Sessions *sessions.Store
	Resolver upload.Resolver
	Targets  *upload.Store
	Activity *activity.Store
	Broker   *events.Broker
}

type SessionCloudTargetData struct {
	CloudTarget upload.SessionCloudTarget `json:"cloud_target"`
}

func NewCloudTargetHandler(sessionStore *sessions.Store, resolver upload.Resolver, targets *upload.Store, activityStore *activity.Store, broker *events.Broker) CloudTargetHandler {
	return CloudTargetHandler{Sessions: sessionStore, Resolver: resolver, Targets: targets, Activity: activityStore, Broker: broker}
}

func (h CloudTargetHandler) Get(w http.ResponseWriter, r *http.Request) {
	session, ok := h.getSession(w, r)
	if !ok {
		return
	}
	if h.Targets != nil {
		if t, found := h.Targets.Get(session.SessionID); found {
			writeData(w, http.StatusOK, SessionCloudTargetData{CloudTarget: t})
			return
		}
	}
	writeData(w, http.StatusOK, SessionCloudTargetData{CloudTarget: upload.SessionCloudTarget{SessionID: session.SessionID, StationID: session.StationID, Status: upload.StatusPending}})
}

func (h CloudTargetHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	session, ok := h.getSession(w, r)
	if !ok {
		return
	}
	if session.Status != sessions.StatusLocked {
		writeAPIError(w, http.StatusConflict, upload.ErrorPendingLocalCompletion, "Session belum terkunci/local complete; cloud target belum dibuat.", upload.ActionWaitLocalCompletion)
		return
	}
	resolver := h.Resolver
	if resolver.Targets == nil {
		resolver.Targets = h.Targets
	}
	t, err := resolver.ResolveForSession(r.Context(), session)
	if err != nil {
		h.record("cloud.target_failed", activity.ResultFailure, "Google Drive folder target resolution failed")
		h.publish("cloud.target_failed", t)
		code, action := upload.ErrorPrefixResolveFailed, upload.ActionRetryTarget
		var targetErr upload.CloudTargetError
		if errors.As(err, &targetErr) {
			code, action = targetErr.Code, targetErr.Action
		} else if t.LastErrorCode != "" {
			code, action = t.LastErrorCode, t.LastErrorAction
		}
		writeAPIErrorWithDetails(w, http.StatusConflict, code, "Google Drive folder target belum siap.", action, map[string]any{"status": t.Status, "session_id": t.SessionID})
		return
	}
	h.record("cloud.target_resolved", activity.ResultSuccess, "Google Drive folder target resolved")
	h.publish("cloud.target_resolved", t)
	writeData(w, http.StatusOK, SessionCloudTargetData{CloudTarget: t})
}

func (h CloudTargetHandler) getSession(w http.ResponseWriter, r *http.Request) (sessions.Session, bool) {
	if h.Sessions == nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSIONS_UNAVAILABLE", "Session service belum siap.", "Restart aplikasi lalu coba lagi.")
		return sessions.Session{}, false
	}
	session, err := h.Sessions.Get(r.PathValue("session_id"))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session tidak ditemukan.", "Refresh dashboard lalu pilih session yang tersedia.")
		return sessions.Session{}, false
	}
	return session, true
}
func (h CloudTargetHandler) record(t string, r activity.Result, m string) {
	if h.Activity != nil {
		h.Activity.Record(t, r, m)
	}
}
func (h CloudTargetHandler) publish(name string, t upload.SessionCloudTarget) {
	if h.Broker == nil {
		return
	}
	ev, err := events.New(name, "cloud", t.SessionID, map[string]any{"session_id": t.SessionID, "station_id": t.StationID, "status": t.Status, "drive_root_folder_id": t.DriveRootFolderID, "drive_root_folder_name": t.DriveRootFolderName, "drive_folder_path": t.DriveFolderPath, "drive_session_folder_id": t.DriveSessionFolderID, "remote_identity": t.RemoteIdentity, "last_error_code": t.LastErrorCode, "last_error_action": t.LastErrorAction})
	if err == nil {
		h.Broker.Publish(ev)
	}
}
