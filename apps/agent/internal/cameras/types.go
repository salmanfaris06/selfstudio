package cameras

import "time"

type Runtime string

const (
	RuntimeNativeWindows Runtime = "native_windows"
	RuntimeWSL           Runtime = "wsl"
	RuntimeUnknown       Runtime = "unknown"
)

type Transport string

const (
	TransportUSB     Transport = "usb"
	TransportPTP     Transport = "ptp"
	TransportUnknown Transport = "unknown"
)

type DiscoveryStatus string

const (
	DiscoveryStatusReady             DiscoveryStatus = "ready"
	DiscoveryStatusNoCameras         DiscoveryStatus = "no_cameras"
	DiscoveryStatusGPhoto2Missing    DiscoveryStatus = "gphoto2_missing"
	DiscoveryStatusWSLMissing        DiscoveryStatus = "wsl_missing"
	DiscoveryStatusUSBIPDCheckNeeded DiscoveryStatus = "usbipd_check_needed"
	DiscoveryStatusError             DiscoveryStatus = "error"
)

type SafeAction string

const (
	ActionNone               SafeAction = "NONE"
	ActionInstallGPhoto2     SafeAction = "INSTALL_GPHOTO2"
	ActionCheckWSL           SafeAction = "CHECK_WSL"
	ActionCheckUSBIPD        SafeAction = "CHECK_USBIPD"
	ActionConnectCamera      SafeAction = "CONNECT_CAMERA"
	ActionRetryDiscovery     SafeAction = "RETRY_CAMERA_DISCOVERY"
	ActionCheckCameraUSBMode SafeAction = "CHECK_CAMERA_USB_MODE"
	ActionChooseDifferent    SafeAction = "CHOOSE_DIFFERENT_CAMERA"
)

type DetectedCamera struct {
	IdentityKey string    `json:"identity_key"`
	Name        string    `json:"name"`
	Model       string    `json:"model,omitempty"`
	Port        string    `json:"port"`
	DevicePath  string    `json:"device_path,omitempty"`
	BusID       string    `json:"bus_id,omitempty"`
	Transport   Transport `json:"transport"`
	Runtime     Runtime   `json:"runtime"`
	Connected   bool      `json:"connected"`
	Diagnostics []string  `json:"diagnostics,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
}

type DiscoveryResult struct {
	Status      DiscoveryStatus  `json:"status"`
	Action      SafeAction       `json:"action"`
	Runtime     Runtime          `json:"runtime"`
	Cameras     []DetectedCamera `json:"cameras"`
	Diagnostics []string         `json:"diagnostics"`
	ScannedAt   time.Time        `json:"scanned_at"`
}

type CameraAssignment struct {
	IdentityKey string     `json:"identity_key"`
	CameraName  string     `json:"camera_name"`
	Port        string     `json:"port"`
	Runtime     Runtime    `json:"runtime"`
	AssignedAt  time.Time  `json:"assigned_at"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	Connected   bool       `json:"connected"`
}
