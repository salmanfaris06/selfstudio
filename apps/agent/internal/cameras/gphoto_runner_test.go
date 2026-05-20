package cameras

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	responses map[string]CommandResult
	errors    map[string]error
	calls     []CommandSpec
}

func (f *fakeRunner) Run(ctx context.Context, spec CommandSpec) (CommandResult, error) {
	f.calls = append(f.calls, spec)
	key := spec.Name
	if spec.Name == "wsl.exe" {
		key = "wsl"
	}
	if err := f.errors[key]; err != nil {
		return CommandResult{}, err
	}
	return f.responses[key], nil
}

func TestDiscoverNativeBinaryMissingMapsInstallGphoto2(t *testing.T) {
	runner := &fakeRunner{errors: map[string]error{"gphoto2": ErrCommandUnavailable, "wsl": ErrCommandUnavailable}}
	service := NewDiscoveryService(runner)
	result, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover error = %v", err)
	}
	if result.Status != DiscoveryStatusGPhoto2Missing || result.Action != ActionInstallGPhoto2 {
		t.Fatalf("status/action = %s/%s", result.Status, result.Action)
	}
}

func TestDiscoverWSLUnavailableMapsCheckWSL(t *testing.T) {
	runner := &fakeRunner{
		errors:    map[string]error{"gphoto2": ErrCommandUnavailable},
		responses: map[string]CommandResult{"wsl": {ExitCode: 1, Stderr: "The Windows Subsystem for Linux has no installed distributions."}},
	}
	service := NewDiscoveryService(runner)
	result, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover error = %v", err)
	}
	if result.Status != DiscoveryStatusWSLMissing || result.Action != ActionCheckWSL {
		t.Fatalf("status/action = %s/%s", result.Status, result.Action)
	}
}

func TestDiscoverNoCamerasReturnsConnectCamera(t *testing.T) {
	runner := &fakeRunner{responses: map[string]CommandResult{"gphoto2": {ExitCode: 0, Stdout: "Model Port\n----------------------------------------------------------\n"}}}
	service := NewDiscoveryService(runner)
	result, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover error = %v", err)
	}
	if result.Status != DiscoveryStatusNoCameras || result.Action != ActionConnectCamera {
		t.Fatalf("status/action = %s/%s", result.Status, result.Action)
	}
}

func TestDiscoverWSLNoCamerasRunsUSBIPDReadOnlyDiagnostic(t *testing.T) {
	runner := &fakeRunner{
		errors: map[string]error{"gphoto2": ErrCommandUnavailable},
		responses: map[string]CommandResult{
			"wsl":    {ExitCode: 0, Stdout: "Model Port\n----------------------------------------------------------\n"},
			"usbipd": {ExitCode: 0, Stdout: "Connected:\n1-4 Sony Camera Not attached\n"},
		},
	}
	service := NewDiscoveryService(runner)
	result, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover error = %v", err)
	}
	if result.Status != DiscoveryStatusUSBIPDCheckNeeded || result.Action != ActionCheckUSBIPD {
		t.Fatalf("status/action = %s/%s diagnostics=%v", result.Status, result.Action, result.Diagnostics)
	}
	if len(runner.calls) != 3 || runner.calls[2].Name != "usbipd" || runner.calls[2].Args[0] != "list" {
		t.Fatalf("calls = %+v", runner.calls)
	}
}

func TestDiscoverUSBIPDUnavailableReturnsSafeCheckUSBIPD(t *testing.T) {
	runner := &fakeRunner{
		errors:    map[string]error{"gphoto2": ErrCommandUnavailable, "usbipd": ErrCommandUnavailable},
		responses: map[string]CommandResult{"wsl": {ExitCode: 0, Stdout: "Model Port\n----------------------------------------------------------\n"}},
	}
	service := NewDiscoveryService(runner)
	result, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover error = %v", err)
	}
	if result.Status != DiscoveryStatusUSBIPDCheckNeeded || result.Action != ActionCheckUSBIPD {
		t.Fatalf("status/action = %s/%s", result.Status, result.Action)
	}
}

func TestDiscoverTimeoutMapsRetry(t *testing.T) {
	runner := &fakeRunner{errors: map[string]error{"gphoto2": ErrCommandTimeout}}
	service := NewDiscoveryService(runner)
	result, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover error = %v", err)
	}
	if result.Status != DiscoveryStatusError || result.Action != ActionRetryDiscovery {
		t.Fatalf("status/action = %s/%s", result.Status, result.Action)
	}
}

func TestRunnerUsesAllowlistedCommands(t *testing.T) {
	runner := &fakeRunner{errors: map[string]error{"gphoto2": errors.New("stop")}}
	service := NewDiscoveryService(runner)
	_, _ = service.Discover(context.Background())
	if len(runner.calls) != 1 || runner.calls[0].Name != "gphoto2" || len(runner.calls[0].Args) != 1 || runner.calls[0].Args[0] != "--auto-detect" {
		t.Fatalf("unexpected calls: %+v", runner.calls)
	}
}
