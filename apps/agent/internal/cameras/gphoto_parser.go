package cameras

import (
	"regexp"
	"strings"
	"time"
)

var nonIdentityChars = regexp.MustCompile(`[^a-z0-9]+`)

func BuildIdentityKey(runtimeValue string, port string, name string) string {
	runtimePart := normalizeIdentityPart(runtimeValue)
	portPart := strings.ToLower(strings.TrimSpace(port))
	namePart := normalizeIdentityPart(name)
	if runtimePart == "" {
		runtimePart = string(RuntimeUnknown)
	}
	if portPart == "" {
		portPart = "unknown_port"
	}
	if namePart == "" {
		namePart = "unknown_camera"
	}
	return runtimePart + "|" + portPart + "|" + namePart
}

func normalizeIdentityPart(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	replaced := nonIdentityChars.ReplaceAllString(lower, "_")
	return strings.Trim(replaced, "_")
}

func ParseAutoDetect(output string, runtimeValue Runtime) DiscoveryResult {
	now := time.Now().UTC()
	result := DiscoveryResult{Status: DiscoveryStatusNoCameras, Action: ActionConnectCamera, Runtime: runtimeValue, Cameras: []DetectedCamera{}, Diagnostics: []string{}, ScannedAt: now}
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "---") || strings.EqualFold(trimmed, "Model Port") || strings.HasPrefix(strings.ToLower(trimmed), "model") {
			continue
		}
		name, port, ok := parseAutoDetectRow(trimmed)
		if !ok {
			if len(result.Diagnostics) < 5 {
				result.Diagnostics = append(result.Diagnostics, sanitizeDiagnostic(trimmed))
			}
			continue
		}
		if name == "" || port == "" {
			continue
		}
		camera := DetectedCamera{IdentityKey: BuildIdentityKey(string(runtimeValue), port, name), Name: name, Model: name, Port: port, Transport: transportFromPort(port), Runtime: runtimeValue, Connected: true, DetectedAt: now}
		result.Cameras = append(result.Cameras, camera)
	}
	if len(result.Cameras) > 0 {
		result.Status = DiscoveryStatusReady
		result.Action = ActionNone
	}
	return result
}

func parseAutoDetectRow(line string) (string, string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	portIndex := -1
	for i := len(fields) - 1; i >= 1; i-- {
		if looksLikeGPhotoPort(fields[i]) {
			portIndex = i
			break
		}
	}
	if portIndex < 1 {
		return "", "", false
	}
	name := strings.TrimSpace(strings.Join(fields[:portIndex], " "))
	port := strings.TrimSpace(fields[portIndex])
	return name, port, name != "" && port != ""
}

func looksLikeGPhotoPort(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" || !strings.Contains(lower, ":") {
		return false
	}
	if strings.Contains(lower, "://") {
		return false
	}
	blockedPrefixes := []string{"warning:", "error:", "note:", "model:"}
	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

func transportFromPort(port string) Transport {
	lower := strings.ToLower(strings.TrimSpace(port))
	if strings.HasPrefix(lower, "usb:") {
		return TransportUSB
	}
	if strings.Contains(lower, "ptp") {
		return TransportPTP
	}
	return TransportUnknown
}

func sanitizeDiagnostic(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "diagnostic unavailable"
	}
	blocked := []string{"--", "powershell", "cmd.exe", "token", "secret", "password"}
	lower := strings.ToLower(trimmed)
	for _, word := range blocked {
		if strings.Contains(lower, word) {
			return "safe diagnostic omitted"
		}
	}
	if len(trimmed) > 160 {
		trimmed = trimmed[:160]
	}
	return trimmed
}
