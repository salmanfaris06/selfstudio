package stations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReadinessStatus string

const (
	ReadinessReady   ReadinessStatus = "ready"
	ReadinessWarning ReadinessStatus = "warning"
	ReadinessFailed  ReadinessStatus = "failed"
	ReadinessUnknown ReadinessStatus = "unknown"
)

type ReadinessCheck struct {
	CheckKey string          `json:"check_key"`
	Status   ReadinessStatus `json:"status"`
	Label    string          `json:"label"`
	Action   string          `json:"action"`
}

type Readiness struct {
	StationID string           `json:"station_id"`
	Status    ReadinessStatus  `json:"status"`
	Label     string           `json:"label"`
	Action    string           `json:"action"`
	CheckedAt time.Time        `json:"checked_at"`
	Checks    []ReadinessCheck `json:"checks"`
}

type CameraDiscovery interface {
	Discover(context.Context) (CameraDiscoveryResult, error)
}

type CameraDiscoveryResult struct {
	Status  string
	Action  string
	Runtime string
	Cameras []CameraDetected
}

type CameraDetected struct {
	IdentityKey string
	Port        string
	Runtime     string
	Connected   bool
}

type TetherStatusProvider interface {
	Status(stationID string) TetherStatusResult
}

type TetherStatusResult struct {
	Status          string
	LastErrorAction string
}

type TestCaptureResultProvider interface {
	LastReadiness(stationID string) (TestCaptureReadinessResult, bool)
}

type TestCaptureReadinessResult struct {
	Status string
	Label  string
	Action string
}

type CameraReadinessOptions struct {
	Required           bool
	Discovery          CameraDiscovery
	Tether             TetherStatusProvider
	TestCaptureResults TestCaptureResultProvider
}

type ReadinessValidator struct {
	outputRoot string
	camera     CameraReadinessOptions
}

func NewReadinessValidator(outputRoot string) ReadinessValidator {
	return NewReadinessValidatorWithCamera(outputRoot, CameraReadinessOptions{})
}

func NewReadinessValidatorWithCamera(outputRoot string, camera CameraReadinessOptions) ReadinessValidator {
	if outputRoot == "" {
		outputRoot = filepath.Join("local-data", "output")
	}
	return ReadinessValidator{outputRoot: outputRoot, camera: camera}
}

func (v ReadinessValidator) Check(station Station) Readiness {
	cameraChecks := v.cameraChecks(station)
	checks := []ReadinessCheck{
		checkReadableDir("input_folder", station.InputFolder, "Input folder bisa dibaca", "Perbaiki input folder lalu jalankan recheck."),
		checkWritableOutputRule(v.outputRoot, station.OutputRule),
		checkLUT(station.DefaultLUTPath),
		v.legacyDeviceCheck(cameraChecks),
	}
	checks = append(checks, cameraChecks...)

	status := ReadinessReady
	label := "Station siap"
	action := "Tidak ada aksi diperlukan."
	for _, check := range checks {
		if check.Status == ReadinessFailed {
			status = ReadinessFailed
			label = "Station belum siap"
			action = check.Action
			if action == "" {
				action = "Perbaiki check yang gagal lalu jalankan recheck."
			}
			break
		}
		if (check.Status == ReadinessUnknown || check.Status == ReadinessWarning) && status == ReadinessReady {
			status = ReadinessWarning
			label = "Station butuh verifikasi operator"
			action = check.Action
			if action == "" {
				action = "Pastikan device/tether aktif sebelum event."
			}
		}
	}

	return Readiness{StationID: station.StationID, Status: status, Label: label, Action: action, CheckedAt: time.Now().UTC(), Checks: checks}
}

func (v ReadinessValidator) legacyDeviceCheck(cameraChecks []ReadinessCheck) ReadinessCheck {
	if cameraCheckStatus(cameraChecks, "camera_assignment") == ReadinessReady && cameraCheckStatus(cameraChecks, "camera_connected") == ReadinessReady {
		return ReadinessCheck{CheckKey: "device", Status: ReadinessReady, Label: "Legacy device check digantikan oleh assigned gPhoto2 camera", Action: "Tidak ada aksi diperlukan."}
	}
	return ReadinessCheck{
		CheckKey: "device",
		Status:   ReadinessUnknown,
		Label:    "Device identifier belum diverifikasi otomatis",
		Action:   "Pastikan kamera/tether software menulis JPG ke input folder.",
	}
}

func cameraCheckStatus(checks []ReadinessCheck, key string) ReadinessStatus {
	for _, check := range checks {
		if check.CheckKey == key {
			return check.Status
		}
	}
	return ReadinessUnknown
}

func (v ReadinessValidator) cameraChecks(station Station) []ReadinessCheck {
	severity := func(status ReadinessStatus) ReadinessStatus {
		if status == ReadinessFailed && !v.camera.Required {
			return ReadinessWarning
		}
		return status
	}
	checks := []ReadinessCheck{checkWritableExistingDir("input_folder_writable", station.InputFolder, "Input folder bisa ditulis untuk test capture/listener", "CHECK_STATION_INPUT_FOLDER")}
	if station.CameraAssignment == nil || strings.TrimSpace(station.CameraAssignment.IdentityKey) == "" {
		checks = append(checks,
			ReadinessCheck{CheckKey: "camera_assignment", Status: severity(ReadinessFailed), Label: "Camera belum di-assign ke station", Action: "ASSIGN_CAMERA"},
			ReadinessCheck{CheckKey: "gphoto2_availability", Status: severity(ReadinessFailed), Label: "gPhoto2 belum bisa divalidasi tanpa assignment", Action: "ASSIGN_CAMERA"},
			ReadinessCheck{CheckKey: "camera_connected", Status: severity(ReadinessFailed), Label: "Camera connection belum bisa divalidasi tanpa assignment", Action: "ASSIGN_CAMERA"},
		)
	} else {
		checks = append(checks, ReadinessCheck{CheckKey: "camera_assignment", Status: ReadinessReady, Label: "Camera assignment tersedia", Action: "Tidak ada aksi diperlukan."})
		if v.camera.Discovery == nil {
			checks = append(checks,
				ReadinessCheck{CheckKey: "gphoto2_availability", Status: ReadinessUnknown, Label: "gPhoto2 discovery belum terhubung", Action: "RECHECK_CAMERA_READINESS"},
				ReadinessCheck{CheckKey: "camera_connected", Status: severity(ReadinessFailed), Label: "Camera connection belum bisa diverifikasi", Action: "RETRY_CAMERA_DISCOVERY"},
			)
		} else {
			result, err := v.camera.Discovery.Discover(context.Background())
			if err != nil || result.Status == "error" {
				checks = append(checks,
					ReadinessCheck{CheckKey: "gphoto2_availability", Status: severity(ReadinessFailed), Label: "gPhoto2 discovery gagal aman", Action: "RETRY_CAMERA_DISCOVERY"},
					ReadinessCheck{CheckKey: "camera_connected", Status: severity(ReadinessFailed), Label: "Camera connection belum terverifikasi", Action: "RETRY_CAMERA_DISCOVERY"},
				)
			} else if result.Status == "gphoto2_missing" || result.Status == "wsl_missing" {
				action := safeCameraAction(result.Action, "INSTALL_GPHOTO2")
				checks = append(checks,
					ReadinessCheck{CheckKey: "gphoto2_availability", Status: severity(ReadinessFailed), Label: "gPhoto2 runtime belum tersedia", Action: action},
					ReadinessCheck{CheckKey: "camera_connected", Status: severity(ReadinessFailed), Label: "Camera belum bisa dicek sebelum gPhoto2 tersedia", Action: action},
				)
			} else {
				checks = append(checks, ReadinessCheck{CheckKey: "gphoto2_availability", Status: ReadinessReady, Label: "gPhoto2 discovery tersedia", Action: "Tidak ada aksi diperlukan."})
				connected := false
				for _, camera := range result.Cameras {
					if strings.EqualFold(strings.TrimSpace(camera.IdentityKey), strings.TrimSpace(station.CameraAssignment.IdentityKey)) && camera.Connected {
						connected = true
						break
					}
				}
				if connected {
					checks = append(checks, ReadinessCheck{CheckKey: "camera_connected", Status: ReadinessReady, Label: "Assigned camera terdeteksi oleh gPhoto2", Action: "Tidak ada aksi diperlukan."})
				} else {
					action := safeCameraAction(result.Action, "CONNECT_CAMERA")
					checks = append(checks, ReadinessCheck{CheckKey: "camera_connected", Status: severity(ReadinessFailed), Label: "Assigned camera belum terdeteksi oleh gPhoto2", Action: action})
				}
			}
		}
	}
	checks = append(checks, v.tetherCheck(station.StationID, severity), v.testCaptureCheck(station.StationID))
	return checks
}

func (v ReadinessValidator) tetherCheck(stationID string, severity func(ReadinessStatus) ReadinessStatus) ReadinessCheck {
	if v.camera.Tether == nil {
		return ReadinessCheck{CheckKey: "tether_listener", Status: severity(ReadinessFailed), Label: "Tether listener status belum tersedia", Action: "START_TETHER_LISTENER"}
	}
	status := v.camera.Tether.Status(stationID)
	if status.Status == "running" {
		return ReadinessCheck{CheckKey: "tether_listener", Status: ReadinessReady, Label: "Tether listener running", Action: "Tidak ada aksi diperlukan."}
	}
	action := safeCameraAction(status.LastErrorAction, "START_TETHER_LISTENER")
	return ReadinessCheck{CheckKey: "tether_listener", Status: severity(ReadinessFailed), Label: "Tether listener belum running", Action: action}
}

func (v ReadinessValidator) testCaptureCheck(stationID string) ReadinessCheck {
	if v.camera.TestCaptureResults == nil {
		return ReadinessCheck{CheckKey: "camera_test_capture", Status: ReadinessUnknown, Label: "Test capture belum pernah dijalankan", Action: "RETRY_TEST_CAPTURE"}
	}
	last, ok := v.camera.TestCaptureResults.LastReadiness(stationID)
	if !ok || last.Status == "" || last.Status == "not_run" {
		return ReadinessCheck{CheckKey: "camera_test_capture", Status: ReadinessUnknown, Label: "Test capture belum pernah dijalankan", Action: "RETRY_TEST_CAPTURE"}
	}
	status := ReadinessWarning
	if last.Status == "success" {
		status = ReadinessReady
	} else if last.Status == "failed" {
		status = ReadinessWarning
	}
	action := safeCameraAction(last.Action, "RETRY_TEST_CAPTURE")
	if last.Status == "success" && action == "RETRY_TEST_CAPTURE" {
		action = "Tidak ada aksi diperlukan."
	}
	return ReadinessCheck{CheckKey: "camera_test_capture", Status: status, Label: nonEmpty(last.Label, "Hasil test capture terakhir tersedia"), Action: action}
}

func safeCameraAction(action string, fallback string) string {
	a := strings.TrimSpace(action)
	allowed := map[string]bool{"INSTALL_GPHOTO2": true, "CHECK_WSL": true, "CHECK_USBIPD": true, "CONNECT_CAMERA": true, "CHECK_CAMERA_USB_MODE": true, "ASSIGN_CAMERA": true, "START_TETHER_LISTENER": true, "STOP_TETHER_LISTENER": true, "CHECK_STATION_INPUT_FOLDER": true, "RETRY_CAMERA_DISCOVERY": true, "RETRY_TEST_CAPTURE": true, "RECHECK_CAMERA_READINESS": true, "Tidak ada aksi diperlukan.": true}
	if allowed[a] {
		return a
	}
	return fallback
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func checkReadableDir(key string, dir string, okLabel string, failAction string) ReadinessCheck {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ReadinessCheck{CheckKey: key, Status: ReadinessFailed, Label: "Folder tidak ditemukan atau bukan folder", Action: failAction}
	}
	if _, err := os.ReadDir(dir); err != nil {
		return ReadinessCheck{CheckKey: key, Status: ReadinessFailed, Label: "Folder tidak bisa dibaca", Action: failAction}
	}
	return ReadinessCheck{CheckKey: key, Status: ReadinessReady, Label: okLabel, Action: "Tidak ada aksi diperlukan."}
}

func checkWritableOutputRule(outputRoot string, outputRule string) ReadinessCheck {
	probeDir, err := deriveOutputProbeDir(outputRoot, outputRule)
	if err != nil {
		return ReadinessCheck{CheckKey: "output_folder", Status: ReadinessFailed, Label: "Output rule tidak bisa dipetakan dengan aman", Action: "Perbaiki output rule lalu jalankan recheck."}
	}
	return checkWritableExistingDir("output_folder", probeDir, "Output folder sesuai rule bisa ditulis", "Perbaiki output folder atau output rule lalu jalankan recheck.")
}

func deriveOutputProbeDir(outputRoot string, outputRule string) (string, error) {
	rootClean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(outputRoot, "\\", "/")))
	rootAbs, err := filepath.Abs(rootClean)
	if err != nil {
		return "", err
	}
	sample := strings.NewReplacer(
		"{station_id}", "station_1",
		"{customer_name}", "sample_customer",
		"{order_number}", "sample_order",
		"{session_id}", "sample_session",
	).Replace(outputRule)
	sample = filepath.Clean(filepath.FromSlash(strings.ReplaceAll(sample, "\\", "/")))
	if filepath.IsAbs(sample) || sample == ".." || strings.HasPrefix(sample, ".."+string(filepath.Separator)) {
		return "", ErrInvalidStationConfig
	}
	candidate := filepath.Join(rootAbs, sample)
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", err
	}
	if candidateAbs != rootAbs && !strings.HasPrefix(candidateAbs, rootAbs+string(filepath.Separator)) {
		return "", ErrInvalidStationConfig
	}
	return candidateAbs, nil
}

func checkWritableExistingDir(key string, dir string, okLabel string, failAction string) ReadinessCheck {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ReadinessCheck{CheckKey: key, Status: ReadinessFailed, Label: "Output folder sesuai rule belum ada", Action: failAction}
	}
	probe, err := os.CreateTemp(dir, ".selfstudio-write-check-*")
	if err != nil {
		return ReadinessCheck{CheckKey: key, Status: ReadinessFailed, Label: "Output folder tidak bisa ditulis", Action: failAction}
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return ReadinessCheck{CheckKey: key, Status: ReadinessFailed, Label: "Output folder gagal flush write probe", Action: failAction}
	}
	if err := os.Remove(name); err != nil {
		return ReadinessCheck{CheckKey: key, Status: ReadinessFailed, Label: "Output folder menyisakan file probe", Action: failAction}
	}
	return ReadinessCheck{CheckKey: key, Status: ReadinessReady, Label: okLabel, Action: "Tidak ada aksi diperlukan."}
}

func checkLUT(path string) ReadinessCheck {
	if !strings.EqualFold(filepath.Ext(path), ".cube") {
		return ReadinessCheck{CheckKey: "default_lut", Status: ReadinessFailed, Label: "Format LUT harus .cube", Action: "Pilih file LUT .cube yang valid."}
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ReadinessCheck{CheckKey: "default_lut", Status: ReadinessFailed, Label: "File LUT tidak ditemukan", Action: "Perbaiki path LUT lalu jalankan recheck."}
	}
	file, err := os.Open(path)
	if err != nil {
		return ReadinessCheck{CheckKey: "default_lut", Status: ReadinessFailed, Label: "File LUT tidak bisa dibaca", Action: "Perbaiki permission/path LUT lalu jalankan recheck."}
	}
	defer file.Close()
	buf := make([]byte, 1)
	_, _ = file.Read(buf)
	return ReadinessCheck{CheckKey: "default_lut", Status: ReadinessReady, Label: "File LUT ditemukan dan bisa dibaca", Action: "Tidak ada aksi diperlukan."}
}
