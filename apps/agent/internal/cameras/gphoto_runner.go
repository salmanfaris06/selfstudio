package cameras

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrCommandUnavailable = errors.New("command unavailable")
	ErrCommandTimeout     = errors.New("command timeout")
)

type CommandSpec struct {
	Name    string
	Args    []string
	Timeout time.Duration
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type CommandRunner interface {
	Run(ctx context.Context, spec CommandSpec) (CommandResult, error)
}

type ExecRunner struct {
	OutputLimit int
}

func (r ExecRunner) Run(ctx context.Context, spec CommandSpec) (CommandResult, error) {
	if !isAllowlisted(spec) {
		return CommandResult{}, ErrCommandUnavailable
	}
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, spec.Name, spec.Args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{Stdout: limitOutput(stdout.String(), r.OutputLimit), Stderr: limitOutput(stderr.String(), r.OutputLimit)}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		return result, ErrCommandTimeout
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return result, ErrCommandUnavailable
		}
		return result, err
	}
	return result, nil
}

func isAllowlisted(spec CommandSpec) bool {
	if spec.Name == "gphoto2" && len(spec.Args) == 1 && spec.Args[0] == "--auto-detect" {
		return true
	}
	if spec.Name == "gphoto2" && isSafeCaptureArgs(spec.Args) {
		return true
	}
	if spec.Name == "wsl.exe" && len(spec.Args) == 4 && spec.Args[0] == "--" && spec.Args[1] == "gphoto2" && spec.Args[2] == "--auto-detect" {
		return false
	}
	if spec.Name == "wsl.exe" && len(spec.Args) == 3 && spec.Args[0] == "--" && spec.Args[1] == "gphoto2" && spec.Args[2] == "--auto-detect" {
		return true
	}
	if spec.Name == "wsl.exe" && len(spec.Args) >= 3 && spec.Args[0] == "--" && spec.Args[1] == "gphoto2" && isSafeCaptureArgs(spec.Args[2:]) {
		return true
	}
	if spec.Name == "usbipd" && len(spec.Args) == 1 && spec.Args[0] == "list" {
		return true
	}
	return false
}

func isSafeCaptureArgs(args []string) bool {
	if len(args) != 6 {
		return false
	}
	if args[0] != "--port" || args[2] != "--filename" || args[4] != "--capture-image-and-download" || args[5] != "--quiet" {
		return false
	}
	filename := strings.TrimSpace(args[3])
	return validGPhotoPort(args[1]) && filename != "" && !strings.ContainsAny(filename, ";&|`$\n\r")
}

func limitOutput(value string, limit int) string {
	if limit <= 0 {
		limit = 16 * 1024
	}
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

type DiscoveryService struct {
	runner CommandRunner
}

func NewDiscoveryService(runner CommandRunner) DiscoveryService {
	if runner == nil {
		runner = ExecRunner{OutputLimit: 16 * 1024}
	}
	return DiscoveryService{runner: runner}
}

func (s DiscoveryService) Discover(ctx context.Context) (DiscoveryResult, error) {
	native, err := s.runner.Run(ctx, CommandSpec{Name: "gphoto2", Args: []string{"--auto-detect"}, Timeout: 5 * time.Second})
	if err == nil {
		return resultFromCommand(native, RuntimeNativeWindows), nil
	}
	if errors.Is(err, ErrCommandTimeout) {
		return safeResult(DiscoveryStatusError, ActionRetryDiscovery, RuntimeNativeWindows, "gPhoto2 discovery timeout."), nil
	}
	if !errors.Is(err, ErrCommandUnavailable) {
		return safeResult(DiscoveryStatusError, ActionRetryDiscovery, RuntimeNativeWindows, "gPhoto2 discovery failed safely."), nil
	}

	wsl, wslErr := s.runner.Run(ctx, CommandSpec{Name: "wsl.exe", Args: []string{"--", "gphoto2", "--auto-detect"}, Timeout: 6 * time.Second})
	if wslErr == nil {
		if wsl.ExitCode != 0 && strings.Contains(strings.ToLower(wsl.Stderr), "no installed distributions") {
			return safeResult(DiscoveryStatusWSLMissing, ActionCheckWSL, RuntimeWSL, "WSL distro is not available."), nil
		}
		result := resultFromCommand(wsl, RuntimeWSL)
		if result.Status == DiscoveryStatusNoCameras {
			return s.withUSBIPDDiagnostic(ctx, result), nil
		}
		return result, nil
	}
	if errors.Is(wslErr, ErrCommandUnavailable) {
		return safeResult(DiscoveryStatusGPhoto2Missing, ActionInstallGPhoto2, RuntimeUnknown, "gPhoto2 tidak ditemukan di native atau WSL runtime."), nil
	}
	if errors.Is(wslErr, ErrCommandTimeout) {
		return safeResult(DiscoveryStatusError, ActionRetryDiscovery, RuntimeWSL, "WSL gPhoto2 discovery timeout."), nil
	}
	return safeResult(DiscoveryStatusWSLMissing, ActionCheckWSL, RuntimeWSL, "WSL discovery tidak tersedia."), nil
}

func (s DiscoveryService) withUSBIPDDiagnostic(ctx context.Context, result DiscoveryResult) DiscoveryResult {
	usbipd, err := s.runner.Run(ctx, CommandSpec{Name: "usbipd", Args: []string{"list"}, Timeout: 3 * time.Second})
	if err != nil {
		result.Status = DiscoveryStatusUSBIPDCheckNeeded
		result.Action = ActionCheckUSBIPD
		result.Diagnostics = appendSafeDiagnostic(result.Diagnostics, "usbipd diagnostic tidak tersedia; periksa instalasi usbipd dan attachment kamera ke WSL secara manual.")
		return result
	}
	result.Diagnostics = appendSafeDiagnostic(result.Diagnostics, summarizeUSBIPDList(usbipd.Stdout+"\n"+usbipd.Stderr))
	if usbipd.ExitCode != 0 || usbipdMentionsCamera(usbipd.Stdout+"\n"+usbipd.Stderr) {
		result.Status = DiscoveryStatusUSBIPDCheckNeeded
		result.Action = ActionCheckUSBIPD
	}
	return result
}

func summarizeUSBIPDList(output string) string {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "not attached") || strings.Contains(lower, "shared") || strings.Contains(lower, "camera") || strings.Contains(lower, "mtp") || strings.Contains(lower, "ptp") {
		return "usbipd melihat perangkat USB yang mungkin perlu di-attach ke WSL secara manual dengan approval."
	}
	if strings.TrimSpace(output) == "" {
		return "usbipd list tidak mengembalikan perangkat USB."
	}
	return "usbipd diagnostic selesai tanpa menjalankan bind atau attach."
}

func usbipdMentionsCamera(output string) bool {
	lower := strings.ToLower(output)
	keywords := []string{"camera", "ptp", "mtp", "imaging", "sony", "canon", "nikon", "fujifilm", "fuji"}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func appendSafeDiagnostic(existing []string, diagnostic string) []string {
	if len(existing) >= 5 {
		return existing
	}
	return append(existing, sanitizeDiagnostic(diagnostic))
}

func resultFromCommand(command CommandResult, runtimeValue Runtime) DiscoveryResult {
	combined := command.Stdout + "\n" + command.Stderr
	result := ParseAutoDetect(combined, runtimeValue)
	if command.ExitCode != 0 && len(result.Cameras) == 0 {
		lower := strings.ToLower(command.Stderr)
		if strings.Contains(lower, "not found") || strings.Contains(lower, "not installed") {
			return safeResult(DiscoveryStatusGPhoto2Missing, ActionInstallGPhoto2, runtimeValue, "gPhoto2 belum tersedia untuk camera discovery.")
		}
		if strings.Contains(lower, "permission") || strings.Contains(lower, "busy") {
			return safeResult(DiscoveryStatusError, ActionRetryDiscovery, runtimeValue, "Kamera busy atau akses sementara gagal.")
		}
	}
	return result
}

func safeResult(status DiscoveryStatus, action SafeAction, runtimeValue Runtime, diagnostic string) DiscoveryResult {
	return DiscoveryResult{Status: status, Action: action, Runtime: runtimeValue, Cameras: []DetectedCamera{}, Diagnostics: []string{sanitizeDiagnostic(diagnostic)}, ScannedAt: time.Now().UTC()}
}
