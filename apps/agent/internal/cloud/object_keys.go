package cloud

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type ValidationError struct{ Code, Message, Action string }

func (e ValidationError) Error() string { return e.Message }

var bucketRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,61}[a-z0-9]$`)

func ValidateSettings(s Settings) error {
	if s.Provider == "" {
		s.Provider = ProviderGoogleDrive
	}
	if s.Provider != ProviderGoogleDrive {
		return ValidationError{"CLOUD_PROVIDER_UNSUPPORTED", "Provider cloud tidak didukung.", ActionFixRules}
	}
	if strings.TrimSpace(s.DriveRootFolderID) == "" {
		return ValidationError{"DRIVE_FOLDER_REQUIRED", "Root folder Google Drive wajib diisi.", ActionFixDriveFolder}
	}
	if !isSafeDriveID(s.DriveRootFolderID) {
		return ValidationError{"DRIVE_FOLDER_INVALID", "Root folder Google Drive tidak valid.", ActionFixDriveFolder}
	}
	if s.DriveRootFolderName != "" {
		if _, err := SafeDriveSegment(s.DriveRootFolderName, "root"); err != nil {
			return err
		}
	}
	return nil
}

func SanitizeTargetRoot(root string) (string, error) {
	root = strings.Trim(strings.TrimSpace(root), "/")
	if root == "" {
		return "", nil
	}
	if strings.Contains(root, "\\") || strings.Contains(root, "//") || strings.Contains(root, "./") || strings.Contains(root, "../") {
		return "", ValidationError{"CLOUD_TARGET_RULE_INVALID", "Target root tidak boleh berisi traversal path.", ActionFixRules}
	}
	parts := strings.Split(root, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || hasControl(p) {
			return "", ValidationError{"CLOUD_TARGET_RULE_INVALID", "Target root memiliki segment tidak aman.", ActionFixRules}
		}
	}
	return root, nil
}

func SafeSegment(input, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	var b strings.Builder
	lastSep := false
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSep = false
			continue
		}
		if r == '-' || r == '_' || r == ' ' || r == '.' {
			if !lastSep {
				b.WriteByte('-')
				lastSep = true
			}
			continue
		}
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" || out == "." || out == ".." {
		out = fallback
	}
	return out
}

func SafeFileName(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || hasControl(name) {
		return "", ValidationError{"CLOUD_TARGET_RULE_INVALID", "Nama file tidak aman.", ActionFixRules}
	}
	base := filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(base))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if ext != ".jpg" && ext != ".jpeg" {
		ext = SafeSegment(ext, "")
	}
	safeStem := SafeSegment(stem, "file")
	if safeStem == "" {
		return "", ValidationError{"CLOUD_TARGET_RULE_INVALID", "Nama file akhir kosong.", ActionFixRules}
	}
	return safeStem + ext, nil
}

type PreviewInput struct {
	TargetRootPrefix string    `json:"target_root_prefix"`
	CustomerName     string    `json:"customer_name"`
	OrderNumber      string    `json:"order_number"`
	StationID        string    `json:"station_id"`
	SessionID        string    `json:"session_id"`
	AssetKind        string    `json:"asset_kind"`
	FileName         string    `json:"file_name"`
	TakenAt          time.Time `json:"taken_at"`
}
type Preview struct {
	ObjectKey  string `json:"object_key,omitempty"`
	FolderPath string `json:"folder_path,omitempty"`
	Template   string `json:"template"`
}

func BuildObjectKey(in PreviewInput) (Preview, error) {
	root, err := SanitizeTargetRoot(in.TargetRootPrefix)
	if err != nil {
		return Preview{}, err
	}
	if in.TakenAt.IsZero() {
		in.TakenAt = time.Now().UTC()
	}
	file, err := SafeFileName(in.FileName)
	if err != nil {
		return Preview{}, err
	}
	asset := SafeSegment(in.AssetKind, "original")
	if asset != "original" && asset != "graded" {
		asset = "original"
	}
	parts := []string{root, in.TakenAt.Format("2006"), in.TakenAt.Format("01"), in.TakenAt.Format("02"), SafeSegment(in.CustomerName, "customer-unknown"), SafeSegment(in.OrderNumber, "order-unknown"), SafeSegment(in.StationID, "station"), SafeSegment(in.SessionID, "session"), asset, file}
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	key := strings.Join(clean, "/")
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "../") || strings.Contains(key, "./") || len(key) > 1024 {
		return Preview{}, ValidationError{"CLOUD_TARGET_RULE_INVALID", "Object key tidak aman atau terlalu panjang.", ActionFixRules}
	}
	return Preview{ObjectKey: key, Template: ObjectNamingTemplate}, nil
}

func BuildDriveFolderPreview(in PreviewInput) (Preview, error) {
	if in.TakenAt.IsZero() {
		in.TakenAt = time.Now().UTC()
	}
	parts := []string{in.TakenAt.Format("2006"), in.TakenAt.Format("01"), in.TakenAt.Format("02")}
	for _, candidate := range []struct{ value, fallback string }{
		{in.CustomerName, "customer-unknown"},
		{in.OrderNumber, "order-unknown"},
		{in.StationID, "station"},
		{in.SessionID, "session"},
	} {
		segment, err := SafeDriveSegment(candidate.value, candidate.fallback)
		if err != nil {
			return Preview{}, err
		}
		parts = append(parts, segment)
	}
	path := strings.Join(parts, "/")
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "../") || strings.Contains(path, "./") || len(path) > 512 {
		return Preview{}, ValidationError{"DRIVE_FOLDER_RULE_INVALID", "Folder path Google Drive tidak aman atau terlalu panjang.", ActionFixRules}
	}
	return Preview{FolderPath: path, Template: FolderNamingTemplate}, nil
}

func SafeDriveSegment(input, fallback string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if strings.Contains(trimmed, "..") || hasControl(trimmed) {
		return "", ValidationError{"DRIVE_FOLDER_RULE_INVALID", "Nama folder Google Drive tidak aman.", ActionFixRules}
	}
	segment := SafeSegment(trimmed, fallback)
	if len(segment) > 120 {
		segment = strings.Trim(segment[:120], "-._")
	}
	if segment == "" || segment == "." || segment == ".." {
		return "", ValidationError{"DRIVE_FOLDER_RULE_INVALID", "Nama folder Google Drive kosong atau tidak aman.", ActionFixRules}
	}
	return segment, nil
}

func isSafeDriveID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 256 || strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") || hasControl(id) {
		return false
	}
	return true
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func MapValidationError(err error) (string, string, string) {
	if v, ok := err.(ValidationError); ok {
		return v.Code, v.Message, v.Action
	}
	return "CLOUD_TARGET_RULE_INVALID", fmt.Sprint(err), ActionFixRules
}
