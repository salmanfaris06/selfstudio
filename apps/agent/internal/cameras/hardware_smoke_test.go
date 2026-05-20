package cameras

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSmokeVerifier struct {
	discoverOK    bool
	listenerOK    bool
	driveStatus   string
	driveRequired bool
	failWait      bool
	failIngest    bool
}

func (f fakeSmokeVerifier) Discover(context.Context, HardwareSmokeStation) (bool, SafeAction, string) {
	if !f.discoverOK {
		return false, ActionConnectCamera, "connect camera"
	}
	return true, "", ""
}
func (f fakeSmokeVerifier) EnsureListener(context.Context, HardwareSmokeStation) (bool, SafeAction, string) {
	if !f.listenerOK {
		return false, ActionRetryTetherListener, "retry"
	}
	return true, "", ""
}
func (f fakeSmokeVerifier) PrepareSession(context.Context, HardwareSmokeStation, bool) (string, bool, SafeAction, string) {
	return "session_smoke", true, "", ""
}
func (f fakeSmokeVerifier) WaitForNewJPG(context.Context, string, time.Time, time.Duration) (string, error) {
	if f.failWait {
		return "", os.ErrDeadlineExceeded
	}
	return filepath.Join("input", "new-photo.jpg"), nil
}
func (f fakeSmokeVerifier) VerifyIngestion(context.Context, string, string, time.Duration) (string, int, error) {
	if f.failIngest {
		return "", 0, os.ErrNotExist
	}
	return "session_smoke", 1, nil
}
func (f fakeSmokeVerifier) VerifyLocalOriginal(context.Context, string, string, time.Duration) (string, error) {
	return filepath.Join("local-data", "output", "original.jpg"), nil
}
func (f fakeSmokeVerifier) VerifyGraded(context.Context, string, string, time.Duration) (string, error) {
	return filepath.Join("local-data", "output", "graded.jpg"), nil
}
func (f fakeSmokeVerifier) VerifyDrive(context.Context, string, string, time.Duration) (string, bool, error) {
	status := f.driveStatus
	if status == "" {
		status = "not_configured"
	}
	return status, f.driveRequired, nil
}

func TestHardwareSmokeLocalOnlySkipsDriveAndSerializesSafeReport(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	runner := HardwareSmokeRunner{Verifier: fakeSmokeVerifier{discoverOK: true, listenerOK: true}, Writer: &HardwareSmokeReportWriter{Root: dir}, Now: func() time.Time { return now }}
	report, err := runner.Run(context.Background(), HardwareSmokeStation{StationID: "station_1", StationName: "Station 1", InputFolder: "input", Assignment: &TetherAssignment{IdentityKey: "runtime|usb:001,004|Sony", CameraName: "Sony A6000", Port: "usb:001,004", Runtime: RuntimeWSL}}, HardwareSmokeRequest{Mode: SmokeModeLocalOnly})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.OverallStatus != SmokeStatusPassed {
		t.Fatalf("status = %s", report.OverallStatus)
	}
	if report.Summary.DriveStatus != "not_configured" || report.Summary.DriveRequired {
		t.Fatalf("drive summary = %+v", report.Summary)
	}
	if report.ReportFile == "" {
		t.Fatal("report file missing")
	}
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(b)
	for _, forbidden := range []string{"usb:001", "runtime|", "local-data/output", "stdout raw", "stderr raw", "secret_token_value"} {
		if strings.Contains(jsonText, forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, jsonText)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, report.ReportFile)); err != nil {
		t.Fatalf("saved report missing: %v", err)
	}
}

func TestHardwareSmokeFailureSavesPartialReport(t *testing.T) {
	dir := t.TempDir()
	runner := HardwareSmokeRunner{Verifier: fakeSmokeVerifier{discoverOK: true, listenerOK: true, failIngest: true}, Writer: &HardwareSmokeReportWriter{Root: dir}, Now: func() time.Time { return time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC) }}
	report, err := runner.Run(context.Background(), HardwareSmokeStation{StationID: "station_1", StationName: "Station 1", InputFolder: "input", Assignment: &TetherAssignment{IdentityKey: "id", CameraName: "Cam", Port: "p", Runtime: RuntimeWSL}}, HardwareSmokeRequest{})
	if err == nil {
		t.Fatal("expected failure")
	}
	if report.Summary.FailureCode != "INGESTION_NOT_OBSERVED" {
		t.Fatalf("failure = %+v", report.Summary)
	}
	if report.ReportFile == "" {
		t.Fatal("partial report was not saved")
	}
}

func TestHardwareSmokeReportWriterDoesNotOverwriteExistingReports(t *testing.T) {
	dir := t.TempDir()
	writer := HardwareSmokeReportWriter{Root: dir}
	report := HardwareSmokeReport{ReportID: "same", StartedAt: time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)}
	first, err := writer.Save(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := writer.Save(report)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("expected unique filenames, got %s", first)
	}
}

func TestHardwareSmokeSourceDoesNotContainForbiddenOperations(t *testing.T) {
	b, err := os.ReadFile("hardware_smoke.go")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ToLower(string(b))
	for _, forbidden := range []string{"exec.command(\"usbipd\"", "exec.command(\"winget\"", "exec.command(\"choco\"", "exec.command(\"apt\"", "exec.command(\"taskkill\"", "exec.command(\"pkill\"", "removeall("} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("forbidden operation token present: %s", forbidden)
		}
	}
}
