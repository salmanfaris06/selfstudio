package api

import (
	"encoding/json"
	"net/http"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/events"
)

type cloudSettingsStore interface {
	LoadOrDefault() (cloud.Settings, error)
	Save(cloud.Settings) error
}

type CloudSettingsHandler struct {
	Store    cloudSettingsStore
	Checker  cloud.Checker
	Activity *activity.Store
	Broker   *events.Broker
}

func NewCloudSettingsHandler(store cloudSettingsStore, checker cloud.Checker, activityStore *activity.Store, broker *events.Broker) CloudSettingsHandler {
	return CloudSettingsHandler{Store: store, Checker: checker, Activity: activityStore, Broker: broker}
}

func (h CloudSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.Store.LoadOrDefault()
	if err != nil {
		writeAPIError(w, 500, "CLOUD_CONFIG_READ_FAILED", "Cloud settings tidak bisa dibaca.", cloud.ActionRetryCheck)
		return
	}
	writeData(w, 200, s.Public())
}
func (h CloudSettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req cloud.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, 400, "INVALID_JSON", "Request JSON tidak valid.", "FIX_REQUEST")
		return
	}
	current, err := h.Store.LoadOrDefault()
	if err != nil {
		writeAPIError(w, 500, "CLOUD_CONFIG_READ_FAILED", "Cloud settings tidak bisa dibaca.", cloud.ActionRetryCheck)
		return
	}
	s := current
	if req.Provider != "" {
		s.Provider = req.Provider
	} else {
		s.Provider = cloud.ProviderGoogleDrive
	}
	s.BucketName = ""
	s.TargetRootPrefix = ""
	s.DriveRootFolderID = req.DriveRootFolderID
	s.DriveRootFolderName = req.DriveRootFolderName
	s.FolderNamingTemplate = cloud.FolderNamingTemplate
	if req.ServiceAccountJSON != "" {
		s.ServiceAccountJSON = req.ServiceAccountJSON
		s.CredentialFilePath = ""
	} else if req.CredentialFilePath != "" {
		s.CredentialFilePath = req.CredentialFilePath
		s.ServiceAccountJSON = ""
	}
	if err := cloud.ValidateSettings(s); err != nil {
		code, msg, action := cloud.MapValidationError(err)
		writeAPIError(w, 400, code, msg, action)
		return
	}
	s.ConnectionStatus = cloud.StatusNotConfigured
	s.LastError = ""
	s.LastErrorCode = ""
	s.LastErrorAction = ""
	s.LastCheckedAt = nil
	if err := h.Store.Save(s); err != nil {
		writeAPIError(w, 500, "CLOUD_CONFIG_SAVE_FAILED", "Cloud settings tidak bisa disimpan.", cloud.ActionRetryCheck)
		return
	}
	h.record("cloud.settings_updated", activity.ResultSuccess, "Cloud settings updated")
	h.publish(s)
	writeData(w, 200, s.Public())
}
func (h CloudSettingsHandler) Check(w http.ResponseWriter, r *http.Request) {
	s, err := h.Store.LoadOrDefault()
	if err != nil {
		writeAPIError(w, 500, "CLOUD_CONFIG_READ_FAILED", "Cloud settings tidak bisa dibaca.", cloud.ActionRetryCheck)
		return
	}
	now := time.Now().UTC()
	res := h.Checker.Check(r.Context(), s)
	s.ConnectionStatus = res.Status
	s.LastCheckedAt = &now
	s.LastError = res.ErrorMessage
	s.LastErrorCode = res.ErrorCode
	s.LastErrorAction = res.ErrorAction
	if res.Status == cloud.StatusAuthorized {
		s.LastError = ""
		s.LastErrorCode = ""
		s.LastErrorAction = ""
	}
	if err := h.Store.Save(s); err != nil {
		writeAPIErrorWithDetails(w, 500, "CLOUD_CONFIG_SAVE_FAILED", "Hasil connection check tidak bisa disimpan.", cloud.ActionRetryCheck, map[string]any{"connection_status": s.ConnectionStatus})
		return
	}
	result := activity.ResultFailure
	if res.Status == cloud.StatusAuthorized {
		result = activity.ResultSuccess
	}
	h.record("cloud.connection_checked", result, "Cloud connection check completed")
	h.publish(s)
	if res.Status != cloud.StatusAuthorized {
		writeAPIErrorWithDetails(w, 400, res.ErrorCode, res.ErrorMessage, res.ErrorAction, map[string]any{"connection_status": s.ConnectionStatus})
		return
	}
	writeData(w, 200, s.Public())
}
func (h CloudSettingsHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var in cloud.PreviewInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	p, err := cloud.BuildObjectKey(in)
	if err != nil {
		c, m, a := cloud.MapValidationError(err)
		writeAPIError(w, 400, c, m, a)
		return
	}
	writeData(w, 200, p)
}

func (h CloudSettingsHandler) FolderPreview(w http.ResponseWriter, r *http.Request) {
	var in cloud.PreviewInput
	_ = json.NewDecoder(r.Body).Decode(&in)
	p, err := cloud.BuildDriveFolderPreview(in)
	if err != nil {
		c, m, a := cloud.MapValidationError(err)
		writeAPIError(w, 400, c, m, a)
		return
	}
	writeData(w, 200, p)
}
func (h CloudSettingsHandler) record(t string, r activity.Result, m string) {
	if h.Activity != nil {
		h.Activity.Record(t, r, m)
	}
}
func (h CloudSettingsHandler) publish(s cloud.Settings) {
	if h.Broker == nil {
		return
	}
	ev, err := events.New("cloud.status_updated", "cloud", "settings", map[string]any{"provider": s.Public().Provider, "drive_root_folder_id": s.DriveRootFolderID, "drive_root_folder_name": s.DriveRootFolderName, "folder_naming_template": cloud.FolderNamingTemplate, "credentials_configured": s.CredentialsConfigured(), "connection_status": s.ConnectionStatus, "last_checked_at": s.LastCheckedAt, "last_error_code": s.LastErrorCode, "last_error_action": s.LastErrorAction})
	if err == nil {
		h.Broker.Publish(ev)
	}
}
