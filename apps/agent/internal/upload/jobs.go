package upload

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"selfstudio/agent/internal/cloud"
)

const (
	AssetKindOriginal = "original"
	AssetKindGraded   = "graded"

	JobStatusPending     = "pending"
	JobStatusUploading   = "uploading"
	JobStatusUploaded    = "uploaded"
	JobStatusFailed      = "failed"
	JobStatusNotEligible = "not_eligible"

	SessionUploadNotConfigured          = "not_configured"
	SessionUploadTargetPending          = "target_pending"
	SessionUploadPending                = "pending"
	SessionUploadUploading              = "uploading"
	SessionUploadUploaded               = "uploaded"
	SessionUploadPartialFailed          = "partial_failed"
	SessionUploadFailed                 = "failed"
	SessionUploadPendingLocalCompletion = "pending_local_completion"

	ErrorUploadPendingLocalCompletion = "UPLOAD_PENDING_LOCAL_COMPLETION"
	ErrorCloudTargetNotReady          = "CLOUD_TARGET_NOT_READY"
	ErrorUploadLocalFileMissing       = "UPLOAD_LOCAL_FILE_MISSING"
	ErrorUploadFailed                 = "UPLOAD_FAILED"
	ErrorUploadStateSaveFailed        = "UPLOAD_STATE_SAVE_FAILED"
	ErrorUploadRemoteCheckNeeded      = "UPLOAD_REMOTE_CHECK_NEEDED"

	ActionResolveCloudTarget = "RESOLVE_CLOUD_TARGET"
	ActionCheckLocalOutput   = "CHECK_LOCAL_OUTPUT"
	ActionRetryCloudUpload   = "RETRY_CLOUD_UPLOAD"
	ActionCheckDriveFile     = "CHECK_DRIVE_FILE"
)

type FileUploadJob struct {
	JobID                string     `json:"job_id"`
	SessionID            string     `json:"session_id"`
	StationID            string     `json:"station_id"`
	PhotoID              string     `json:"photo_id"`
	AssetKind            string     `json:"asset_kind"`
	LocalPath            string     `json:"local_path"`
	BucketName           string     `json:"bucket_name"`
	ObjectKey            string     `json:"object_key"`
	DriveFolderID        string     `json:"drive_folder_id,omitempty"`
	DriveFileID          string     `json:"drive_file_id,omitempty"`
	RemoteIdentity       string     `json:"remote_identity,omitempty"`
	RemoteGeneration     int64      `json:"remote_generation,omitempty"`
	RemoteMetageneration int64      `json:"remote_metageneration,omitempty"`
	RemoteETag           string     `json:"remote_etag,omitempty"`
	DedupeKey            string     `json:"dedupe_key,omitempty"`
	Status               string     `json:"status"`
	AttemptCount         int        `json:"attempt_count"`
	MaxAttempts          int        `json:"max_attempts,omitempty"`
	LastErrorCode        string     `json:"last_error_code,omitempty"`
	LastErrorAction      string     `json:"last_error_action,omitempty"`
	LastAttemptAt        *time.Time `json:"last_attempt_at,omitempty"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	RetryAfterSeconds    int        `json:"retry_after,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	UploadedAt           *time.Time `json:"uploaded_at,omitempty"`
}

func JobID(sessionID, photoID, assetKind string) string {
	return sessionID + ":" + photoID + ":" + assetKind
}

func BuildFileObjectKey(target SessionCloudTarget, assetKind, localPath string) (string, error) {
	if assetKind != AssetKindOriginal && assetKind != AssetKindGraded {
		return "", errors.New("invalid asset kind")
	}
	if target.Status != StatusReady {
		return "", errors.New("cloud target not ready")
	}
	name, err := cloud.SafeFileName(filepath.Base(localPath))
	if err != nil || name == "" {
		return "", errors.New("invalid file name")
	}
	base := strings.TrimRight(target.ObjectPrefix, "/")
	if base == "" {
		base = strings.TrimRight(target.DriveFolderPath, "/")
	}
	if base == "" {
		base = target.DriveSessionFolderID
	}
	if base == "" {
		return "", errors.New("cloud target not ready")
	}
	key := base + "/" + assetKind + "/" + name
	if err := ValidateObjectPrefix(key); err != nil {
		return "", err
	}
	return key, nil
}

func validateJob(j FileUploadJob) error {
	if j.JobID == "" || j.SessionID == "" || j.StationID == "" || j.PhotoID == "" {
		return errors.New("upload job missing identity")
	}
	if j.AssetKind != AssetKindOriginal && j.AssetKind != AssetKindGraded {
		return errors.New("invalid upload job asset kind")
	}
	switch j.Status {
	case JobStatusPending, JobStatusUploading, JobStatusUploaded, JobStatusFailed, JobStatusRetrying, JobStatusRetryScheduled, JobStatusNotEligible:
	default:
		return errors.New("invalid upload job status")
	}
	if j.Status != JobStatusNotEligible && j.LocalPath == "" {
		return errors.New("upload job missing local file")
	}
	if j.Status != JobStatusNotEligible && j.DriveFolderID == "" && (j.BucketName == "" || j.ObjectKey == "") {
		return errors.New("upload job missing file identity")
	}
	if j.ObjectKey != "" {
		if err := ValidateObjectPrefix(j.ObjectKey); err != nil {
			return err
		}
	}
	if j.JobID != JobID(j.SessionID, j.PhotoID, j.AssetKind) {
		return errors.New("upload job id is not deterministic")
	}
	return nil
}
