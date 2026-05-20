package upload

import "time"

const (
	StatusPending   = "pending"
	StatusResolving = "resolving"
	StatusReady     = "ready"
	StatusFailed    = "failed"

	ErrorNotConfigured          = "CLOUD_NOT_CONFIGURED"
	ErrorNotAuthorized          = "CLOUD_NOT_AUTHORIZED"
	ErrorTargetRuleInvalid      = "CLOUD_TARGET_RULE_INVALID"
	ErrorPrefixResolveFailed    = "CLOUD_PREFIX_RESOLVE_FAILED"
	ErrorTargetSaveFailed       = "CLOUD_TARGET_SAVE_FAILED"
	ErrorPendingLocalCompletion = "CLOUD_PENDING_LOCAL_COMPLETION"

	ActionFixCredentials      = "FIX_DRIVE_CREDENTIALS"
	ActionRetryCheck          = "RETRY_DRIVE_CHECK"
	ActionFixRules            = "FIX_DRIVE_FOLDER_RULES"
	ActionRetryTarget         = "RETRY_DRIVE_FOLDER"
	ActionFixDriveFolder      = "FIX_DRIVE_FOLDER"
	ActionWaitLocalCompletion = "WAIT_FOR_LOCAL_COMPLETION"
)

type DriveFolderRef struct {
	Level    string `json:"level"`
	Name     string `json:"name"`
	FolderID string `json:"folder_id"`
	ParentID string `json:"parent_id,omitempty"`
}

type SessionCloudTarget struct {
	SessionID            string           `json:"session_id"`
	StationID            string           `json:"station_id"`
	BucketName           string           `json:"bucket_name,omitempty"`
	TargetRootPrefix     string           `json:"target_root_prefix,omitempty"`
	ObjectPrefix         string           `json:"object_prefix,omitempty"`
	DriveRootFolderID    string           `json:"drive_root_folder_id,omitempty"`
	DriveRootFolderName  string           `json:"drive_root_folder_name,omitempty"`
	DriveFolderPath      string           `json:"drive_folder_path,omitempty"`
	DriveSessionFolderID string           `json:"drive_session_folder_id,omitempty"`
	DriveFolderChain     []DriveFolderRef `json:"drive_folder_chain,omitempty"`
	RemoteIdentity       string           `json:"remote_identity"`
	Status               string           `json:"status"`
	AttemptCount         int              `json:"attempt_count"`
	LastErrorCode        string           `json:"last_error_code,omitempty"`
	LastErrorAction      string           `json:"last_error_action,omitempty"`
	LastCheckedAt        *time.Time       `json:"last_checked_at,omitempty"`
	ResolvedAt           *time.Time       `json:"resolved_at,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

func (t SessionCloudTarget) PublicStatus() string {
	if t.Status == "" {
		return StatusPending
	}
	return t.Status
}
