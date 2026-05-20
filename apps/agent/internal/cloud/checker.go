package cloud

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type CheckResult struct {
	Status       string
	ErrorCode    string
	ErrorMessage string
	ErrorAction  string
}
type Checker interface {
	Check(ctx context.Context, settings Settings) CheckResult
}

type GCSChecker struct{}
type GoogleDriveChecker struct{}

const driveFolderMimeType = "application/vnd.google-apps.folder"

type driveProbeClient interface {
	GetRootFolder(ctx context.Context, folderID string) (*drive.File, error)
	CreateProbeFolder(ctx context.Context, parentID, name string) (*drive.File, error)
	DeleteProbe(ctx context.Context, fileID string) error
}

type driveServiceProbeClient struct{ service *drive.Service }

func (c driveServiceProbeClient) GetRootFolder(ctx context.Context, folderID string) (*drive.File, error) {
	return c.service.Files.Get(folderID).Fields("id,name,mimeType,trashed").SupportsAllDrives(true).Context(ctx).Do()
}

func (c driveServiceProbeClient) CreateProbeFolder(ctx context.Context, parentID, name string) (*drive.File, error) {
	return c.service.Files.Create(&drive.File{Name: name, MimeType: driveFolderMimeType, Parents: []string{parentID}}).Fields("id,name,mimeType").SupportsAllDrives(true).Context(ctx).Do()
}

func (c driveServiceProbeClient) DeleteProbe(ctx context.Context, fileID string) error {
	return c.service.Files.Delete(fileID).SupportsAllDrives(true).Context(ctx).Do()
}

func (GCSChecker) Check(ctx context.Context, s Settings) CheckResult {
	if !s.CredentialsConfigured() {
		return CheckResult{Status: StatusFailed, ErrorCode: "CLOUD_NOT_CONFIGURED", ErrorMessage: "Credential cloud belum dikonfigurasi.", ErrorAction: ActionFixCredentials}
	}
	if err := ValidateSettings(s); err != nil {
		c, m, a := MapValidationError(err)
		return CheckResult{Status: StatusFailed, ErrorCode: c, ErrorMessage: m, ErrorAction: a}
	}
	opts := []option.ClientOption{}
	if s.ServiceAccountJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(s.ServiceAccountJSON)))
	} else if s.CredentialFilePath != "" {
		opts = append(opts, option.WithCredentialsFile(s.CredentialFilePath))
	}
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return safeCheckError(err)
	}
	defer client.Close()
	_, err = client.Bucket(s.BucketName).Attrs(ctx)
	if err != nil {
		return safeCheckError(err)
	}
	return CheckResult{Status: StatusAuthorized}
}

func (GoogleDriveChecker) Check(ctx context.Context, s Settings) CheckResult {
	if !s.CredentialsConfigured() {
		return CheckResult{Status: StatusFailed, ErrorCode: "DRIVE_NOT_CONFIGURED", ErrorMessage: "Credential Google Drive belum dikonfigurasi.", ErrorAction: ActionFixCredentials}
	}
	if err := ValidateSettings(s); err != nil {
		c, m, a := MapValidationError(err)
		return CheckResult{Status: StatusFailed, ErrorCode: c, ErrorMessage: m, ErrorAction: a}
	}
	opts := []option.ClientOption{option.WithScopes(drive.DriveFileScope, drive.DriveMetadataReadonlyScope)}
	if s.ServiceAccountJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(s.ServiceAccountJSON)))
	} else if s.CredentialFilePath != "" {
		opts = append(opts, option.WithCredentialsFile(s.CredentialFilePath))
	}
	service, err := drive.NewService(ctx, opts...)
	if err != nil {
		return safeDriveCheckError(err)
	}
	return CheckGoogleDriveWithClient(ctx, s, driveServiceProbeClient{service: service})
}

func CheckGoogleDriveWithClient(ctx context.Context, s Settings, client driveProbeClient) CheckResult {
	file, err := client.GetRootFolder(ctx, s.DriveRootFolderID)
	if err != nil {
		return safeDriveCheckError(err)
	}
	if file.Trashed || file.MimeType != driveFolderMimeType {
		return CheckResult{Status: StatusFailed, ErrorCode: "DRIVE_FOLDER_INVALID", ErrorMessage: "Root folder Google Drive tidak valid.", ErrorAction: ActionFixDriveFolder}
	}
	probeName := fmt.Sprintf(".selfstudio-drive-write-probe-%d", time.Now().UTC().UnixNano())
	probe, err := client.CreateProbeFolder(ctx, s.DriveRootFolderID, probeName)
	if err != nil {
		return safeDriveCheckError(err)
	}
	if probe != nil && probe.Id != "" {
		if err := client.DeleteProbe(ctx, probe.Id); err != nil {
			return safeDriveCheckError(err)
		}
	}
	return CheckResult{Status: StatusAuthorized}
}

func safeCheckError(err error) CheckResult {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "credential") || strings.Contains(msg, "private") || strings.Contains(msg, "json") {
		return CheckResult{Status: StatusFailed, ErrorCode: "CLOUD_CREDENTIALS_INVALID", ErrorMessage: "Credential cloud tidak valid atau tidak dapat dipakai.", ErrorAction: ActionFixCredentials}
	}
	if strings.Contains(msg, "bucket") || strings.Contains(msg, "403") || strings.Contains(msg, "404") || strings.Contains(msg, "permission") {
		return CheckResult{Status: StatusFailed, ErrorCode: "CLOUD_BUCKET_UNAUTHORIZED", ErrorMessage: "Bucket tidak ditemukan atau tidak diizinkan.", ErrorAction: ActionFixBucket}
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "timeout") {
		return CheckResult{Status: StatusFailed, ErrorCode: "CLOUD_CHECK_FAILED", ErrorMessage: "Connection check timeout. Coba lagi.", ErrorAction: ActionRetryCheck}
	}
	return CheckResult{Status: StatusFailed, ErrorCode: "CLOUD_CHECK_FAILED", ErrorMessage: "Connection check gagal. Coba lagi atau periksa jaringan.", ErrorAction: ActionRetryCheck}
}

func safeDriveCheckError(err error) CheckResult {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "credential") || strings.Contains(msg, "private") || strings.Contains(msg, "json") || strings.Contains(msg, "oauth") || strings.Contains(msg, "token") {
		return CheckResult{Status: StatusFailed, ErrorCode: "DRIVE_CREDENTIALS_INVALID", ErrorMessage: "Credential Google Drive tidak valid atau tidak dapat dipakai.", ErrorAction: ActionFixCredentials}
	}
	if strings.Contains(msg, "403") || strings.Contains(msg, "404") || strings.Contains(msg, "permission") || strings.Contains(msg, "not found") || strings.Contains(msg, "folder") {
		return CheckResult{Status: StatusFailed, ErrorCode: "DRIVE_FOLDER_UNAUTHORIZED", ErrorMessage: "Root folder Google Drive tidak ditemukan atau tidak diizinkan.", ErrorAction: ActionFixDriveFolder}
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "timeout") {
		return CheckResult{Status: StatusFailed, ErrorCode: "DRIVE_CHECK_FAILED", ErrorMessage: "Connection check Google Drive timeout. Coba lagi.", ErrorAction: ActionRetryCheck}
	}
	return CheckResult{Status: StatusFailed, ErrorCode: "DRIVE_CHECK_FAILED", ErrorMessage: "Connection check Google Drive gagal. Coba lagi atau periksa jaringan.", ErrorAction: ActionRetryCheck}
}

type FakeChecker struct{ Result CheckResult }

func (f FakeChecker) Check(ctx context.Context, settings Settings) CheckResult {
	if f.Result.Status == "" {
		return CheckResult{Status: StatusAuthorized}
	}
	return f.Result
}
