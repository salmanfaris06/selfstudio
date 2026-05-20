package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type UploadResult struct {
	RemoteIdentity       string
	DriveFolderID        string
	DriveFileID          string
	RemoteGeneration     int64
	RemoteMetageneration int64
	RemoteETag           string
	AlreadyExisted       bool
}
type Uploader interface {
	Upload(ctx context.Context, bucketName, objectKey, localPath string) (UploadResult, error)
}

type SafeUploadError struct{ Code, Action string }

func (e SafeUploadError) Error() string { return e.Code }

func MapUploadError(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	var se SafeUploadError
	if ok := AsSafeUploadError(err, &se); ok {
		return se.Code, se.Action
	}
	mapped := MapDriveUploadError(err)
	return mapped.Code, mapped.Action
}

func AsSafeUploadError(err error, target *SafeUploadError) bool {
	if e, ok := err.(SafeUploadError); ok {
		*target = e
		return true
	}
	if e, ok := err.(*SafeUploadError); ok {
		*target = *e
		return true
	}
	return false
}

// LocalCopyUploader is a test-only uploader. Production wiring must use the configured cloud provider uploader.
type LocalCopyUploader struct{ DestinationRoot string }

func (u LocalCopyUploader) Upload(ctx context.Context, bucketName, objectKey, localPath string) (UploadResult, error) {
	select {
	case <-ctx.Done():
		return UploadResult{}, ctx.Err()
	default:
	}
	if strings.TrimSpace(bucketName) == "" || strings.TrimSpace(objectKey) == "" {
		return UploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	src, err := os.Open(localPath)
	if err != nil {
		return UploadResult{}, SafeUploadError{Code: ErrorUploadLocalFileMissing, Action: ActionCheckLocalOutput}
	}
	defer src.Close()
	if u.DestinationRoot == "" {
		return UploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	dstPath := u.DestinationRoot + string(os.PathSeparator) + bucketName + string(os.PathSeparator) + strings.ReplaceAll(objectKey, "/", string(os.PathSeparator))
	if err := os.MkdirAll(strings.TrimSuffix(dstPath, string(os.PathSeparator)+filepathBase(dstPath)), 0o755); err != nil {
		return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
	}
	if err := dst.Close(); err != nil {
		return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
	}
	return UploadResult{RemoteIdentity: fmt.Sprintf("gs://%s/%s", bucketName, objectKey)}, nil
}

func filepathBase(p string) string {
	i := strings.LastIndexAny(p, `/\\`)
	if i >= 0 {
		return p[i+1:]
	}
	return p
}

type TimeoutUploader struct {
	Inner   Uploader
	Timeout time.Duration
}

func (u TimeoutUploader) Upload(ctx context.Context, bucketName, objectKey, localPath string) (UploadResult, error) {
	timeout := u.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return u.Inner.Upload(c, bucketName, objectKey, localPath)
}

func (u TimeoutUploader) Stat(ctx context.Context, bucketName, objectKey string) (RemoteObjectInfo, error) {
	verifier, ok := u.Inner.(RemoteVerifier)
	if !ok {
		return RemoteObjectInfo{}, SafeUploadError{Code: ErrorUploadObjectCheckNeeded, Action: ActionCheckCloudObject}
	}
	timeout := u.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return verifier.Stat(c, bucketName, objectKey)
}
