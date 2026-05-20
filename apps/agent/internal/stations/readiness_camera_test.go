package stations

import (
	"context"
	"testing"
)

type fakeDiscovery struct{ result CameraDiscoveryResult }

func (f fakeDiscovery) Discover(context.Context) (CameraDiscoveryResult, error) { return f.result, nil }

type fakeTether struct{ status string }

func (f fakeTether) Status(stationID string) TetherStatusResult {
	return TetherStatusResult{Status: f.status, LastErrorAction: "START_TETHER_LISTENER"}
}

type fakeCaptureResults struct{ result TestCaptureReadinessResult }

func (f fakeCaptureResults) LastReadiness(stationID string) (TestCaptureReadinessResult, bool) {
	return f.result, true
}

func TestCameraReadinessOptionalMissingAssignmentWarns(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	readiness := NewReadinessValidatorWithCamera(outputRoot, CameraReadinessOptions{Required: false, Tether: fakeTether{status: "stopped"}}).Check(station)
	if readiness.Status != ReadinessWarning {
		t.Fatalf("status = %q, want warning", readiness.Status)
	}
	if got := findCheck(t, readiness, "camera_assignment"); got.Status != ReadinessWarning || got.Action != "ASSIGN_CAMERA" {
		t.Fatalf("camera_assignment = %+v", got)
	}
}

func TestCameraReadinessRequiredMissingAssignmentFails(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	readiness := NewReadinessValidatorWithCamera(outputRoot, CameraReadinessOptions{Required: true, Tether: fakeTether{status: "stopped"}}).Check(station)
	if readiness.Status != ReadinessFailed {
		t.Fatalf("status = %q, want failed", readiness.Status)
	}
}

func TestCameraReadinessAssignedCameraPresentReady(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	station.CameraAssignment = &CameraAssignment{IdentityKey: "native_windows|usb:001,002|sony", CameraName: "Sony", Port: "usb:001,002", Runtime: "native_windows"}
	readiness := NewReadinessValidatorWithCamera(outputRoot, CameraReadinessOptions{Required: true, Discovery: fakeDiscovery{result: CameraDiscoveryResult{Status: "ready", Action: "NONE", Cameras: []CameraDetected{{IdentityKey: station.CameraAssignment.IdentityKey, Connected: true}}}}, Tether: fakeTether{status: "running"}, TestCaptureResults: fakeCaptureResults{result: TestCaptureReadinessResult{Status: "success", Label: "ok", Action: "NONE"}}}).Check(station)
	if readiness.Status != ReadinessReady {
		t.Fatalf("aggregate status = %q, checks=%+v", readiness.Status, readiness.Checks)
	}
	if got := findCheck(t, readiness, "device"); got.Status != ReadinessReady {
		t.Fatalf("legacy device should be superseded when assigned camera is ready: %+v", got)
	}
	if got := findCheck(t, readiness, "camera_connected"); got.Status != ReadinessReady {
		t.Fatalf("camera_connected = %+v", got)
	}
	if got := findCheck(t, readiness, "tether_listener"); got.Status != ReadinessReady {
		t.Fatalf("tether_listener = %+v", got)
	}
	if got := findCheck(t, readiness, "camera_test_capture"); got.Status != ReadinessReady {
		t.Fatalf("camera_test_capture = %+v", got)
	}
}
