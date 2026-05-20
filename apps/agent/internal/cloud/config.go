package cloud

import "time"

const (
	ProviderGCS         = "gcs"
	ProviderGoogleDrive = "google_drive"

	StatusNotConfigured = "not_configured"
	StatusChecking      = "checking"
	StatusAuthorized    = "authorized"
	StatusFailed        = "failed"

	ActionFixCredentials = "FIX_DRIVE_CREDENTIALS"
	ActionFixBucket      = "FIX_CLOUD_BUCKET"
	ActionFixDriveFolder = "FIX_DRIVE_FOLDER"
	ActionRetryCheck     = "RETRY_DRIVE_CHECK"
	ActionFixRules       = "FIX_DRIVE_FOLDER_RULES"
)

type Settings struct {
	Provider             string     `json:"provider"`
	BucketName           string     `json:"bucket_name,omitempty"`
	TargetRootPrefix     string     `json:"target_root_prefix,omitempty"`
	ObjectNamingTemplate string     `json:"object_naming_template,omitempty"`
	DriveRootFolderID    string     `json:"drive_root_folder_id,omitempty"`
	DriveRootFolderName  string     `json:"drive_root_folder_name,omitempty"`
	FolderNamingTemplate string     `json:"folder_naming_template,omitempty"`
	ServiceAccountJSON   string     `json:"service_account_json,omitempty"`
	CredentialFilePath   string     `json:"credential_file_path,omitempty"`
	ConnectionStatus     string     `json:"connection_status"`
	LastCheckedAt        *time.Time `json:"last_checked_at,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	LastErrorCode        string     `json:"last_error_code,omitempty"`
	LastErrorAction      string     `json:"last_error_action,omitempty"`
}

type PublicSettings struct {
	Provider              string     `json:"provider"`
	DriveRootFolderID     string     `json:"drive_root_folder_id,omitempty"`
	DriveRootFolderName   string     `json:"drive_root_folder_name,omitempty"`
	FolderNamingTemplate  string     `json:"folder_naming_template"`
	CredentialsConfigured bool       `json:"credentials_configured"`
	ConnectionStatus      string     `json:"connection_status"`
	LastCheckedAt         *time.Time `json:"last_checked_at,omitempty"`
	LastErrorCode         string     `json:"last_error_code,omitempty"`
	LastErrorAction       string     `json:"last_error_action,omitempty"`
}

type UpdateRequest struct {
	Provider            string `json:"provider"`
	BucketName          string `json:"bucket_name"`
	TargetRootPrefix    string `json:"target_root_prefix"`
	DriveRootFolderID   string `json:"drive_root_folder_id"`
	DriveRootFolderName string `json:"drive_root_folder_name"`
	ServiceAccountJSON  string `json:"service_account_json"`
	CredentialFilePath  string `json:"credential_file_path"`
}

const ObjectNamingTemplate = "{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}/{asset_kind}/{safe_file_name}"
const FolderNamingTemplate = "{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}"

func DefaultSettings() Settings {
	return Settings{Provider: ProviderGoogleDrive, ObjectNamingTemplate: ObjectNamingTemplate, FolderNamingTemplate: FolderNamingTemplate, ConnectionStatus: StatusNotConfigured}
}

func (s Settings) CredentialsConfigured() bool {
	return s.ServiceAccountJSON != "" || s.CredentialFilePath != ""
}

func (s Settings) Public() PublicSettings {
	status := s.ConnectionStatus
	if status == "" {
		status = StatusNotConfigured
	}
	provider := s.Provider
	if provider == "" {
		provider = ProviderGoogleDrive
	}
	folderTemplate := s.FolderNamingTemplate
	if folderTemplate == "" {
		folderTemplate = FolderNamingTemplate
	}
	return PublicSettings{Provider: provider, DriveRootFolderID: s.DriveRootFolderID, DriveRootFolderName: s.DriveRootFolderName, FolderNamingTemplate: folderTemplate, CredentialsConfigured: s.CredentialsConfigured(), ConnectionStatus: status, LastCheckedAt: s.LastCheckedAt, LastErrorCode: s.LastErrorCode, LastErrorAction: s.LastErrorAction}
}
