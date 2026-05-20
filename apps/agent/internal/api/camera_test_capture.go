package api

import (
	"errors"
	"net/http"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/stations"
)

type CameraTestCaptureHandler struct {
	store         *stations.Store
	activityStore *activity.Store
	broker        *events.Broker
	supervisor    *cameras.TetherSupervisor
	service       *cameras.TestCaptureService
}

type CameraTestCaptureData struct {
	TestCapture cameras.TestCaptureResult `json:"test_capture"`
}

type safeCameraTestCaptureErrorDetails struct {
	Status   cameras.TestCaptureStatus `json:"status"`
	Action   string                    `json:"action"`
	FileName string                    `json:"file_name,omitempty"`
}

func NewCameraTestCaptureHandler(store *stations.Store, activityStore *activity.Store, broker *events.Broker, supervisor *cameras.TetherSupervisor, service *cameras.TestCaptureService) CameraTestCaptureHandler {
	if supervisor == nil {
		supervisor = cameras.NewTetherSupervisor(nil)
	}
	if service == nil {
		service = cameras.NewTestCaptureService(nil)
	}
	return CameraTestCaptureHandler{store: store, activityStore: activityStore, broker: broker, supervisor: supervisor, service: service}
}

func (h CameraTestCaptureHandler) Run(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if h.store == nil || h.service == nil {
		writeAPIError(w, http.StatusInternalServerError, "CAMERA_TEST_CAPTURE_UNAVAILABLE", "Test capture belum siap.", "RETRY_TEST_CAPTURE")
		return
	}
	station, err := h.store.Get(stationID)
	if err != nil {
		if errors.Is(err, stations.ErrStationNotFound) {
			writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "RECHECK_CAMERA_READINESS")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "CAMERA_TEST_CAPTURE_UNAVAILABLE", "Station tidak bisa dibaca.", "RETRY_TEST_CAPTURE")
		return
	}
	var assignment *cameras.TestCaptureAssignment
	if station.CameraAssignment != nil {
		assignment = &cameras.TestCaptureAssignment{IdentityKey: station.CameraAssignment.IdentityKey, CameraName: station.CameraAssignment.CameraName, Port: station.CameraAssignment.Port, Runtime: cameras.Runtime(station.CameraAssignment.Runtime)}
	}
	listener := cameras.TetherListener{StationID: stationID, Status: cameras.TetherStatusStopped, LastErrorAction: cameras.ActionStartTetherListener}
	if h.supervisor != nil {
		listener = h.supervisor.Status(stationID)
	}
	result, err := h.service.Run(r.Context(), cameras.TestCaptureConfig{StationID: station.StationID, InputFolder: station.InputFolder, Assignment: assignment, Listener: listener})
	if err != nil {
		h.record(stationID, false, result.Label)
		h.publish(stationID, result)
		h.writeError(w, err, result)
		return
	}
	h.record(stationID, true, "Camera test capture berhasil dan tervalidasi watcher.")
	h.publish(stationID, result)
	writeData(w, http.StatusOK, CameraTestCaptureData{TestCapture: result})
}

func (h CameraTestCaptureHandler) writeError(w http.ResponseWriter, err error, result cameras.TestCaptureResult) {
	status := http.StatusBadRequest
	code := "CAMERA_TEST_CAPTURE_FAILED"
	switch {
	case errors.Is(err, cameras.ErrTestCaptureAssignmentRequired):
		code = "CAMERA_ASSIGNMENT_REQUIRED"
	case errors.Is(err, cameras.ErrTestCaptureInputUnwritable):
		code = "STATION_INPUT_FOLDER_UNWRITABLE"
	case errors.Is(err, cameras.ErrTestCaptureListenerConflict):
		code = "TETHER_LISTENER_CONFLICT"
	case errors.Is(err, cameras.ErrTestCaptureBusy):
		status = http.StatusConflict
		code = "CAMERA_TEST_CAPTURE_RUNNING"
	case errors.Is(err, cameras.ErrTestCaptureTimeout):
		code = "CAMERA_TEST_CAPTURE_TIMEOUT"
	case errors.Is(err, cameras.ErrTestCaptureInvalidJPG):
		code = "CAMERA_TEST_CAPTURE_INVALID_JPG"
	}
	details := map[string]any{"status": result.Status, "action": string(result.Action)}
	if fileName := safeBaseName(result.FileName); fileName != "" {
		details["file_name"] = fileName
	}
	writeAPIErrorWithDetails(w, status, code, result.Label, string(result.Action), details)
}

func safeBaseName(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' || name[i] == '\\' {
			return name[i+1:]
		}
	}
	return name
}

func (h CameraTestCaptureHandler) record(stationID string, success bool, message string) {
	if h.activityStore == nil {
		return
	}
	result := activity.ResultSuccess
	actionType := "station.camera_test_capture_completed"
	if !success {
		result = activity.ResultFailure
		actionType = "station.camera_test_capture_failed"
	}
	h.activityStore.RecordWithRefs(actionType, result, message, &stationID, nil)
}

func (h CameraTestCaptureHandler) publish(stationID string, result cameras.TestCaptureResult) {
	if h.broker == nil {
		return
	}
	event, err := events.New("station.camera_test_capture_completed", "station", stationID, map[string]any{"test_capture": result})
	if err == nil {
		h.broker.Publish(event)
	}
}
