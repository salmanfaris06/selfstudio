package cameras

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type TetherStatus string

const (
	TetherStatusStopped  TetherStatus = "stopped"
	TetherStatusStarting TetherStatus = "starting"
	TetherStatusRunning  TetherStatus = "running"
	TetherStatusStopping TetherStatus = "stopping"
	TetherStatusError    TetherStatus = "error"
)

const (
	ActionAssignCamera            SafeAction = "ASSIGN_CAMERA"
	ActionCheckStationInputFolder SafeAction = "CHECK_STATION_INPUT_FOLDER"
	ActionStartTetherListener     SafeAction = "START_TETHER_LISTENER"
	ActionStopTetherListener      SafeAction = "STOP_TETHER_LISTENER"
	ActionRetryTetherListener     SafeAction = "RETRY_TETHER_LISTENER"
)

var (
	ErrTetherStationInvalid        = errors.New("invalid tether station")
	ErrTetherAssignmentRequired    = errors.New("camera assignment required")
	ErrTetherInputFolderUnwritable = errors.New("station input folder unwritable")
	ErrTetherInvalidRuntime        = errors.New("invalid tether runtime")
	ErrTetherInvalidPort           = errors.New("invalid tether port")
	ErrTetherPathOutsideInput      = errors.New("tether filename outside input folder")
)

type TetherAssignment struct {
	IdentityKey string
	CameraName  string
	Port        string
	Runtime     Runtime
}

type TetherStationConfig struct {
	StationID   string
	InputFolder string
	Assignment  *TetherAssignment
}

type TetherListener struct {
	StationID              string       `json:"station_id"`
	Status                 TetherStatus `json:"status"`
	Runtime                Runtime      `json:"runtime"`
	CameraName             string       `json:"camera_name,omitempty"`
	StartedAt              *time.Time   `json:"started_at,omitempty"`
	StoppedAt              *time.Time   `json:"stopped_at,omitempty"`
	LastCaptureAt          *time.Time   `json:"last_capture_at,omitempty"`
	LastDownloadedFileName string       `json:"last_downloaded_file_name,omitempty"`
	LastErrorCode          string       `json:"last_error_code,omitempty"`
	LastErrorAction        SafeAction   `json:"last_error_action,omitempty"`
	Message                string       `json:"message"`
	AlreadyRunning         bool         `json:"already_running,omitempty"`
}

type TetherCommandSpec struct {
	Name            string
	Args            []string
	FilenamePattern string
}

type TetherProcess interface {
	Wait() error
	Kill() error
}

type TetherProcessStarter interface {
	Start(ctx context.Context, spec TetherCommandSpec, output func(string)) (TetherProcess, error)
}

type ExecTetherStarter struct{}

func (ExecTetherStarter) Start(ctx context.Context, spec TetherCommandSpec, output func(string)) (TetherProcess, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go boundedRead(stdout, output)
	go boundedRead(stderr, output)
	return execTetherProcess{cmd: cmd}, nil
}

type execTetherProcess struct{ cmd *exec.Cmd }

func (p execTetherProcess) Wait() error { return p.cmd.Wait() }
func (p execTetherProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func boundedRead(r io.Reader, output func(string)) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1024), 8*1024)
	for s.Scan() {
		output(s.Text())
	}
}

type TetherExitHandler func(stationID string, listener TetherListener)

type TetherSupervisor struct {
	starter      TetherProcessStarter
	mu           sync.Mutex
	listeners    map[string]*tetherRuntime
	now          func() time.Time
	onUnexpected TetherExitHandler
}

type tetherRuntime struct {
	listener TetherListener
	cancel   context.CancelFunc
	process  TetherProcess
	stopped  bool
}

func NewTetherSupervisor(starter TetherProcessStarter) *TetherSupervisor {
	if starter == nil {
		starter = ExecTetherStarter{}
	}
	return &TetherSupervisor{starter: starter, listeners: map[string]*tetherRuntime{}, now: func() time.Time { return time.Now().UTC() }}
}

func (s *TetherSupervisor) SetUnexpectedExitHandler(handler TetherExitHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onUnexpected = handler
}

func (s *TetherSupervisor) Status(stationID string) TetherListener {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rt := s.listeners[stationID]; rt != nil {
		return rt.listener
	}
	return TetherListener{StationID: stationID, Status: TetherStatusStopped, LastErrorAction: ActionStartTetherListener, Message: "Tether listener stopped."}
}

func (s *TetherSupervisor) Start(ctx context.Context, cfg TetherStationConfig) (TetherListener, error) {
	if err := validateStationID(cfg.StationID); err != nil {
		return TetherListener{}, err
	}
	if cfg.Assignment == nil || strings.TrimSpace(cfg.Assignment.IdentityKey) == "" || strings.TrimSpace(cfg.Assignment.Port) == "" || strings.TrimSpace(string(cfg.Assignment.Runtime)) == "" {
		return TetherListener{StationID: cfg.StationID, Status: TetherStatusError, LastErrorCode: "CAMERA_ASSIGNMENT_REQUIRED", LastErrorAction: ActionAssignCamera, Message: "Camera assignment required before starting tether listener."}, ErrTetherAssignmentRequired
	}
	if err := checkWritableFolder(cfg.InputFolder); err != nil {
		return TetherListener{StationID: cfg.StationID, Status: TetherStatusError, LastErrorCode: "STATION_INPUT_FOLDER_UNWRITABLE", LastErrorAction: ActionCheckStationInputFolder, Message: "Station input folder is not writable."}, ErrTetherInputFolderUnwritable
	}
	spec, err := BuildTetherCommand(cfg)
	if err != nil {
		return TetherListener{StationID: cfg.StationID, Status: TetherStatusError, LastErrorCode: codeForTetherBuildError(err), LastErrorAction: actionForTetherBuildError(err), Message: messageForTetherBuildError(err)}, err
	}
	s.mu.Lock()
	if rt := s.listeners[cfg.StationID]; rt != nil && (rt.listener.Status == TetherStatusStarting || rt.listener.Status == TetherStatusRunning) {
		l := rt.listener
		l.AlreadyRunning = true
		s.mu.Unlock()
		return l, nil
	}
	startTime := s.now()
	procCtx, cancel := context.WithCancel(context.Background())
	rt := &tetherRuntime{cancel: cancel, listener: TetherListener{StationID: cfg.StationID, Status: TetherStatusStarting, Runtime: cfg.Assignment.Runtime, CameraName: cfg.Assignment.CameraName, StartedAt: &startTime, LastErrorAction: ActionStopTetherListener, Message: "Starting tether listener."}}
	s.listeners[cfg.StationID] = rt
	s.mu.Unlock()
	process, err := s.starter.Start(procCtx, spec, func(line string) { s.observeOutput(cfg.StationID, line) })
	if err != nil {
		cancel()
		s.setError(cfg.StationID, "TETHER_LISTENER_START_FAILED", ActionRetryTetherListener, "Tether listener failed to start.")
		return s.Status(cfg.StationID), err
	}
	s.mu.Lock()
	current := s.listeners[cfg.StationID]
	if current != rt || rt.stopped || rt.listener.Status == TetherStatusStopping || rt.listener.Status == TetherStatusStopped {
		s.mu.Unlock()
		cancel()
		_ = process.Kill()
		return s.Status(cfg.StationID), nil
	}
	rt.process = process
	rt.listener.Status = TetherStatusRunning
	rt.listener.Message = "Tether listener running."
	s.mu.Unlock()
	go s.monitor(cfg.StationID, process)
	_ = ctx
	return s.Status(cfg.StationID), nil
}

func (s *TetherSupervisor) Stop(stationID string) (TetherListener, error) {
	s.mu.Lock()
	rt := s.listeners[stationID]
	if rt == nil {
		s.mu.Unlock()
		return TetherListener{StationID: stationID, Status: TetherStatusStopped, LastErrorAction: ActionStartTetherListener, Message: "Tether listener already stopped."}, nil
	}
	now := s.now()
	rt.listener.Status = TetherStatusStopping
	rt.listener.StoppedAt = &now
	rt.listener.Message = "Stopping tether listener."
	rt.stopped = true
	cancel := rt.cancel
	proc := rt.process
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if proc != nil {
		go func() { time.Sleep(300 * time.Millisecond); _ = proc.Kill() }()
	}
	s.mu.Lock()
	rt.listener.Status = TetherStatusStopped
	rt.listener.Message = "Tether listener stopped."
	l := rt.listener
	delete(s.listeners, stationID)
	s.mu.Unlock()
	return l, nil
}

func (s *TetherSupervisor) StopAll() {
	s.mu.Lock()
	ids := make([]string, 0, len(s.listeners))
	for id := range s.listeners {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	for _, id := range ids {
		_, _ = s.Stop(id)
	}
}

func (s *TetherSupervisor) monitor(stationID string, p TetherProcess) {
	err := p.Wait()
	var handler TetherExitHandler
	var listener TetherListener
	unexpected := false
	s.mu.Lock()
	rt := s.listeners[stationID]
	if rt == nil {
		s.mu.Unlock()
		return
	}
	now := s.now()
	rt.listener.StoppedAt = &now
	if rt.listener.Status == TetherStatusStopping || err == nil {
		rt.listener.Status = TetherStatusStopped
		rt.listener.Message = "Tether listener stopped."
	} else {
		rt.listener.Status = TetherStatusError
		rt.listener.LastErrorCode = "TETHER_LISTENER_EXITED"
		rt.listener.LastErrorAction = ActionRetryTetherListener
		rt.listener.Message = "Tether listener exited unexpectedly."
		unexpected = true
		handler = s.onUnexpected
		listener = rt.listener
	}
	s.mu.Unlock()
	if unexpected && handler != nil {
		handler(stationID, listener)
	}
}

func (s *TetherSupervisor) observeOutput(stationID, line string) {
	safe := SanitizeTetherDiagnostic(line)
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := s.listeners[stationID]
	if rt == nil {
		return
	}
	rt.listener.Message = safe
	if name := extractSafeDownloadedName(line); name != "" {
		now := s.now()
		rt.listener.LastCaptureAt = &now
		rt.listener.LastDownloadedFileName = name
	}
}
func (s *TetherSupervisor) setError(stationID, code string, action SafeAction, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := s.listeners[stationID]
	if rt == nil {
		return
	}
	rt.listener.Status = TetherStatusError
	rt.listener.LastErrorCode = code
	rt.listener.LastErrorAction = action
	rt.listener.Message = msg
}

func BuildTetherCommand(cfg TetherStationConfig) (TetherCommandSpec, error) {
	if err := validateStationID(cfg.StationID); err != nil {
		return TetherCommandSpec{}, err
	}
	if cfg.Assignment == nil {
		return TetherCommandSpec{}, ErrTetherAssignmentRequired
	}
	if cfg.Assignment.Runtime != RuntimeNativeWindows && cfg.Assignment.Runtime != RuntimeWSL {
		return TetherCommandSpec{}, ErrTetherInvalidRuntime
	}
	if !validGPhotoPort(cfg.Assignment.Port) {
		return TetherCommandSpec{}, ErrTetherInvalidPort
	}
	pattern, err := BuildConfinedFilenamePattern(cfg.StationID, cfg.InputFolder, cfg.Assignment.Runtime)
	if err != nil {
		return TetherCommandSpec{}, err
	}
	args := []string{"--port", cfg.Assignment.Port, "--filename", pattern, "--wait-event-and-download"}
	if cfg.Assignment.Runtime == RuntimeWSL {
		return TetherCommandSpec{Name: "wsl.exe", Args: append([]string{"--", "gphoto2"}, args...), FilenamePattern: pattern}, nil
	}
	return TetherCommandSpec{Name: "gphoto2", Args: args, FilenamePattern: pattern}, nil
}

var stationIDRE = regexp.MustCompile(`^station_[1-3]$`)
var portRE = regexp.MustCompile(`^(usb:[0-9]{3},[0-9]{3}|ptpip:[A-Za-z0-9_.:-]+|disk:[A-Za-z0-9_.,:/\\-]+|serial:[A-Za-z0-9_.,:/\\-]+)$`)

func validateStationID(id string) error {
	if !stationIDRE.MatchString(id) {
		return ErrTetherStationInvalid
	}
	return nil
}
func validGPhotoPort(port string) bool {
	return portRE.MatchString(strings.TrimSpace(port)) && !strings.ContainsAny(port, ";&|`$\n\r")
}

func BuildConfinedFilenamePattern(stationID, inputFolder string, rt Runtime) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(inputFolder))
	if err != nil {
		return "", err
	}
	if abs == string(filepath.Separator) || strings.TrimSpace(inputFolder) == "" {
		return "", ErrTetherPathOutsideInput
	}
	file := stationID + "_capture_%Y%m%d_%H%M%S_%03n.%C"
	pattern := filepath.Join(abs, file)
	rel, err := filepath.Rel(abs, filepath.Clean(pattern))
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", ErrTetherPathOutsideInput
	}
	if rt == RuntimeWSL {
		return windowsPathToWSL(pattern)
	}
	return pattern, nil
}

func windowsPathToWSL(path string) (string, error) {
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, `\\`) || strings.HasPrefix(clean, `//`) {
		return "", ErrTetherPathOutsideInput
	}
	if len(clean) >= 3 && clean[1] == ':' {
		drive := strings.ToLower(string(clean[0]))
		rest := strings.ReplaceAll(clean[2:], `\\`, "/")
		rest = strings.ReplaceAll(rest, `\`, "/")
		return "/mnt/" + drive + rest, nil
	}
	if runtime.GOOS == "windows" {
		return "", ErrTetherPathOutsideInput
	}
	return filepath.ToSlash(clean), nil
}

func checkWritableFolder(folder string) error {
	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		return ErrTetherInputFolderUnwritable
	}
	f, err := os.CreateTemp(folder, ".selfstudio-tether-*.tmp")
	if err != nil {
		return ErrTetherInputFolderUnwritable
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

func codeForTetherBuildError(err error) string {
	switch {
	case errors.Is(err, ErrTetherInvalidRuntime):
		return "GPHOTO2_RUNTIME_UNAVAILABLE"
	case errors.Is(err, ErrTetherInvalidPort):
		return "INVALID_CAMERA_PORT"
	case errors.Is(err, ErrTetherPathOutsideInput):
		return "TETHER_FILENAME_UNSAFE"
	default:
		return "TETHER_LISTENER_START_FAILED"
	}
}
func actionForTetherBuildError(err error) SafeAction {
	if errors.Is(err, ErrTetherPathOutsideInput) {
		return ActionCheckStationInputFolder
	}
	if errors.Is(err, ErrTetherInvalidRuntime) {
		return ActionInstallGPhoto2
	}
	return ActionRetryTetherListener
}
func messageForTetherBuildError(err error) string {
	if errors.Is(err, ErrTetherPathOutsideInput) {
		return "Tether filename pattern is not confined to station input folder."
	}
	return "Tether listener configuration is invalid."
}

var winPathRE = regexp.MustCompile(`[A-Za-z]:\\[^\s]+`)
var wslPathRE = regexp.MustCompile(`/mnt/[A-Za-z]/[^\s]+`)

func SanitizeTetherDiagnostic(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Tether listener update."
	}
	trimmed = winPathRE.ReplaceAllString(trimmed, "[path]")
	trimmed = wslPathRE.ReplaceAllString(trimmed, "[path]")
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "gphoto2 --") || strings.Contains(lower, "wsl.exe --") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") {
		return "Tether listener diagnostic omitted safely."
	}
	if len(trimmed) > 180 {
		trimmed = trimmed[:180]
	}
	return trimmed
}
func extractSafeDownloadedName(line string) string {
	sanitized := strings.ReplaceAll(line, "\\", "/")
	fields := strings.Fields(sanitized)
	for _, f := range fields {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".jpg" || ext == ".jpeg" {
			return filepath.Base(strings.Trim(f, "\"'.,"))
		}
	}
	return ""
}
