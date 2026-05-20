package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

type StationConfigHandler struct {
	store         *stations.Store
	persistence   stations.Persistence
	activityStore *activity.Store
	broker        *events.Broker
}

type BackupData struct {
	Backup stations.BackupMetadata `json:"backup"`
}

type RestoreData struct {
	Restored     bool `json:"restored"`
	StationCount int  `json:"station_count"`
}

type RestoreRequest struct {
	Filename string `json:"filename"`
}

func NewStationConfigHandler(store *stations.Store, persistence stations.Persistence, activityStore *activity.Store, broker *events.Broker) StationConfigHandler {
	return StationConfigHandler{store: store, persistence: persistence, activityStore: activityStore, broker: broker}
}

func (h StationConfigHandler) Backup(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.record("station.config_backup_failed", activity.ResultFailure, "Backup station config gagal karena store belum siap.")
		writeAPIError(w, http.StatusInternalServerError, "STATION_CONFIG_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	backup, err := h.persistence.Backup(h.store)
	if err != nil {
		h.record("station.config_backup_failed", activity.ResultFailure, "Backup station config gagal.")
		writeAPIError(w, http.StatusInternalServerError, "STATION_BACKUP_FAILED", "Backup station config gagal dibuat.", "Coba backup ulang atau periksa folder local-data/config.")
		return
	}
	h.record("station.config_backup_created", activity.ResultSuccess, "Backup station config berhasil dibuat: "+backup.Filename)
	writeData(w, http.StatusOK, BackupData{Backup: backup})
}

func (h StationConfigHandler) Restore(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.record("station.config_restore_failed", activity.ResultFailure, "Restore station config gagal karena store belum siap.")
		writeAPIError(w, http.StatusInternalServerError, "STATION_CONFIG_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	request, ok := decodeRestoreRequest(w, r)
	if !ok {
		h.record("station.config_restore_failed", activity.ResultFailure, "Restore station config gagal karena request tidak valid.")
		return
	}
	count, err := h.persistence.Restore(h.store, request.Filename)
	if err != nil {
		h.record("station.config_restore_failed", activity.ResultFailure, "Restore station config gagal.")
		if errors.Is(err, stations.ErrInvalidBackup) || errors.Is(err, stations.ErrInvalidStationConfig) || errors.Is(err, stations.ErrDuplicateInputFolder) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_STATION_BACKUP", "Backup station config tidak valid.", "Pilih backup lain atau buat ulang konfigurasi station.")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "STATION_RESTORE_FAILED", "Restore station config gagal.", "Coba restore ulang atau periksa file backup.")
		return
	}
	h.record("station.config_restored", activity.ResultSuccess, "Station config berhasil direstore dari backup: "+request.Filename)
	h.publishRestored(count)
	writeData(w, http.StatusOK, RestoreData{Restored: true, StationCount: count})
}

func decodeRestoreRequest(w http.ResponseWriter, r *http.Request) (RestoreRequest, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type restore harus application/json.", "Kirim request dengan Content-Type application/json.")
		return RestoreRequest{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxStationBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request RestoreRequest
	if err := decoder.Decode(&request); err != nil || request.Filename == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request restore tidak valid.", "Kirim filename backup yang valid.")
		return RestoreRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request restore tidak valid.", "Kirim satu payload JSON restore yang valid.")
		return RestoreRequest{}, false
	}
	return request, true
}

func (h StationConfigHandler) record(actionType string, result activity.Result, message string) {
	if h.activityStore == nil {
		return
	}
	h.activityStore.RecordWithRefs(actionType, result, message, nil, nil)
}

func (h StationConfigHandler) publishRestored(count int) {
	if h.broker == nil {
		return
	}
	event, err := events.New("stations.restored", "station_config", "all", map[string]any{"station_count": count})
	if err != nil {
		return
	}
	h.broker.Publish(event)
}
