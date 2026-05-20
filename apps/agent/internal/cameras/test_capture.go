package cameras

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"selfstudio/agent/internal/stations"
)

type TestCaptureStatus string

const (
	TestCaptureNotRun  TestCaptureStatus = "not_run"
	TestCaptureRunning TestCaptureStatus = "running"
	TestCaptureSuccess TestCaptureStatus = "success"
	TestCaptureFailed  TestCaptureStatus = "failed"
)

var (
	ErrTestCaptureAssignmentRequired = errors.New("test capture assignment required")
	ErrTestCaptureInputUnwritable    = errors.New("test capture input unwritable")
	ErrTestCaptureListenerConflict   = errors.New("test capture listener conflict")
	ErrTestCaptureInvalidRuntime     = errors.New("test capture invalid runtime")
	ErrTestCaptureInvalidPort        = errors.New("test capture invalid port")
	ErrTestCaptureTimeout            = errors.New("test capture timeout")
	ErrTestCaptureInvalidJPG         = errors.New("test capture invalid jpg")
	ErrTestCaptureBusy               = errors.New("test capture busy")
)

type TestCaptureAssignment struct {
	IdentityKey string
	CameraName  string
	Port        string
	Runtime     Runtime
}

type TestCaptureConfig struct {
	StationID   string
	InputFolder string
	Assignment  *TestCaptureAssignment
	Listener    TetherListener
	Timeout     time.Duration
	Stability   time.Duration
}

type TestCaptureResult struct {
	StationID      string            `json:"station_id"`
	Status         TestCaptureStatus `json:"status"`
	Label          string            `json:"label"`
	Action         SafeAction        `json:"action"`
	FileName       string            `json:"file_name,omitempty"`
	CapturedAt     *time.Time        `json:"captured_at,omitempty"`
	DetectedAt     *time.Time        `json:"detected_at,omitempty"`
	StableAt       *time.Time        `json:"stable_at,omitempty"`
	ValidationOnly bool              `json:"validation_only"`
}

type TestCaptureCommandSpec struct {
	Name     string
	Args     []string
	Filename string
}

type TestCaptureRunner interface {
	Capture(ctx context.Context, spec TestCaptureCommandSpec) error
}

type ExecTestCaptureRunner struct{}

func (ExecTestCaptureRunner) Capture(ctx context.Context, spec TestCaptureCommandSpec) error {
	result, err := ExecRunner{OutputLimit: 1024}.Run(ctx, CommandSpec{Name: spec.Name, Args: spec.Args, Timeout: 15 * time.Second})
	_ = result
	return err
}

type TestCaptureService struct {
	runner TestCaptureRunner
	mu     sync.Mutex
	locks  map[string]bool
	last   map[string]TestCaptureResult
}

func NewTestCaptureService(runner TestCaptureRunner) *TestCaptureService {
	if runner == nil {
		runner = ExecTestCaptureRunner{}
	}
	return &TestCaptureService{runner: runner, locks: map[string]bool{}, last: map[string]TestCaptureResult{}}
}

func (s *TestCaptureService) Last(stationID string) (TestCaptureResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.last[stationID]
	return result, ok
}

func (s *TestCaptureService) LastReadiness(stationID string) (stations.TestCaptureReadinessResult, bool) {
	last, ok := s.Last(stationID)
	if !ok {
		return stations.TestCaptureReadinessResult{}, false
	}
	return stations.TestCaptureReadinessResult{Status: string(last.Status), Label: last.Label, Action: string(last.Action)}, true
}

func (s *TestCaptureService) Run(ctx context.Context, cfg TestCaptureConfig) (TestCaptureResult, error) {
	if !s.acquire(cfg.StationID) {
		result := failure(cfg.StationID, "Test capture sedang berjalan untuk station ini.", ActionRetryTestCapture())
		s.store(result)
		return result, ErrTestCaptureBusy
	}
	defer s.release(cfg.StationID)

	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Stability <= 0 {
		cfg.Stability = 500 * time.Millisecond
	}
	if cfg.Listener.Status == TetherStatusRunning || cfg.Listener.Status == TetherStatusStarting {
		result := failure(cfg.StationID, "Tether listener masih running; hentikan dulu sebelum test capture command.", ActionStopTetherListener)
		s.store(result)
		return result, ErrTestCaptureListenerConflict
	}
	spec, err := BuildTestCaptureCommand(cfg)
	if err != nil {
		result := failure(cfg.StationID, messageForTestCaptureBuildError(err), actionForTestCaptureBuildError(err))
		s.store(result)
		return result, err
	}

	start := time.Now().UTC()
	captureCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := s.runner.Capture(captureCtx, spec); err != nil {
		result := failure(cfg.StationID, "Test capture gagal dijalankan secara aman.", ActionRetryTestCapture())
		s.cleanupCreatedTestFile(cfg.InputFolder, filepath.Base(spec.Filename))
		s.store(result)
		return result, err
	}
	expected := map[string]struct{}{filepath.Base(spec.Filename): {}}
	validationCtx := captureCtx
	validationTimeout := time.Until(start.Add(cfg.Timeout))
	if validationTimeout <= 0 {
		result := failure(cfg.StationID, "Timeout test capture habis sebelum watcher validation selesai.", ActionRetryTestCapture())
		s.cleanupCreatedTestFile(cfg.InputFolder, filepath.Base(spec.Filename))
		s.store(result)
		return result, ErrTestCaptureTimeout
	}
	validation, err := stations.RunWatchValidationForStation(validationCtx, stations.Station{StationID: cfg.StationID, InputFolder: cfg.InputFolder}, stations.WatchValidationRequest{TimeoutMs: int(validationTimeout / time.Millisecond), StabilityMs: int(cfg.Stability / time.Millisecond)}, expected)
	if err != nil {
		result := failure(cfg.StationID, "Watcher validation gagal mendeteksi file test capture.", ActionRetryTestCapture())
		s.cleanupCreatedTestFile(cfg.InputFolder, filepath.Base(spec.Filename))
		s.store(result)
		return result, err
	}
	if validation.Status != stations.ValidationStatusSuccess || validation.SourcePath == nil {
		result := failure(cfg.StationID, "Expected JPG test capture belum stabil di input folder.", ActionRetryTestCapture())
		s.cleanupCreatedTestFile(cfg.InputFolder, filepath.Base(spec.Filename))
		s.store(result)
		return result, ErrTestCaptureTimeout
	}
	if err := validateJPEGSOI(*validation.SourcePath); err != nil {
		result := failure(cfg.StationID, "File test capture bukan JPG valid.", ActionRetryTestCapture())
		s.cleanupCreatedTestFile(cfg.InputFolder, filepath.Base(spec.Filename))
		s.store(result)
		return result, err
	}
	result := TestCaptureResult{StationID: cfg.StationID, Status: TestCaptureSuccess, Label: "Test capture JPG terdeteksi oleh watcher validation", Action: ActionNone, FileName: filepath.Base(spec.Filename), CapturedAt: &start, DetectedAt: validation.DetectedAt, StableAt: validation.StableAt, ValidationOnly: true}
	s.store(result)
	return result, nil
}

func (s *TestCaptureService) acquire(stationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[stationID] {
		return false
	}
	s.locks[stationID] = true
	return true
}
func (s *TestCaptureService) release(stationID string) {
	s.mu.Lock()
	delete(s.locks, stationID)
	s.mu.Unlock()
}
func (s *TestCaptureService) store(result TestCaptureResult) {
	s.mu.Lock()
	s.last[result.StationID] = result
	s.mu.Unlock()
}
func (s *TestCaptureService) cleanupCreatedTestFile(inputFolder, name string) {
	if !strings.HasPrefix(name, "selfstudio_") || !strings.Contains(name, "_test_capture_") {
		return
	}
	_ = os.Remove(filepath.Join(inputFolder, name))
}

func BuildTestCaptureCommand(cfg TestCaptureConfig) (TestCaptureCommandSpec, error) {
	if err := validateStationID(cfg.StationID); err != nil {
		return TestCaptureCommandSpec{}, err
	}
	if cfg.Assignment == nil || strings.TrimSpace(cfg.Assignment.IdentityKey) == "" {
		return TestCaptureCommandSpec{}, ErrTestCaptureAssignmentRequired
	}
	if cfg.Assignment.Runtime != RuntimeNativeWindows && cfg.Assignment.Runtime != RuntimeWSL {
		return TestCaptureCommandSpec{}, ErrTestCaptureInvalidRuntime
	}
	if !validGPhotoPort(cfg.Assignment.Port) {
		return TestCaptureCommandSpec{}, ErrTestCaptureInvalidPort
	}
	if err := checkWritableFolder(cfg.InputFolder); err != nil {
		return TestCaptureCommandSpec{}, ErrTestCaptureInputUnwritable
	}
	fileName := fmt.Sprintf("selfstudio_%s_test_capture_%d_%s.jpg", cfg.StationID, time.Now().UTC().UnixNano(), uniqueTestCaptureSuffix())
	abs, err := filepath.Abs(filepath.Clean(cfg.InputFolder))
	if err != nil {
		return TestCaptureCommandSpec{}, err
	}
	file := filepath.Join(abs, fileName)
	created, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return TestCaptureCommandSpec{}, err
	}
	if err := created.Close(); err != nil {
		_ = os.Remove(file)
		return TestCaptureCommandSpec{}, err
	}
	rel, err := filepath.Rel(abs, file)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		_ = os.Remove(file)
		return TestCaptureCommandSpec{}, ErrTetherPathOutsideInput
	}
	filenameArg := file
	if cfg.Assignment.Runtime == RuntimeWSL {
		filenameArg, err = windowsPathToWSL(file)
		if err != nil {
			return TestCaptureCommandSpec{}, err
		}
	}
	args := []string{"--port", cfg.Assignment.Port, "--filename", filenameArg, "--capture-image-and-download", "--quiet"}
	if cfg.Assignment.Runtime == RuntimeWSL {
		return TestCaptureCommandSpec{Name: "wsl.exe", Args: append([]string{"--", "gphoto2"}, args...), Filename: file}, nil
	}
	return TestCaptureCommandSpec{Name: "gphoto2", Args: args, Filename: file}, nil
}

func uniqueTestCaptureSuffix() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}

func validateJPEGSOI(path string) error {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() <= 2 {
		return ErrTestCaptureInvalidJPG
	}
	f, err := os.Open(path)
	if err != nil {
		return ErrTestCaptureInvalidJPG
	}
	defer f.Close()
	buf := make([]byte, 2)
	if _, err := f.Read(buf); err != nil {
		return ErrTestCaptureInvalidJPG
	}
	if buf[0] != 0xFF || buf[1] != 0xD8 {
		return ErrTestCaptureInvalidJPG
	}
	return nil
}

func failure(stationID, label string, action SafeAction) TestCaptureResult {
	return TestCaptureResult{StationID: stationID, Status: TestCaptureFailed, Label: label, Action: action, ValidationOnly: true}
}
func ActionRetryTestCapture() SafeAction { return SafeAction("RETRY_TEST_CAPTURE") }
func messageForTestCaptureBuildError(err error) string {
	switch {
	case errors.Is(err, ErrTestCaptureAssignmentRequired):
		return "Camera assignment wajib sebelum test capture."
	case errors.Is(err, ErrTestCaptureInputUnwritable):
		return "Input folder station belum bisa ditulis."
	case errors.Is(err, ErrTestCaptureInvalidRuntime):
		return "Runtime gPhoto2 untuk station belum valid."
	case errors.Is(err, ErrTestCaptureInvalidPort):
		return "Port camera assignment tidak valid."
	default:
		return "Konfigurasi test capture tidak valid."
	}
}
func actionForTestCaptureBuildError(err error) SafeAction {
	switch {
	case errors.Is(err, ErrTestCaptureAssignmentRequired):
		return ActionAssignCamera
	case errors.Is(err, ErrTestCaptureInputUnwritable), errors.Is(err, ErrTetherPathOutsideInput):
		return ActionCheckStationInputFolder
	case errors.Is(err, ErrTestCaptureInvalidRuntime):
		return ActionInstallGPhoto2
	default:
		return ActionRetryTestCapture()
	}
}
