package cameras

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	SmokeModeLocalOnly     = "local_only"
	SmokeModeDriveOptional = "drive_optional"
	SmokeModeDriveVerify   = "drive_verify"

	SmokeStatusPassed  = "passed"
	SmokeStatusFailed  = "failed"
	SmokeStatusWarning = "warning"
	SmokeStatusAborted = "aborted"

	SmokeStepStationConfigLoaded             = "station_config_loaded"
	SmokeStepCameraAssignmentVerified        = "camera_assignment_verified"
	SmokeStepGPhoto2Discovery                = "gphoto2_discovery"
	SmokeStepCameraConnectedVerified         = "camera_connected_verified"
	SmokeStepListenerStartedOrRunning        = "listener_started_or_running"
	SmokeStepOperatorPhysicalShutterPrompted = "operator_physical_shutter_prompted"
	SmokeStepDownloadedFileDetected          = "downloaded_file_detected_in_input_folder"
	SmokeStepIngestionVerified               = "ingestion_verified"
	SmokeStepLocalOriginalVerified           = "local_original_verified"
	SmokeStepGradedProcessingVerified        = "graded_processing_verified"
	SmokeStepDriveUploadVerifiedOrSkipped    = "drive_upload_verified_or_skipped"
	SmokeStepReportSaved                     = "report_saved"
)

const (
	ActionPressShutterOrCheckCamera SafeAction = "PRESS_SHUTTER_OR_CHECK_CAMERA"
	ActionCheckWatcher              SafeAction = "CHECK_WATCHER"
	ActionConfigureDriveOptional    SafeAction = "CONFIGURE_DRIVE_OPTIONAL"
	ActionRetryProcessing           SafeAction = "RETRY_PROCESSING"
	ActionCheckLUT                  SafeAction = "CHECK_LUT"
)

var requiredSmokeSteps = []string{
	SmokeStepStationConfigLoaded,
	SmokeStepCameraAssignmentVerified,
	SmokeStepGPhoto2Discovery,
	SmokeStepCameraConnectedVerified,
	SmokeStepListenerStartedOrRunning,
	SmokeStepOperatorPhysicalShutterPrompted,
	SmokeStepDownloadedFileDetected,
	SmokeStepIngestionVerified,
	SmokeStepLocalOriginalVerified,
	SmokeStepGradedProcessingVerified,
	SmokeStepDriveUploadVerifiedOrSkipped,
	SmokeStepReportSaved,
}

type HardwareSmokeStation struct {
	StationID   string
	StationName string
	InputFolder string
	Assignment  *TetherAssignment
}

type HardwareSmokeRequest struct {
	Mode                         string        `json:"mode"`
	StopOnFailure                *bool         `json:"stop_on_failure,omitempty"`
	TimeoutMs                    int           `json:"timeout_ms,omitempty"`
	AllowActiveSession           bool          `json:"allow_active_session"`
	RestorePreviousListenerState bool          `json:"restore_previous_listener_state"`
	Timeout                      time.Duration `json:"-"`
}

type SafeSmokeCamera struct {
	CameraName string `json:"camera_name"`
	CameraID   string `json:"camera_id"`
	Assigned   bool   `json:"assigned"`
	Connected  bool   `json:"connected"`
}

type HardwareSmokeReport struct {
	ReportID      string               `json:"report_id"`
	ReportFile    string               `json:"report_file,omitempty"`
	StartedAt     time.Time            `json:"started_at"`
	CompletedAt   *time.Time           `json:"completed_at,omitempty"`
	OverallStatus string               `json:"overall_status"`
	Mode          string               `json:"mode"`
	StopOnFailure bool                 `json:"stop_on_failure"`
	StationID     string               `json:"station_id"`
	StationName   string               `json:"station_name"`
	Camera        SafeSmokeCamera      `json:"camera"`
	SessionID     string               `json:"session_id,omitempty"`
	FileName      string               `json:"file_name,omitempty"`
	Steps         []HardwareSmokeStep  `json:"steps"`
	Summary       HardwareSmokeSummary `json:"summary"`
	NextAction    string               `json:"next_action"`
}

type HardwareSmokeStep struct {
	StepKey     string     `json:"step_key"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMs  int64      `json:"duration_ms"`
	StationID   string     `json:"station_id,omitempty"`
	FileName    string     `json:"file_name,omitempty"`
	Count       int        `json:"count,omitempty"`
	Result      string     `json:"result,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
	ErrorAction string     `json:"error_action,omitempty"`
	Message     string     `json:"message"`
}

type HardwareSmokeSummary struct {
	IngestionCount      int    `json:"ingestion_count"`
	IngestionResult     string `json:"ingestion_result"`
	LocalOriginalResult string `json:"local_original_result"`
	LocalOriginalFile   string `json:"local_original_file,omitempty"`
	GradedResult        string `json:"graded_processing_result"`
	GradedFile          string `json:"graded_file,omitempty"`
	DriveStatus         string `json:"drive_status"`
	DriveRequired       bool   `json:"drive_required"`
	DurationMs          int64  `json:"duration_ms"`
	FailureCode         string `json:"failure_code,omitempty"`
	FailureAction       string `json:"failure_action,omitempty"`
}

type HardwareSmokeVerifier interface {
	Discover(ctx context.Context, station HardwareSmokeStation) (bool, SafeAction, string)
	EnsureListener(ctx context.Context, station HardwareSmokeStation) (bool, SafeAction, string)
	PrepareSession(ctx context.Context, station HardwareSmokeStation, allowActiveSession bool) (string, bool, SafeAction, string)
	WaitForNewJPG(ctx context.Context, inputFolder string, since time.Time, timeout time.Duration) (string, error)
	VerifyIngestion(ctx context.Context, stationID string, fileName string, timeout time.Duration) (string, int, error)
	VerifyLocalOriginal(ctx context.Context, sessionID string, fileName string, timeout time.Duration) (string, error)
	VerifyGraded(ctx context.Context, sessionID string, fileName string, timeout time.Duration) (string, error)
	VerifyDrive(ctx context.Context, sessionID string, mode string, timeout time.Duration) (string, bool, error)
}

type HardwareSmokeRunner struct {
	Verifier HardwareSmokeVerifier
	Writer   *HardwareSmokeReportWriter
	Now      func() time.Time
}

func (r *HardwareSmokeRunner) Run(ctx context.Context, station HardwareSmokeStation, req HardwareSmokeRequest) (HardwareSmokeReport, error) {
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	mode := normalizeSmokeMode(req.Mode)
	stop := true
	if req.StopOnFailure != nil {
		stop = *req.StopOnFailure
	}
	timeout := req.Timeout
	if timeout <= 0 && req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	started := r.Now()
	report := HardwareSmokeReport{ReportID: newSmokeReportID(started), StartedAt: started, OverallStatus: SmokeStatusFailed, Mode: mode, StopOnFailure: stop, StationID: station.StationID, StationName: safeText(station.StationName), Camera: safeCamera(station.Assignment), Steps: []HardwareSmokeStep{}, NextAction: string(ActionRetryTetherListener)}
	var firstErr error
	fail := func(step string, code string, action SafeAction, msg string) bool {
		if report.Summary.FailureCode == "" {
			report.Summary.FailureCode = code
			report.Summary.FailureAction = string(action)
			report.NextAction = string(action)
			firstErr = errors.New(code)
		}
		report.OverallStatus = SmokeStatusFailed
		r.addStep(&report, step, "failed", station.StationID, report.FileName, 0, "", code, action, msg)
		return stop
	}
	finishFailure := func() (HardwareSmokeReport, error) {
		r.markRemainingPending(&report)
		r.finishAndSave(&report)
		if firstErr == nil {
			firstErr = errors.New("HARDWARE_SMOKE_FAILED")
		}
		return report, firstErr
	}
	if station.StationID == "" || station.InputFolder == "" {
		fail(SmokeStepStationConfigLoaded, "STATION_CONFIG_REQUIRED", ActionCheckStationInputFolder, "Station configuration is incomplete.")
		return finishFailure()
	}
	r.addStep(&report, SmokeStepStationConfigLoaded, "passed", station.StationID, "", 0, "loaded", "", "", "Station configuration loaded.")
	if station.Assignment == nil || strings.TrimSpace(station.Assignment.IdentityKey) == "" {
		fail(SmokeStepCameraAssignmentVerified, "CAMERA_ASSIGNMENT_REQUIRED", ActionAssignCamera, "Camera assignment required.")
		return finishFailure()
	}
	r.addStep(&report, SmokeStepCameraAssignmentVerified, "passed", station.StationID, "", 0, "assigned", "", "", "Camera assignment verified.")
	connected, action, msg := r.Verifier.Discover(ctx, station)
	if !connected {
		if fail(SmokeStepGPhoto2Discovery, "CAMERA_DISCOVERY_FAILED", action, msg) {
			return finishFailure()
		}
	} else {
		r.addStep(&report, SmokeStepGPhoto2Discovery, "passed", station.StationID, "", 1, "ready", "", "", "gPhoto2 discovery found assigned camera.")
		report.Camera.Connected = true
		r.addStep(&report, SmokeStepCameraConnectedVerified, "passed", station.StationID, "", 1, "connected", "", "", "Assigned camera connected.")
	}
	ok, action, msg := r.Verifier.EnsureListener(ctx, station)
	if !ok {
		if fail(SmokeStepListenerStartedOrRunning, "TETHER_LISTENER_UNAVAILABLE", action, msg) {
			return finishFailure()
		}
	} else {
		r.addStep(&report, SmokeStepListenerStartedOrRunning, "passed", station.StationID, "", 0, "running", "", "", "Tether listener running or started.")
	}
	sessionID, ok, action, msg := r.Verifier.PrepareSession(ctx, station, req.AllowActiveSession)
	if !ok {
		if fail(SmokeStepIngestionVerified, "ACTIVE_SESSION_NOT_ALLOWED", action, msg) {
			return finishFailure()
		}
	} else {
		report.SessionID = safeText(sessionID)
	}
	snapshot := r.Now()
	r.addStep(&report, SmokeStepOperatorPhysicalShutterPrompted, "passed", station.StationID, "", 0, "prompted", "", "", "Operator prompted to press physical shutter.")
	path, err := r.Verifier.WaitForNewJPG(ctx, station.InputFolder, snapshot, timeout)
	if err != nil {
		if fail(SmokeStepDownloadedFileDetected, "PHYSICAL_SHUTTER_TIMEOUT", ActionPressShutterOrCheckCamera, "No new stable JPG was detected after prompt.") {
			return finishFailure()
		}
	}
	fileName := safeBase(path)
	report.FileName = fileName
	if fileName != "" {
		r.addStep(&report, SmokeStepDownloadedFileDetected, "passed", station.StationID, fileName, 1, "detected", "", "", "New stable JPG detected in station input folder.")
	}
	ingestedSessionID, count, err := r.Verifier.VerifyIngestion(ctx, station.StationID, fileName, timeout)
	if err != nil {
		if fail(SmokeStepIngestionVerified, "INGESTION_NOT_OBSERVED", ActionCheckWatcher, "Ingestion was not observed for the smoke file.") {
			return finishFailure()
		}
	} else {
		if report.SessionID != "" && ingestedSessionID != "" && ingestedSessionID != report.SessionID && !req.AllowActiveSession {
			if fail(SmokeStepIngestionVerified, "SMOKE_SESSION_MISMATCH", ActionCheckWatcher, "Smoke file was routed to an unexpected session.") {
				return finishFailure()
			}
		} else {
			report.SessionID = safeText(ingestedSessionID)
		}
	}
	if err == nil {
		report.Summary.IngestionCount = count
		report.Summary.IngestionResult = "passed"
		r.addStep(&report, SmokeStepIngestionVerified, "passed", station.StationID, fileName, count, "ingested", "", "", "Ingestion verified through backend state.")
	}
	orig, err := r.Verifier.VerifyLocalOriginal(ctx, report.SessionID, fileName, timeout)
	if err != nil {
		if fail(SmokeStepLocalOriginalVerified, "LOCAL_ORIGINAL_NOT_SAVED", ActionCheckWatcher, "Local original save was not verified.") {
			return finishFailure()
		}
	} else {
		report.Summary.LocalOriginalResult = "passed"
		report.Summary.LocalOriginalFile = safeBase(orig)
		r.addStep(&report, SmokeStepLocalOriginalVerified, "passed", station.StationID, fileName, 1, "saved", "", "", "Local original save verified.")
	}
	graded, err := r.Verifier.VerifyGraded(ctx, report.SessionID, fileName, timeout)
	if err != nil {
		if fail(SmokeStepGradedProcessingVerified, "GRADED_PROCESSING_FAILED", ActionRetryProcessing, "Graded processing was not verified.") {
			return finishFailure()
		}
	} else {
		report.Summary.GradedResult = "passed"
		report.Summary.GradedFile = safeBase(graded)
		r.addStep(&report, SmokeStepGradedProcessingVerified, "passed", station.StationID, fileName, 1, "processed", "", "", "Graded processing verified.")
	}
	drive, required, err := r.Verifier.VerifyDrive(ctx, report.SessionID, mode, timeout)
	report.Summary.DriveRequired = required
	report.Summary.DriveStatus = safeText(drive)
	if err != nil && required {
		if fail(SmokeStepDriveUploadVerifiedOrSkipped, "DRIVE_UPLOAD_NOT_VERIFIED", ActionConfigureDriveOptional, "Drive upload verification failed.") {
			return finishFailure()
		}
	}
	status := "skipped"
	if required {
		status = "passed"
	}
	if err == nil || !required {
		r.addStep(&report, SmokeStepDriveUploadVerifiedOrSkipped, status, station.StationID, fileName, 0, drive, "", "", "Drive verification completed or skipped according to mode.")
	}
	if firstErr != nil {
		return finishFailure()
	}
	report.OverallStatus = SmokeStatusPassed
	report.NextAction = "READY_FOR_EVENT"
	r.finishAndSave(&report)
	return report, nil
}

func (r *HardwareSmokeRunner) addStep(report *HardwareSmokeReport, key, status, stationID, fileName string, count int, result, code string, action SafeAction, msg string) {
	now := r.Now()
	complete := now
	report.Steps = append(report.Steps, HardwareSmokeStep{StepKey: key, Status: status, StartedAt: now, CompletedAt: &complete, DurationMs: 0, StationID: stationID, FileName: safeBase(fileName), Count: count, Result: safeText(result), ErrorCode: safeText(code), ErrorAction: string(action), Message: safeText(msg)})
}

func (r *HardwareSmokeRunner) finishAndSave(report *HardwareSmokeReport) {
	now := r.Now()
	report.CompletedAt = &now
	report.Summary.DurationMs = now.Sub(report.StartedAt).Milliseconds()
	if r.Writer != nil {
		name := r.Writer.NextName(*report)
		report.ReportFile = name
		r.addStep(report, SmokeStepReportSaved, "passed", report.StationID, report.FileName, 0, name, "", "", "Smoke report saved.")
		if saved, err := r.Writer.SaveAs(*report, name); err == nil {
			report.ReportFile = saved
		} else {
			report.Steps[len(report.Steps)-1].Status = "failed"
			report.Steps[len(report.Steps)-1].ErrorCode = "REPORT_SAVE_FAILED"
			report.Steps[len(report.Steps)-1].ErrorAction = string(ActionCheckStationInputFolder)
		}
	}
}

func (r *HardwareSmokeRunner) markRemainingPending(report *HardwareSmokeReport) {
	seen := map[string]bool{}
	for _, step := range report.Steps {
		seen[step.StepKey] = true
	}
	for _, key := range requiredSmokeSteps {
		if !seen[key] {
			r.addStep(report, key, "pending", report.StationID, report.FileName, 0, "pending", "", "", "Step was not reached before smoke completion.")
		}
	}
}

type HardwareSmokeReportWriter struct {
	Root string
	Now  func() time.Time
}

func (w HardwareSmokeReportWriter) Save(report HardwareSmokeReport) (string, error) {
	return w.SaveAs(report, w.NextName(report))
}

func (w HardwareSmokeReportWriter) NextName(report HardwareSmokeReport) string {
	return fmt.Sprintf("hardware-smoke-%s-%s.json", report.StartedAt.UTC().Format("20060102T150405Z"), safeFileToken(report.ReportID))
}

func (w HardwareSmokeReportWriter) SaveAs(report HardwareSmokeReport, preferredName string) (string, error) {
	root := w.Root
	if root == "" {
		root = filepath.Join("local-data", "reports", "hardware-smoke")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	base := strings.TrimSuffix(safeFileToken(strings.TrimSuffix(preferredName, ".json")), ".json")
	if base == "" || base == "report" {
		base = strings.TrimSuffix(w.NextName(report), ".json")
	}
	for i := 0; i < 20; i++ {
		name := base + ".json"
		if i > 0 {
			name = fmt.Sprintf("%s-%s.json", base, randomHex(3))
		}
		final := filepath.Join(root, name)
		lockPath := final + ".lock"
		lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		lock.Close()
		tmp := filepath.Join(root, ".tmp-"+name+"-"+randomHex(3))
		if err := os.WriteFile(tmp, b, 0o644); err != nil {
			_ = os.Remove(lockPath)
			return "", err
		}
		if _, err := os.Stat(final); err == nil {
			_ = os.Remove(tmp)
			_ = os.Remove(lockPath)
			continue
		}
		if err := os.Rename(tmp, final); err != nil {
			_ = os.Remove(tmp)
			_ = os.Remove(lockPath)
			return "", err
		}
		_ = os.Remove(lockPath)
		return name, nil
	}
	return "", errors.New("unique report file unavailable")
}

func normalizeSmokeMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case SmokeModeDriveOptional, SmokeModeDriveVerify, SmokeModeLocalOnly:
		return mode
	default:
		return SmokeModeLocalOnly
	}
}
func safeCamera(a *TetherAssignment) SafeSmokeCamera {
	if a == nil {
		return SafeSmokeCamera{}
	}
	return SafeSmokeCamera{CameraName: safeText(a.CameraName), CameraID: "camera_" + randomHex(4), Assigned: true}
}
func newSmokeReportID(t time.Time) string {
	return "hwsmoke_" + t.UTC().Format("20060102T150405Z") + "_" + randomHex(4)
}
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}

var unsafeReportPattern = regexp.MustCompile(`(?i)([a-z]:\\|/mnt/|usb:\d|identity|token|stdout|stderr|gphoto2|usbipd|winget|choco|apt\s|taskkill|pkill)`)

func safeText(s string) string {
	s = strings.TrimSpace(s)
	if unsafeReportPattern.MatchString(s) {
		return "[redacted]"
	}
	return s
}
func safeBase(p string) string {
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, "\\", "/")
	return safeText(filepath.Base(p))
}
func safeFileToken(s string) string {
	s = regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(s, "_")
	if s == "" {
		return "report"
	}
	return s
}
