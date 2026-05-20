package stations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadinessReadyWithDeviceUnknownBecomesWarning(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	readiness := NewReadinessValidator(outputRoot).Check(station)

	if readiness.Status != ReadinessWarning {
		t.Fatalf("Status = %q, want warning", readiness.Status)
	}
	device := findCheck(t, readiness, "device")
	if device.Status != ReadinessUnknown {
		t.Fatalf("device status = %q, want unknown", device.Status)
	}
}

func TestReadinessFailsMissingInputFolder(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	station.InputFolder = filepath.Join(t.TempDir(), "missing")
	readiness := NewReadinessValidator(outputRoot).Check(station)

	if readiness.Status != ReadinessFailed {
		t.Fatalf("Status = %q, want failed", readiness.Status)
	}
	check := findCheck(t, readiness, "input_folder")
	if check.Status != ReadinessFailed {
		t.Fatalf("input status = %q", check.Status)
	}
}

func TestReadinessFailsMissingLUT(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	station.DefaultLUTPath = filepath.Join(t.TempDir(), "missing.cube")
	readiness := NewReadinessValidator(outputRoot).Check(station)

	if readiness.Status != ReadinessFailed {
		t.Fatalf("Status = %q, want failed", readiness.Status)
	}
	check := findCheck(t, readiness, "default_lut")
	if check.Status != ReadinessFailed {
		t.Fatalf("lut status = %q", check.Status)
	}
}

func TestReadinessAcceptsUppercaseLUTExtension(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	station.DefaultLUTPath = filepath.Join(t.TempDir(), "DEFAULT.CUBE")
	if err := os.WriteFile(station.DefaultLUTPath, []byte("TITLE test"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readiness := NewReadinessValidator(outputRoot).Check(station)

	check := findCheck(t, readiness, "default_lut")
	if check.Status != ReadinessReady {
		t.Fatalf("lut status = %q", check.Status)
	}
}

func TestReadinessFailsMissingOutputRuleDirectory(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	station.OutputRule = filepath.ToSlash(filepath.Join("missing", "{station_id}"))
	readiness := NewReadinessValidator(outputRoot).Check(station)

	check := findCheck(t, readiness, "output_folder")
	if check.Status != ReadinessFailed {
		t.Fatalf("output status = %q", check.Status)
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "missing")); !os.IsNotExist(err) {
		t.Fatalf("missing output dir err = %v, want not exist", err)
	}
}

func TestReadinessFailsUnsafeOutputRule(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	station.OutputRule = "../outside/{station_id}"
	readiness := NewReadinessValidator(outputRoot).Check(station)

	check := findCheck(t, readiness, "output_folder")
	if check.Status != ReadinessFailed {
		t.Fatalf("output status = %q", check.Status)
	}
}

func TestReadinessFailsInvalidLUTExtension(t *testing.T) {
	station, outputRoot := readyStationFixture(t)
	station.DefaultLUTPath = filepath.Join(t.TempDir(), "default.txt")
	if err := os.WriteFile(station.DefaultLUTPath, []byte("lut"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readiness := NewReadinessValidator(outputRoot).Check(station)

	check := findCheck(t, readiness, "default_lut")
	if check.Status != ReadinessFailed {
		t.Fatalf("lut status = %q", check.Status)
	}
}

func readyStationFixture(t *testing.T) (Station, string) {
	t.Helper()
	root := t.TempDir()
	input := filepath.Join(root, "input")
	output := filepath.Join(root, "output")
	stationOutput := filepath.Join(output, Station1ID)
	lut := filepath.Join(root, "default.cube")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatalf("MkdirAll(input) error = %v", err)
	}
	if err := os.MkdirAll(stationOutput, 0o755); err != nil {
		t.Fatalf("MkdirAll(stationOutput) error = %v", err)
	}
	if err := os.WriteFile(lut, []byte("TITLE test"), 0o644); err != nil {
		t.Fatalf("WriteFile(lut) error = %v", err)
	}
	return Station{StationID: Station1ID, Name: "Station 1", DeviceIdentifier: "Sony", InputFolder: input, BackgroundName: "White", DefaultLUTPath: lut, OutputRule: "{station_id}"}, output
}

func findCheck(t *testing.T, readiness Readiness, key string) ReadinessCheck {
	t.Helper()
	for _, check := range readiness.Checks {
		if check.CheckKey == key {
			return check
		}
	}
	t.Fatalf("check %q not found", key)
	return ReadinessCheck{}
}
