package upload

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"selfstudio/agent/internal/cloud"
)

type GCSUploader struct {
	Settings cloud.Settings
}

func (u GCSUploader) Upload(ctx context.Context, bucketName, objectKey, localPath string) (UploadResult, error) {
	if strings.TrimSpace(bucketName) == "" || strings.TrimSpace(objectKey) == "" {
		return UploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	if err := cloud.ValidateSettings(u.Settings); err != nil || !u.Settings.CredentialsConfigured() || u.Settings.ConnectionStatus != cloud.StatusAuthorized {
		return UploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	if bucketName != u.Settings.BucketName {
		return UploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}

	src, err := os.Open(localPath)
	if err != nil {
		return UploadResult{}, SafeUploadError{Code: ErrorUploadLocalFileMissing, Action: ActionCheckLocalOutput}
	}
	defer src.Close()

	opts := []option.ClientOption{}
	if u.Settings.ServiceAccountJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(u.Settings.ServiceAccountJSON)))
	} else if u.Settings.CredentialFilePath != "" {
		opts = append(opts, option.WithCredentialsFile(u.Settings.CredentialFilePath))
	}
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
	}
	defer client.Close()

	// DoesNotExist is the GCS generation precondition used by this app for create-only,
	// conditionally-idempotent uploads. Retries reuse the same deterministic object key
	// and never create suffix copies or overwrite an existing object without a future
	// explicit identity reconciliation mode.
	obj := client.Bucket(bucketName).Object(objectKey).If(storage.Conditions{DoesNotExist: true})
	writer := obj.NewWriter(ctx)
	writer.ContentType = "image/jpeg"
	if _, err := io.Copy(writer, src); err != nil {
		_ = writer.Close()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
		}
		return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
	}
	if err := writer.Close(); err != nil {
		if isGCSPreconditionConflict(err) || errors.Is(err, storage.ErrObjectNotExist) {
			return UploadResult{}, SafeUploadError{Code: ErrorUploadObjectCheckNeeded, Action: ActionCheckCloudObject}
		}
		return UploadResult{}, SafeUploadError{Code: ErrorUploadFailed, Action: ActionRetryCloudUpload}
	}
	attrs, err := client.Bucket(bucketName).Object(objectKey).Attrs(ctx)
	if err != nil {
		return UploadResult{RemoteIdentity: "gs://" + bucketName + "/" + objectKey}, nil
	}
	return UploadResult{RemoteIdentity: "gs://" + bucketName + "/" + objectKey, RemoteGeneration: attrs.Generation, RemoteMetageneration: attrs.Metageneration, RemoteETag: attrs.Etag}, nil
}

func (u GCSUploader) Stat(ctx context.Context, bucketName, objectKey string) (RemoteObjectInfo, error) {
	if strings.TrimSpace(bucketName) == "" || strings.TrimSpace(objectKey) == "" {
		return RemoteObjectInfo{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	if err := cloud.ValidateSettings(u.Settings); err != nil || !u.Settings.CredentialsConfigured() || u.Settings.ConnectionStatus != cloud.StatusAuthorized {
		return RemoteObjectInfo{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	if bucketName != u.Settings.BucketName {
		return RemoteObjectInfo{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	opts := []option.ClientOption{}
	if u.Settings.ServiceAccountJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(u.Settings.ServiceAccountJSON)))
	} else if u.Settings.CredentialFilePath != "" {
		opts = append(opts, option.WithCredentialsFile(u.Settings.CredentialFilePath))
	}
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return RemoteObjectInfo{}, SafeUploadError{Code: ErrorUploadObjectCheckNeeded, Action: ActionCheckCloudObject}
	}
	defer client.Close()
	attrs, err := client.Bucket(bucketName).Object(objectKey).Attrs(ctx)
	if err != nil {
		return RemoteObjectInfo{}, SafeUploadError{Code: ErrorUploadObjectCheckNeeded, Action: ActionCheckCloudObject}
	}
	return RemoteObjectInfo{RemoteIdentity: "gs://" + bucketName + "/" + objectKey, RemoteGeneration: attrs.Generation, RemoteMetageneration: attrs.Metageneration, RemoteETag: attrs.Etag}, nil
}

func isGCSPreconditionConflict(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == http.StatusPreconditionFailed || gerr.Code == http.StatusConflict
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "precondition") || strings.Contains(msg, "conditionnotmet") || strings.Contains(msg, "already exists")
}
