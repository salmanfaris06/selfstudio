package cameras

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeCaptureRunner struct{ write bool }

func (f fakeCaptureRunner) Capture(ctx context.Context, spec TestCaptureCommandSpec) error {
	if f.write {
		return os.WriteFile(spec.Filename, []byte{0xFF, 0xD8, 0xFF, 0xE0, 1}, 0o644)
	}
	return nil
}

type deadlineCaptureRunner struct{}

func (deadlineCaptureRunner) Capture(ctx context.Context, spec TestCaptureCommandSpec) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestBuildTestCaptureCommandSafeNative(t *testing.T) {
	dir := t.TempDir()
	spec, err := BuildTestCaptureCommand(TestCaptureConfig{StationID: "station_1", InputFolder: dir, Assignment: &TestCaptureAssignment{IdentityKey: "id", Port: "usb:001,002", Runtime: RuntimeNativeWindows}})
	if err != nil {
		t.Fatalf("BuildTestCaptureCommand() error = %v", err)
	}
	if spec.Name != "gphoto2" {
		t.Fatalf("name = %q", spec.Name)
	}
	joined := strings.Join(spec.Args, " ")
	if strings.ContainsAny(joined, ";&|`$\n\r") {
		t.Fatalf("unsafe args: %q", joined)
	}
	if filepath.Dir(spec.Filename) != filepath.Clean(dir) {
		t.Fatalf("filename outside dir: %s", spec.Filename)
	}
	if strings.Contains(joined, "--force-overwrite") {
		t.Fatalf("test capture must not force-overwrite existing files: %q", joined)
	}
	if !isAllowlisted(CommandSpec{Name: spec.Name, Args: spec.Args}) {
		t.Fatalf("BuildTestCaptureCommand output must be accepted by production allowlist: %#v", spec)
	}
}

func TestBuildTestCaptureCommandSafeWSLAcceptedByProductionAllowlist(t *testing.T) {
	dir := t.TempDir()
	spec, err := BuildTestCaptureCommand(TestCaptureConfig{StationID: "station_1", InputFolder: dir, Assignment: &TestCaptureAssignment{IdentityKey: "id", Port: "usb:001,002", Runtime: RuntimeWSL}})
	if err != nil {
		t.Fatalf("BuildTestCaptureCommand() error = %v", err)
	}
	joined := strings.Join(spec.Args, " ")
	if strings.ContainsAny(joined, ";&|`$\n\r") {
		t.Fatalf("unsafe args: %q", joined)
	}
	if strings.Contains(joined, "--force-overwrite") {
		t.Fatalf("test capture must not force-overwrite existing files: %q", joined)
	}
	if !isAllowlisted(CommandSpec{Name: spec.Name, Args: spec.Args}) {
		t.Fatalf("BuildTestCaptureCommand WSL output must be accepted by production allowlist: %#v", spec)
	}
}

func TestProductionAllowlistRejectsUnsafeTestCaptureArgs(t *testing.T) {
	safe := []string{"--port", "usb:001,002", "--filename", filepath.Join(t.TempDir(), "capture.jpg"), "--capture-image-and-download", "--quiet"}
	if !isSafeCaptureArgs(safe) {
		t.Fatalf("expected safe test capture args without force-overwrite to be accepted")
	}
	withForceOverwrite := []string{"--port", "usb:001,002", "--filename", filepath.Join(t.TempDir(), "capture.jpg"), "--capture-image-and-download", "--force-overwrite", "--quiet"}
	if isSafeCaptureArgs(withForceOverwrite) {
		t.Fatalf("must reject force-overwrite test capture args")
	}
	injected := []string{"--port", "usb:001,002", "--filename", filepath.Join(t.TempDir(), "capture.jpg;rm -rf"), "--capture-image-and-download", "--quiet"}
	if isSafeCaptureArgs(injected) {
		t.Fatalf("must reject shell metacharacters in filename args")
	}
}

func TestBuildTestCaptureCommandFilenamesAreCollisionSafe(t *testing.T) {
	dir := t.TempDir()
	cfg := TestCaptureConfig{StationID: "station_1", InputFolder: dir, Assignment: &TestCaptureAssignment{IdentityKey: "id", Port: "usb:001,002", Runtime: RuntimeNativeWindows}}
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		spec, err := BuildTestCaptureCommand(cfg)
		if err != nil {
			t.Fatalf("BuildTestCaptureCommand() error = %v", err)
		}
		base := filepath.Base(spec.Filename)
		if _, ok := seen[base]; ok {
			t.Fatalf("filename collision: %s", base)
		}
		seen[base] = struct{}{}
		if info, err := os.Stat(spec.Filename); err != nil || info.IsDir() {
			t.Fatalf("expected reserved output file %s: info=%v err=%v", spec.Filename, info, err)
		}
		if strings.Contains(strings.Join(spec.Args, " "), "--force-overwrite") {
			t.Fatalf("test capture must not include --force-overwrite")
		}
	}
}

func TestTestCaptureRejectsRunningListener(t *testing.T) {
	dir := t.TempDir()
	service := NewTestCaptureService(fakeCaptureRunner{write: true})
	_, err := service.Run(context.Background(), TestCaptureConfig{StationID: "station_1", InputFolder: dir, Assignment: &TestCaptureAssignment{IdentityKey: "id", Port: "usb:001,002", Runtime: RuntimeNativeWindows}, Listener: TetherListener{Status: TetherStatusRunning}})
	if err != ErrTestCaptureListenerConflict {
		t.Fatalf("err = %v", err)
	}
}

func TestTestCaptureExpectedFileOnlyAndJPEGSOI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "old.jpg"), []byte{0xFF, 0xD8, 1}, 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewTestCaptureService(fakeCaptureRunner{write: true})
	result, err := service.Run(context.Background(), TestCaptureConfig{StationID: "station_1", InputFolder: dir, Assignment: &TestCaptureAssignment{IdentityKey: "id", Port: "usb:001,002", Runtime: RuntimeNativeWindows}, Listener: TetherListener{Status: TetherStatusStopped}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != TestCaptureSuccess || result.FileName == "old.jpg" {
		t.Fatalf("result = %+v", result)
	}
}

func TestTestCaptureBadSOIFailsSafely(t *testing.T) {
	dir := t.TempDir()
	service := NewTestCaptureService(fakeCaptureRunner{write: false})
	_, err := service.Run(context.Background(), TestCaptureConfig{StationID: "station_1", InputFolder: dir, Assignment: &TestCaptureAssignment{IdentityKey: "id", Port: "usb:001,002", Runtime: RuntimeNativeWindows}, Listener: TetherListener{Status: TetherStatusStopped}})
	if err == nil {
		t.Fatalf("expected failure")
	}
}

func TestTestCaptureWatcherValidationRespectsCaptureTimeout(t *testing.T) {
	dir := t.TempDir()
	service := NewTestCaptureService(fakeCaptureRunner{write: false})
	start := time.Now()
	_, err := service.Run(context.Background(), TestCaptureConfig{StationID: "station_1", InputFolder: dir, Assignment: &TestCaptureAssignment{IdentityKey: "id", Port: "usb:001,002", Runtime: RuntimeNativeWindows}, Listener: TetherListener{Status: TetherStatusStopped}, Timeout: 75 * time.Millisecond, Stability: 10 * time.Millisecond})
	if err == nil {
		t.Fatalf("expected timeout failure")
	}
	if elapsed := time.Since(start); elapsed > 350*time.Millisecond {
		t.Fatalf("watcher validation exceeded capture timeout budget: %s", elapsed)
	}
}
