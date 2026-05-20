package upload

import (
	"strings"
	"time"

	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/sessions"
)

const SessionPrefixTemplate = "{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}"
const DriveSessionFolderTemplate = "{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}"

func BuildSessionObjectPrefix(settings cloud.Settings, session sessions.Session) (string, string, error) {
	root, err := cloud.SanitizeTargetRoot(settings.TargetRootPrefix)
	if err != nil {
		return "", "", err
	}
	started := session.StartedAt.UTC()
	if started.IsZero() {
		return "", "", cloud.ValidationError{Code: ErrorTargetRuleInvalid, Message: "Timestamp session tidak valid untuk cloud prefix.", Action: ActionFixRules}
	}
	parts := []string{
		root,
		started.Format("2006"),
		started.Format("01"),
		started.Format("02"),
		cloud.SafeSegment(session.CustomerName, "customer-unknown"),
		cloud.SafeSegment(session.OrderNumber, "order-unknown"),
		cloud.SafeSegment(session.StationID, "station"),
		cloud.SafeSegment(session.SessionID, "session"),
	}
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	prefix := strings.Join(clean, "/")
	if err := ValidateObjectPrefix(prefix); err != nil {
		return "", "", err
	}
	return prefix, "gcs://" + settings.BucketName + "/" + prefix, nil
}

func BuildSessionDriveFolderPath(settings cloud.Settings, session sessions.Session) (string, string, error) {
	started := session.StartedAt.UTC()
	if started.IsZero() {
		return "", "", cloud.ValidationError{Code: ErrorTargetRuleInvalid, Message: "Timestamp session tidak valid untuk Drive folder.", Action: ActionFixRules}
	}
	preview, err := cloud.BuildDriveFolderPreview(cloud.PreviewInput{CustomerName: session.CustomerName, OrderNumber: session.OrderNumber, StationID: session.StationID, SessionID: session.SessionID, TakenAt: started})
	if err != nil {
		return "", "", err
	}
	return preview.FolderPath, "drive://" + settings.DriveRootFolderID + "/" + preview.FolderPath, nil
}

func ValidateObjectPrefix(prefix string) error {
	if prefix == "" || strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "\\") || strings.Contains(prefix, "//") || strings.Contains(prefix, "./") || strings.Contains(prefix, "../") || len(prefix) > 900 {
		return cloud.ValidationError{Code: ErrorTargetRuleInvalid, Message: "Object prefix session tidak aman atau terlalu panjang.", Action: ActionFixRules}
	}
	for _, part := range strings.Split(prefix, "/") {
		if part == "" || part == "." || part == ".." {
			return cloud.ValidationError{Code: ErrorTargetRuleInvalid, Message: "Object prefix session memiliki segment kosong/tidak aman.", Action: ActionFixRules}
		}
		for _, r := range part {
			if r < 32 || r == 127 {
				return cloud.ValidationError{Code: ErrorTargetRuleInvalid, Message: "Object prefix session berisi control character.", Action: ActionFixRules}
			}
		}
	}
	return nil
}

func nowUTC() time.Time { return time.Now().UTC() }
