package cameras

import "testing"

func TestParseAutoDetectSingleCamera(t *testing.T) {
	result := ParseAutoDetect("Model                          Port\n----------------------------------------------------------\nSony Alpha-A6000               usb:001,004\n", RuntimeWSL)
	if len(result.Cameras) != 1 {
		t.Fatalf("len(cameras) = %d, want 1", len(result.Cameras))
	}
	camera := result.Cameras[0]
	if camera.IdentityKey != "wsl|usb:001,004|sony_alpha_a6000" {
		t.Fatalf("identity_key = %q", camera.IdentityKey)
	}
	if camera.Name != "Sony Alpha-A6000" || camera.Port != "usb:001,004" || camera.Transport != TransportUSB || !camera.Connected {
		t.Fatalf("unexpected camera: %+v", camera)
	}
}

func TestParseAutoDetectMultiCameraWithWarnings(t *testing.T) {
	output := "WARNING: could not load something\nModel                          Port\n----------------------------------------------------------\nSony Alpha-A6000               usb:001,004\nCanon EOS                      usb:001,005\nextra localized noise\n"
	result := ParseAutoDetect(output, RuntimeNativeWindows)
	if len(result.Cameras) != 2 {
		t.Fatalf("len(cameras) = %d, want 2 diagnostics=%v", len(result.Cameras), result.Diagnostics)
	}
	if result.Cameras[1].IdentityKey != "native_windows|usb:001,005|canon_eos" {
		t.Fatalf("second identity_key = %q", result.Cameras[1].IdentityKey)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected safe diagnostics for warning/noise")
	}
}

func TestParseAutoDetectNoCameras(t *testing.T) {
	result := ParseAutoDetect("Model                          Port\n----------------------------------------------------------\n", RuntimeWSL)
	if len(result.Cameras) != 0 {
		t.Fatalf("len(cameras) = %d, want 0", len(result.Cameras))
	}
	if result.Status != DiscoveryStatusNoCameras || result.Action != ActionConnectCamera {
		t.Fatalf("status/action = %s/%s", result.Status, result.Action)
	}
}

func TestParseAutoDetectNonUSBPortsAsCameras(t *testing.T) {
	output := "Model                          Port\n----------------------------------------------------------\nSony PTP Camera                ptpip:192.168.1.50\nLegacy Camera                  serial:/dev/ttyS0\n"
	result := ParseAutoDetect(output, RuntimeWSL)
	if len(result.Cameras) != 2 {
		t.Fatalf("len(cameras) = %d, want 2 diagnostics=%v", len(result.Cameras), result.Diagnostics)
	}
	if result.Cameras[0].Port != "ptpip:192.168.1.50" || result.Cameras[0].Transport != TransportPTP {
		t.Fatalf("ptp camera = %+v", result.Cameras[0])
	}
	if result.Cameras[1].Port != "serial:/dev/ttyS0" || result.Cameras[1].Transport != TransportUnknown {
		t.Fatalf("unknown transport camera = %+v", result.Cameras[1])
	}
}

func TestIdentityKeyNormalizesValues(t *testing.T) {
	got := BuildIdentityKey(" WSL ", " USB:001,004 ", " Sony Alpha A6000 ")
	if got != "wsl|usb:001,004|sony_alpha_a6000" {
		t.Fatalf("identity key = %q", got)
	}
}
