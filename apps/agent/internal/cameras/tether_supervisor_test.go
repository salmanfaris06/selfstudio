package cameras

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeTetherStarter struct {
	mu        sync.Mutex
	starts    int
	processes []*fakeTetherProcess
}
type fakeTetherProcess struct {
	wait   chan error
	killed bool
}

func (p *fakeTetherProcess) Wait() error { return <-p.wait }
func (p *fakeTetherProcess) Kill() error {
	p.killed = true
	select {
	case p.wait <- nil:
	default:
	}
	return nil
}
func (s *fakeTetherStarter) Start(ctx context.Context, spec TetherCommandSpec, output func(string)) (TetherProcess, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	p := &fakeTetherProcess{wait: make(chan error, 1)}
	s.processes = append(s.processes, p)
	output(`Saving file as D:\secret\station_1_capture_20260520_101010_001.JPG`)
	return p, nil
}
func (s *fakeTetherStarter) count() int { s.mu.Lock(); defer s.mu.Unlock(); return s.starts }

type blockingTetherStarter struct {
	started chan struct{}
	release chan struct{}
	proc    *fakeTetherProcess
}

func (s *blockingTetherStarter) Start(ctx context.Context, spec TetherCommandSpec, output func(string)) (TetherProcess, error) {
	close(s.started)
	<-s.release
	s.proc = &fakeTetherProcess{wait: make(chan error, 1)}
	return s.proc, nil
}

func testTetherConfig(t *testing.T, stationID string) TetherStationConfig {
	t.Helper()
	dir := t.TempDir()
	return TetherStationConfig{StationID: stationID, InputFolder: dir, Assignment: &TetherAssignment{IdentityKey: "wsl|usb:001,004|sony", CameraName: "Sony A6000", Port: "usb:001,004", Runtime: RuntimeNativeWindows}}
}

func TestTetherSupervisorDuplicateStartCreatesOneProcess(t *testing.T) {
	starter := &fakeTetherStarter{}
	sup := NewTetherSupervisor(starter)
	cfg := testTetherConfig(t, "station_1")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = sup.Start(context.Background(), cfg) }()
	}
	wg.Wait()
	if starter.count() != 1 {
		t.Fatalf("starts=%d want 1", starter.count())
	}
	got := sup.Status("station_1")
	if got.Status != TetherStatusRunning {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestTetherSupervisorStopOnlyStationListener(t *testing.T) {
	starter := &fakeTetherStarter{}
	sup := NewTetherSupervisor(starter)
	_, _ = sup.Start(context.Background(), testTetherConfig(t, "station_1"))
	_, _ = sup.Start(context.Background(), testTetherConfig(t, "station_2"))
	_, _ = sup.Stop("station_1")
	if sup.Status("station_1").Status != TetherStatusStopped {
		t.Fatal("station_1 not stopped")
	}
	if sup.Status("station_2").Status != TetherStatusRunning {
		t.Fatal("station_2 should keep running")
	}
}

func TestTetherSupervisorStopDuringStartupKillsLateProcess(t *testing.T) {
	starter := &blockingTetherStarter{started: make(chan struct{}), release: make(chan struct{})}
	sup := NewTetherSupervisor(starter)
	cfg := testTetherConfig(t, "station_1")
	done := make(chan TetherListener, 1)
	go func() {
		listener, _ := sup.Start(context.Background(), cfg)
		done <- listener
	}()
	<-starter.started
	stopped, err := sup.Stop("station_1")
	if err != nil || stopped.Status != TetherStatusStopped {
		t.Fatalf("stop during startup got %+v err %v", stopped, err)
	}
	close(starter.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("start did not return after startup cancellation")
	}
	if starter.proc == nil || !starter.proc.killed {
		t.Fatalf("late-started process was not killed: %+v", starter.proc)
	}
	if got := sup.Status("station_1"); got.Status != TetherStatusStopped {
		t.Fatalf("status=%+v want stopped", got)
	}
}

func TestTetherSupervisorExitBecomesError(t *testing.T) {
	starter := &fakeTetherStarter{}
	sup := NewTetherSupervisor(starter)
	_, _ = sup.Start(context.Background(), testTetherConfig(t, "station_1"))
	starter.processes[0].wait <- errors.New("boom D:\\raw\\secret")
	for i := 0; i < 100; i++ {
		if sup.Status("station_1").Status == TetherStatusError {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := sup.Status("station_1")
	if got.Status != TetherStatusError || got.LastErrorAction != ActionRetryTetherListener {
		t.Fatalf("got %+v", got)
	}
}

func TestTetherStartPrerequisites(t *testing.T) {
	sup := NewTetherSupervisor(&fakeTetherStarter{})
	cfg := testTetherConfig(t, "station_1")
	cfg.Assignment = nil
	got, err := sup.Start(context.Background(), cfg)
	if !errors.Is(err, ErrTetherAssignmentRequired) || got.LastErrorCode != "CAMERA_ASSIGNMENT_REQUIRED" || got.LastErrorAction != ActionAssignCamera {
		t.Fatalf("assignment got %+v err %v", got, err)
	}
	cfg = testTetherConfig(t, "station_1")
	cfg.InputFolder = filepath.Join(t.TempDir(), "missing")
	got, err = sup.Start(context.Background(), cfg)
	if !errors.Is(err, ErrTetherInputFolderUnwritable) || got.LastErrorCode != "STATION_INPUT_FOLDER_UNWRITABLE" {
		t.Fatalf("folder got %+v err %v", got, err)
	}
}

func TestBuildTetherCommandNativeAndWSL(t *testing.T) {
	dir := t.TempDir()
	cfg := TetherStationConfig{StationID: "station_1", InputFolder: dir, Assignment: &TetherAssignment{IdentityKey: "x", CameraName: "Sony", Port: "usb:001,004", Runtime: RuntimeNativeWindows}}
	spec, err := BuildTetherCommand(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "gphoto2" || strings.Join(spec.Args, " ") != "--port usb:001,004 --filename "+spec.FilenamePattern+" --wait-event-and-download" {
		t.Fatalf("spec=%+v", spec)
	}
	if !strings.HasPrefix(spec.FilenamePattern, filepath.Clean(dir)) {
		t.Fatalf("pattern outside dir: %s", spec.FilenamePattern)
	}
	cfg.Assignment.Runtime = RuntimeWSL
	cfg.InputFolder = `D:\_Project\selfstudio\local-data\input\station-1`
	spec, err = BuildTetherCommand(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "wsl.exe" || spec.Args[0] != "--" || spec.Args[1] != "gphoto2" || !strings.HasPrefix(spec.FilenamePattern, "/mnt/d/") {
		t.Fatalf("wsl spec=%+v", spec)
	}
}

func TestBuildTetherCommandRejectsUnsafeInputs(t *testing.T) {
	dir := t.TempDir()
	cfg := TetherStationConfig{StationID: "station_1", InputFolder: dir, Assignment: &TetherAssignment{IdentityKey: "x", CameraName: "Sony", Port: "usb:001,004;rm -rf /", Runtime: RuntimeNativeWindows}}
	if _, err := BuildTetherCommand(cfg); !errors.Is(err, ErrTetherInvalidPort) {
		t.Fatalf("err=%v", err)
	}
	cfg.Assignment.Port = "usb:001,004"
	cfg.Assignment.Runtime = RuntimeUnknown
	if _, err := BuildTetherCommand(cfg); !errors.Is(err, ErrTetherInvalidRuntime) {
		t.Fatalf("err=%v", err)
	}
	cfg.Assignment.Runtime = RuntimeWSL
	cfg.InputFolder = `\\server\share\station`
	if _, err := BuildTetherCommand(cfg); err == nil {
		t.Fatal("expected UNC reject")
	}
}

func TestSanitizeTetherDiagnostic(t *testing.T) {
	input := `gphoto2 --filename D:\secret\foo.jpg token=abc`
	got := SanitizeTetherDiagnostic(input)
	if strings.Contains(strings.ToLower(got), "gphoto2") || strings.Contains(got, "D:") || strings.Contains(strings.ToLower(got), "token") {
		t.Fatalf("unsafe: %s", got)
	}
	got = SanitizeTetherDiagnostic(`Saved /mnt/c/secret/station_1_001.JPG`)
	if strings.Contains(got, "/mnt/c/") {
		t.Fatalf("wsl path leaked: %s", got)
	}
	if name := extractSafeDownloadedName(`Saving file as D:\secret\station_1_001.JPG`); name != "station_1_001.JPG" {
		t.Fatalf("name=%s", name)
	}
}

func TestNoIngestionShortcutInTetherPackage(t *testing.T) {
	data, err := os.ReadFile("tether_supervisor.go")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"internal/ingestion", "internal/photos", "internal/sessions", "processing", "upload"}
	for _, f := range forbidden {
		if strings.Contains(string(data), f) {
			t.Fatalf("forbidden import/reference %s", f)
		}
	}
}
