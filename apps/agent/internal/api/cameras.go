package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

const maxCameraAssignmentBodyBytes = 4096

type CamerasHandler struct {
	discovery     cameras.DiscoveryService
	store         *stations.Store
	persistence   *stations.Persistence
	activityStore *activity.Store
	broker        *events.Broker
}

type GPhoto2DiscoveryData struct {
	Status      cameras.DiscoveryStatus  `json:"status"`
	Action      cameras.SafeAction       `json:"action"`
	Runtime     cameras.Runtime          `json:"runtime"`
	Cameras     []cameras.DetectedCamera `json:"cameras"`
	Diagnostics []string                 `json:"diagnostics"`
	ScannedAt   string                   `json:"scanned_at"`
}

type CameraAssignmentData struct {
	Station stations.Station `json:"station"`
}

func NewCamerasHandler(store *stations.Store, persistence *stations.Persistence, activityStore *activity.Store, broker *events.Broker, discovery cameras.DiscoveryService) CamerasHandler {
	return CamerasHandler{store: store, persistence: persistence, activityStore: activityStore, broker: broker, discovery: discovery}
}

func (h CamerasHandler) Discover(w http.ResponseWriter, r *http.Request) {
	result, err := h.discovery.Discover(r.Context())
	if err != nil {
		h.recordDiscovery(false, "gPhoto2 camera discovery gagal dijalankan.")
		writeAPIError(w, http.StatusInternalServerError, "GPHOTO2_DISCOVERY_FAILED", "Camera discovery gagal dijalankan.", string(cameras.ActionRetryDiscovery))
		return
	}
	if result.Status == cameras.DiscoveryStatusGPhoto2Missing {
		h.recordDiscovery(false, "gPhoto2 camera discovery gagal karena runtime belum tersedia.")
		writeAPIErrorWithDetails(w, http.StatusServiceUnavailable, "GPHOTO2_UNAVAILABLE", "gPhoto2 belum tersedia untuk camera discovery.", string(result.Action), map[string]any{"runtime": result.Runtime, "diagnostics": result.Diagnostics})
		return
	}
	if result.Status == cameras.DiscoveryStatusWSLMissing {
		h.recordDiscovery(false, "gPhoto2 camera discovery gagal karena WSL belum tersedia.")
		writeAPIErrorWithDetails(w, http.StatusServiceUnavailable, "WSL_UNAVAILABLE", "WSL belum tersedia untuk camera discovery.", string(result.Action), map[string]any{"runtime": result.Runtime, "diagnostics": result.Diagnostics})
		return
	}
	data := GPhoto2DiscoveryData{Status: result.Status, Action: result.Action, Runtime: result.Runtime, Cameras: result.Cameras, Diagnostics: result.Diagnostics, ScannedAt: result.ScannedAt.Format("2006-01-02T15:04:05.000000000Z")}
	h.recordDiscovery(true, "gPhoto2 camera discovery selesai.")
	h.publishDiscovery(data)
	writeData(w, http.StatusOK, data)
}

func (h CamerasHandler) Assign(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if h.store == nil {
		writeAPIError(w, http.StatusInternalServerError, "STATIONS_UNAVAILABLE", "Konfigurasi station belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	request, ok := decodeCameraAssignmentRequest(w, r)
	if !ok {
		return
	}
	previous := h.store.List()
	station, err := h.store.UpdateCameraAssignment(stationID, request)
	if err != nil {
		h.handleAssignError(w, stationID, err)
		return
	}
	if h.persistence != nil {
		if err := h.persistence.Save(h.store); err != nil {
			_ = h.store.ReplaceAll(previous)
			writeAPIError(w, http.StatusInternalServerError, "STATION_CONFIG_UNAVAILABLE", "Camera assignment gagal disimpan ke local storage.", "Coba simpan ulang atau periksa folder local-data/config.")
			return
		}
	}
	h.recordAssignment(stationID, true, "Camera assignment station berhasil disimpan.")
	h.publishStationUpdated(station)
	writeData(w, http.StatusOK, CameraAssignmentData{Station: station})
}

func decodeCameraAssignmentRequest(w http.ResponseWriter, r *http.Request) (stations.UpdateCameraAssignment, bool) {
	if r.Body == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_CAMERA_ASSIGNMENT", "Body camera assignment wajib dikirim.", "Pilih kamera lalu coba simpan lagi.")
		return stations.UpdateCameraAssignment{}, false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type camera assignment harus application/json.", "Kirim request dengan Content-Type application/json.")
		return stations.UpdateCameraAssignment{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCameraAssignmentBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request stations.UpdateCameraAssignment
	if err := decoder.Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_CAMERA_ASSIGNMENT", "Format camera assignment tidak valid.", "Pilih kamera yang valid lalu coba lagi.")
		return stations.UpdateCameraAssignment{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_CAMERA_ASSIGNMENT", "Format camera assignment tidak valid.", "Kirim satu payload JSON yang valid.")
		return stations.UpdateCameraAssignment{}, false
	}
	return request, true
}

func (h CamerasHandler) handleAssignError(w http.ResponseWriter, stationID string, err error) {
	if errors.Is(err, stations.ErrStationNotFound) {
		h.recordAssignment(stationID, false, "Camera assignment gagal karena station tidak ditemukan.")
		writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
		return
	}
	if errors.Is(err, stations.ErrDuplicateCameraAssignment) {
		h.recordAssignment(stationID, false, "Camera assignment gagal karena kamera sudah dipakai station lain.")
		writeAPIError(w, http.StatusConflict, "DUPLICATE_CAMERA_ASSIGNMENT", "Kamera sudah dipakai station lain.", string(cameras.ActionChooseDifferent))
		return
	}
	writeAPIError(w, http.StatusBadRequest, "INVALID_CAMERA_ASSIGNMENT", "Camera assignment tidak valid.", "Pilih kamera yang valid lalu coba lagi.")
}

func (h CamerasHandler) recordDiscovery(success bool, message string) {
	if h.activityStore == nil {
		return
	}
	result := activity.ResultSuccess
	actionType := "camera.discovery_completed"
	if !success {
		result = activity.ResultFailure
		actionType = "camera.discovery_failed"
	}
	h.activityStore.Record(actionType, result, message)
}

func (h CamerasHandler) recordAssignment(stationID string, success bool, message string) {
	if h.activityStore == nil {
		return
	}
	result := activity.ResultSuccess
	actionType := "station.camera_assignment_updated"
	if !success {
		result = activity.ResultFailure
		actionType = "station.camera_assignment_failed"
	}
	h.activityStore.RecordWithRefs(actionType, result, message, &stationID, nil)
}

func (h CamerasHandler) publishDiscovery(data GPhoto2DiscoveryData) {
	if h.broker == nil {
		return
	}
	event, err := events.New("camera.discovery_completed", "camera", "gphoto2", map[string]any{"discovery": data})
	if err == nil {
		h.broker.Publish(event)
	}
}

func (h CamerasHandler) publishStationUpdated(station stations.Station) {
	if h.broker == nil {
		return
	}
	event, err := events.New("station.updated", "station", station.StationID, map[string]any{"station": station})
	if err == nil {
		h.broker.Publish(event)
	}
}
