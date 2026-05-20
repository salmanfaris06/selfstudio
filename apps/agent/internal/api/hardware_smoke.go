package api

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
	"selfstudio/agent/internal/upload"
)

const maxHardwareSmokeBodyBytes = 8 * 1024

type HardwareSmokeHandler struct {
	stationStore  *stations.Store
	activityStore *activity.Store
	broker        *events.Broker
	runner        *cameras.HardwareSmokeRunner
}

type HardwareSmokeData struct {
	Report cameras.HardwareSmokeReport `json:"report"`
}

func NewHardwareSmokeHandler(stationStore *stations.Store, activityStore *activity.Store, broker *events.Broker, runner *cameras.HardwareSmokeRunner) HardwareSmokeHandler {
	return HardwareSmokeHandler{stationStore: stationStore, activityStore: activityStore, broker: broker, runner: runner}
}

func (h HardwareSmokeHandler) Run(w http.ResponseWriter, r *http.Request) {
	stationID := r.PathValue("station_id")
	if h.stationStore == nil || h.runner == nil {
		writeAPIError(w, http.StatusInternalServerError, "HARDWARE_SMOKE_UNAVAILABLE", "Hardware smoke service belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type harus application/json.", "Kirim request JSON dari dashboard.")
		return
	}
	var req cameras.HardwareSmokeRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxHardwareSmokeBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_HARDWARE_SMOKE_REQUEST", "Request hardware smoke tidak valid.", "Pilih mode local-only atau Drive verify lalu coba lagi.")
		return
	}
	station, err := h.stationStore.Get(stationID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "STATION_NOT_FOUND", "Station tidak ditemukan.", "Refresh dashboard lalu pilih station yang tersedia.")
		return
	}
	smokeStation := cameras.HardwareSmokeStation{StationID: station.StationID, StationName: station.Name, InputFolder: station.InputFolder}
	if station.CameraAssignment != nil {
		smokeStation.Assignment = &cameras.TetherAssignment{IdentityKey: station.CameraAssignment.IdentityKey, CameraName: station.CameraAssignment.CameraName, Port: station.CameraAssignment.Port, Runtime: cameras.Runtime(station.CameraAssignment.Runtime)}
	}
	report, runErr := h.runner.Run(r.Context(), smokeStation, req)
	if runErr != nil {
		h.record(stationID, false)
		h.publish(report)
		writeData(w, http.StatusAccepted, HardwareSmokeData{Report: report})
		return
	}
	h.record(stationID, true)
	h.publish(report)
	writeData(w, http.StatusOK, HardwareSmokeData{Report: report})
}

func (h HardwareSmokeHandler) record(stationID string, success bool) {
	if h.activityStore == nil {
		return
	}
	result := activity.ResultSuccess
	action := "station.hardware_smoke_completed"
	msg := "Hardware smoke test selesai."
	if !success {
		result = activity.ResultFailure
		action = "station.hardware_smoke_failed"
		msg = "Hardware smoke test gagal; report diagnostic tersimpan."
	}
	h.activityStore.RecordWithRefs(action, result, msg, &stationID, nil)
}
func (h HardwareSmokeHandler) publish(report cameras.HardwareSmokeReport) {
	if h.broker == nil {
		return
	}
	event, err := events.New("station.hardware_smoke_completed", "station", report.StationID, map[string]any{"station_id": report.StationID, "report_id": report.ReportID, "overall_status": report.OverallStatus, "next_action": report.NextAction})
	if err == nil {
		h.broker.Publish(event)
	}
}

type DefaultHardwareSmokeVerifier struct {
	Discovery     cameras.DiscoveryService
	Tether        *cameras.TetherSupervisor
	PhotoStore    *photos.Store
	SessionStore  *sessions.Store
	UploadTargets *upload.Store
	UploadJobs    *upload.JobsStore
}

func (v DefaultHardwareSmokeVerifier) Discover(ctx context.Context, station cameras.HardwareSmokeStation) (bool, cameras.SafeAction, string) {
	if station.Assignment == nil || strings.TrimSpace(station.Assignment.IdentityKey) == "" {
		return false, cameras.ActionAssignCamera, "Camera assignment required."
	}
	result, err := v.Discovery.Discover(ctx)
	if err != nil {
		return false, cameras.ActionRetryDiscovery, "Camera discovery failed."
	}
	for _, cam := range result.Cameras {
		if strings.TrimSpace(cam.IdentityKey) == strings.TrimSpace(station.Assignment.IdentityKey) && cam.Connected {
			return true, "", ""
		}
	}
	if result.Action != "" {
		return false, result.Action, "Assigned camera not connected."
	}
	return false, cameras.ActionConnectCamera, "Assigned camera not connected."
}
func (v DefaultHardwareSmokeVerifier) EnsureListener(ctx context.Context, station cameras.HardwareSmokeStation) (bool, cameras.SafeAction, string) {
	if v.Tether == nil {
		return false, cameras.ActionRetryTetherListener, "Tether supervisor unavailable."
	}
	status := v.Tether.Status(station.StationID)
	if status.Status == cameras.TetherStatusRunning || status.Status == cameras.TetherStatusStarting {
		return true, "", ""
	}
	listener, err := v.Tether.Start(ctx, cameras.TetherStationConfig{StationID: station.StationID, InputFolder: station.InputFolder, Assignment: station.Assignment})
	if err != nil {
		return false, listener.LastErrorAction, listener.Message
	}
	return listener.Status == cameras.TetherStatusRunning || listener.Status == cameras.TetherStatusStarting, listener.LastErrorAction, listener.Message
}
func (v DefaultHardwareSmokeVerifier) PrepareSession(ctx context.Context, station cameras.HardwareSmokeStation, allowActiveSession bool) (string, bool, cameras.SafeAction, string) {
	if v.SessionStore == nil {
		return "", false, cameras.ActionCheckWatcher, "Session store unavailable."
	}
	now := time.Now().UTC()
	active, ok := v.SessionStore.ActiveForStation(station.StationID, now)
	if ok && !allowActiveSession {
		return active.SessionID, false, cameras.ActionCheckWatcher, "Active customer session detected; explicitly allow active session or run smoke with a dedicated safe session."
	}
	if ok {
		return active.SessionID, true, "", ""
	}
	return "", false, cameras.ActionCheckWatcher, "No active smoke session available; start a dedicated smoke session through the existing session service before running smoke."
}

func (v DefaultHardwareSmokeVerifier) WaitForNewJPG(ctx context.Context, inputFolder string, since time.Time, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	seen := map[string]bool{}
	for {
		entries, _ := osReadDir(inputFolder)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
				continue
			}
			full := filepath.Join(inputFolder, name)
			if seen[full] {
				continue
			}
			info, err := e.Info()
			if err != nil || info.ModTime().Before(since) || info.Size() <= 0 {
				continue
			}
			if stableJPG(ctx, full) {
				return full, nil
			}
			seen[full] = true
		}
		if time.Now().After(deadline) {
			return "", errors.New("timeout")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
func (v DefaultHardwareSmokeVerifier) VerifyIngestion(ctx context.Context, stationID string, fileName string, timeout time.Duration) (string, int, error) {
	p, err := v.waitPhoto(ctx, stationID, fileName, timeout)
	if err != nil {
		return "", 0, err
	}
	return p.SessionID, 1, nil
}
func (v DefaultHardwareSmokeVerifier) VerifyLocalOriginal(ctx context.Context, sessionID string, fileName string, timeout time.Duration) (string, error) {
	p, err := v.waitSessionPhoto(ctx, sessionID, fileName, timeout, func(p photos.Photo) bool {
		return p.OriginalSaveStatus == photos.OriginalStatusSaved && p.LocalOriginalPath != ""
	})
	if err != nil {
		return "", err
	}
	return p.LocalOriginalPath, nil
}
func (v DefaultHardwareSmokeVerifier) VerifyGraded(ctx context.Context, sessionID string, fileName string, timeout time.Duration) (string, error) {
	p, err := v.waitSessionPhoto(ctx, sessionID, fileName, timeout, func(p photos.Photo) bool {
		return p.GradedProcessingStatus == photos.GradedStatusProcessed && p.LocalGradedPath != ""
	})
	if err != nil {
		return "", err
	}
	return p.LocalGradedPath, nil
}
func (v DefaultHardwareSmokeVerifier) VerifyDrive(ctx context.Context, sessionID string, mode string, timeout time.Duration) (string, bool, error) {
	if mode != cameras.SmokeModeDriveVerify {
		return "not_configured", false, nil
	}
	if v.UploadTargets == nil || v.UploadJobs == nil {
		return "not_configured", true, errors.New("drive not configured")
	}
	deadline := time.Now().Add(timeout)
	for {
		target, ok := v.UploadTargets.Get(sessionID)
		if !ok || target.PublicStatus() == upload.SessionUploadNotConfigured {
			return "not_configured", true, errors.New("drive not configured")
		}
		status := upload.AggregateUploadStatus(target, v.UploadJobs.ListBySession(sessionID))
		switch status {
		case upload.SessionUploadUploaded:
			return status, true, nil
		case upload.SessionUploadFailed, upload.SessionUploadPartialFailed:
			return status, true, errors.New("drive upload failed")
		case upload.SessionUploadPending, upload.SessionUploadUploading, upload.SessionUploadTargetPending, upload.SessionUploadPendingLocalCompletion:
			if time.Now().After(deadline) {
				return status, true, errors.New("drive upload pending")
			}
		default:
			return status, true, errors.New("drive upload not verified")
		}
		select {
		case <-ctx.Done():
			return status, true, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
func (v DefaultHardwareSmokeVerifier) waitPhoto(ctx context.Context, stationID, fileName string, timeout time.Duration) (photos.Photo, error) {
	return v.waitAny(ctx, timeout, func(p photos.Photo) bool { return p.StationID == stationID && filepath.Base(p.SourcePath) == fileName })
}
func (v DefaultHardwareSmokeVerifier) waitSessionPhoto(ctx context.Context, sessionID, fileName string, timeout time.Duration, ok func(photos.Photo) bool) (photos.Photo, error) {
	return v.waitAny(ctx, timeout, func(p photos.Photo) bool {
		return p.SessionID == sessionID && (filepath.Base(p.SourcePath) == fileName || filepath.Base(p.LocalOriginalPath) == fileName) && ok(p)
	})
}
func (v DefaultHardwareSmokeVerifier) waitAny(ctx context.Context, timeout time.Duration, pred func(photos.Photo) bool) (photos.Photo, error) {
	if v.PhotoStore == nil {
		return photos.Photo{}, errors.New("photo store unavailable")
	}
	deadline := time.Now().Add(timeout)
	for {
		for _, p := range v.PhotoStore.ListAll() {
			if pred(p) {
				return p, nil
			}
		}
		if time.Now().After(deadline) {
			return photos.Photo{}, errors.New("timeout")
		}
		select {
		case <-ctx.Done():
			return photos.Photo{}, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

var osReadDir = os.ReadDir

func stableJPG(ctx context.Context, path string) bool {
	if !isJPEGSOI(path) {
		return false
	}
	first, err := os.Stat(path)
	if err != nil || first.Size() <= 0 {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(750 * time.Millisecond):
	}
	second, err := os.Stat(path)
	if err != nil {
		return false
	}
	return first.Size() == second.Size() && first.ModTime().Equal(second.ModTime()) && isJPEGSOI(path)
}

func isJPEGSOI(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 2)
	n, _ := f.Read(b)
	return n == 2 && b[0] == 0xff && b[1] == 0xd8
}
