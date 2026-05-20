package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

type CameraTetherHandler struct {
	store          *stations.Store
	supervisor     *cameras.TetherSupervisor
	stateStore     *cameras.TetherStateStore
	recovery       *cameras.TetherRecoveryCoordinator
	camerasHandler *CamerasHandler
}

type TetherListenerData struct {
	Listener cameras.TetherListener         `json:"listener"`
	Settings cameras.TetherListenerSettings `json:"settings"`
	Recovery cameras.TetherRecoveryStatus   `json:"recovery"`
}

type tetherSettingsRequest struct {
	AutoRestartEnabled bool `json:"auto_restart_enabled"`
}

func NewCameraTetherHandler(store *stations.Store, supervisor *cameras.TetherSupervisor, camerasHandler *CamerasHandler) CameraTetherHandler {
	return NewCameraTetherHandlerWithRecovery(store, supervisor, nil, nil, camerasHandler)
}

func NewCameraTetherHandlerWithRecovery(store *stations.Store, supervisor *cameras.TetherSupervisor, stateStore *cameras.TetherStateStore, recovery *cameras.TetherRecoveryCoordinator, camerasHandler *CamerasHandler) CameraTetherHandler {
	if supervisor == nil {
		supervisor = cameras.NewTetherSupervisor(nil)
	}
	return CameraTetherHandler{store: store, supervisor: supervisor, stateStore: stateStore, recovery: recovery, camerasHandler: camerasHandler}
}

func (h CameraTetherHandler) Get(w http.ResponseWriter, r *http.Request) {
	stationID, ok := h.validateStationScope(w, r.PathValue("station_id"))
	if !ok {
		return
	}
	writeData(w, http.StatusOK, h.data(stationID, h.supervisor.Status(stationID)))
}

func (h CameraTetherHandler) Start(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.configForStation(w, r.PathValue("station_id"))
	if !ok {
		return
	}
	listener, err := h.supervisor.Start(r.Context(), cfg)
	if err != nil {
		h.writeStartError(w, cfg.StationID, listener, err)
		return
	}
	h.markDesiredRunning(cfg.StationID)
	if listener.AlreadyRunning {
		h.record(cfg.StationID, "station.tether_listener_start_noop", true, "Tether listener sudah berjalan untuk station ini.")
	} else {
		h.record(cfg.StationID, "station.tether_listener_started", true, "Tether listener station berhasil dimulai.")
	}
	h.publish(listener)
	writeData(w, http.StatusOK, h.data(cfg.StationID, listener))
}

func (h CameraTetherHandler) Stop(w http.ResponseWriter, r *http.Request) {
	stationID, ok := h.validateStationScope(w, r.PathValue("station_id"))
	if !ok {
		return
	}
	listener, err := h.supervisor.Stop(stationID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "TETHER_LISTENER_STOP_FAILED", "Tether listener gagal dihentikan.", string(cameras.ActionStopTetherListener))
		return
	}
	h.markDesiredStopped(stationID)
	if h.recovery != nil {
		h.recovery.Cancel(stationID)
	}
	h.record(stationID, "station.tether_listener_stopped", true, "Tether listener station dihentikan.")
	h.publish(listener)
	writeData(w, http.StatusOK, h.data(stationID, listener))
}

func (h CameraTetherHandler) Retry(w http.ResponseWriter, r *http.Request) {
	cfg, ok := h.configForStation(w, r.PathValue("station_id"))
	if !ok {
		return
	}
	h.markDesiredRunning(cfg.StationID)
	if h.recovery != nil {
		h.recovery.ManualRetry(cfg.StationID)
	}
	listener, err := h.supervisor.Start(r.Context(), cfg)
	if err != nil {
		if h.recovery != nil {
			h.recovery.MarkManualAttemptFailed(cfg.StationID, listener.LastErrorCode, listener.LastErrorAction, "Retry tether listener gagal aman; ikuti next action di dashboard.")
		}
		h.writeStartError(w, cfg.StationID, listener, err)
		return
	}
	h.record(cfg.StationID, "station.tether_reconnect_attempted", true, "Retry tether listener dijalankan aman oleh operator.")
	h.publish(listener)
	writeData(w, http.StatusOK, h.data(cfg.StationID, listener))
}

func (h CameraTetherHandler) PutSettings(w http.ResponseWriter, r *http.Request) {
	stationID, ok := h.validateStationScope(w, r.PathValue("station_id"))
	if !ok {
		return
	}
	var body tetherSettingsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_TETHER_SETTINGS", "Setting tether listener tidak valid.", string(cameras.ActionRetryTetherListener))
		return
	}
	settings, err := h.updateSettings(stationID, func(s cameras.TetherListenerSettings) cameras.TetherListenerSettings {
		s.AutoRestartEnabled = body.AutoRestartEnabled
		return s
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "TETHER_SETTINGS_SAVE_FAILED", "Setting tether listener gagal disimpan.", string(cameras.ActionRetryTetherListener))
		return
	}
	if !settings.AutoRestartEnabled && h.recovery != nil {
		h.recovery.Cancel(stationID)
	}
	listener := h.supervisor.Status(stationID)
	h.record(stationID, "station.tether_settings_updated", true, "Setting auto-restart tether diperbarui.")
	h.publish(listener)
	writeData(w, http.StatusOK, TetherListenerData{Listener: listener, Settings: settings, Recovery: h.recoveryStatus(stationID)})
}

func (h CameraTetherHandler) configForStation(w http.ResponseWriter, stationID string) (cameras.TetherStationConfig, bool) {
	station, ok := h.lookupStation(w, stationID)
	if !ok {
		return cameras.TetherStationConfig{}, false
	}
	var assignment *cameras.TetherAssignment
	if station.CameraAssignment != nil {
		assignment = &cameras.TetherAssignment{IdentityKey: station.CameraAssignment.IdentityKey, CameraName: station.CameraAssignment.CameraName, Port: station.CameraAssignment.Port, Runtime: cameras.Runtime(station.CameraAssignment.Runtime)}
	}
	return cameras.TetherStationConfig{StationID: station.StationID, InputFolder: station.InputFolder, Assignment: assignment}, true
}

func (h CameraTetherHandler) validateStationScope(w http.ResponseWriter, stationID string) (string, bool) {
	station, ok := h.lookupStation(w, stationID)
	if !ok {
		return "", false
	}
	return station.StationID, true
}

func (h CameraTetherHandler) lookupStation(w http.ResponseWriter, stationID string) (stations.Station, bool) {
	if h.store == nil {
		writeAPIError(w, http.StatusInternalServerError, "STATIONS_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return stations.Station{}, false
	}
	station, err := h.store.Get(stationID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
		return stations.Station{}, false
	}
	return station, true
}

func (h CameraTetherHandler) writeStartError(w http.ResponseWriter, stationID string, listener cameras.TetherListener, err error) {
	h.record(stationID, "station.tether_listener_failed", false, listener.Message)
	status := http.StatusBadRequest
	code := listener.LastErrorCode
	if code == "" {
		code = "TETHER_LISTENER_START_FAILED"
	}
	action := string(listener.LastErrorAction)
	if action == "" {
		action = string(cameras.ActionRetryTetherListener)
	}
	if errors.Is(err, cameras.ErrTetherAssignmentRequired) {
		code = "CAMERA_ASSIGNMENT_REQUIRED"
	}
	if errors.Is(err, cameras.ErrTetherInputFolderUnwritable) {
		code = "STATION_INPUT_FOLDER_UNWRITABLE"
	}
	writeAPIErrorWithDetails(w, status, code, cameras.SanitizeTetherDiagnostic(listener.Message), action, map[string]any{"listener": safeListenerDetails(listener)})
}

func (h CameraTetherHandler) markDesiredRunning(stationID string) {
	_, _ = h.updateSettings(stationID, func(s cameras.TetherListenerSettings) cameras.TetherListenerSettings {
		now := time.Now().UTC()
		s.DesiredState = cameras.TetherDesiredRunning
		s.LastStartedAt = &now
		return s
	})
}

func (h CameraTetherHandler) markDesiredStopped(stationID string) {
	_, _ = h.updateSettings(stationID, func(s cameras.TetherListenerSettings) cameras.TetherListenerSettings {
		now := time.Now().UTC()
		s.DesiredState = cameras.TetherDesiredStopped
		s.LastStoppedAt = &now
		return s
	})
}

func (h CameraTetherHandler) updateSettings(stationID string, mutate func(cameras.TetherListenerSettings) cameras.TetherListenerSettings) (cameras.TetherListenerSettings, error) {
	if h.stateStore == nil {
		return mutate(cameras.TetherListenerSettings{StationID: stationID, DesiredState: cameras.TetherDesiredStopped, UpdatedAt: time.Now().UTC()}), nil
	}
	return h.stateStore.Update(stationID, mutate)
}

func (h CameraTetherHandler) settings(stationID string) cameras.TetherListenerSettings {
	if h.stateStore == nil {
		return cameras.TetherListenerSettings{StationID: stationID, DesiredState: cameras.TetherDesiredStopped, UpdatedAt: time.Now().UTC()}
	}
	settings, err := h.stateStore.Get(stationID)
	if err != nil {
		return cameras.TetherListenerSettings{StationID: stationID, DesiredState: cameras.TetherDesiredStopped, UpdatedAt: time.Now().UTC()}
	}
	return settings
}

func (h CameraTetherHandler) recoveryStatus(stationID string) cameras.TetherRecoveryStatus {
	if h.recovery == nil {
		return cameras.TetherRecoveryStatus{StationID: stationID, Status: cameras.TetherRecoveryIdle, Message: "Tidak ada recovery tether aktif.", UpdatedAt: time.Now().UTC()}
	}
	return h.recovery.Status(stationID)
}

func (h CameraTetherHandler) data(stationID string, listener cameras.TetherListener) TetherListenerData {
	return TetherListenerData{Listener: listener, Settings: h.settings(stationID), Recovery: h.recoveryStatus(stationID)}
}

func safeListenerDetails(listener cameras.TetherListener) map[string]any {
	return map[string]any{"station_id": listener.StationID, "status": listener.Status, "last_error_code": listener.LastErrorCode, "last_error_action": listener.LastErrorAction, "message": cameras.SanitizeTetherDiagnostic(listener.Message)}
}

func (h CameraTetherHandler) record(stationID, action string, success bool, message string) {
	if h.camerasHandler == nil {
		return
	}
	if h.camerasHandler.activityStore == nil {
		return
	}
	result := activity.ResultSuccess
	if !success {
		result = activity.ResultFailure
	}
	h.camerasHandler.activityStore.RecordWithRefs(action, result, cameras.SanitizeTetherDiagnostic(message), &stationID, nil)
}
func (h CameraTetherHandler) publish(listener cameras.TetherListener) {
	if h.camerasHandler == nil || h.camerasHandler.broker == nil {
		return
	}
	event, err := events.New("camera.tether_listener_updated", "station", listener.StationID, map[string]any{"listener": listener})
	if err == nil {
		h.camerasHandler.broker.Publish(event)
	}
}
