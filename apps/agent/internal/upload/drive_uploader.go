package upload

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"selfstudio/agent/internal/cloud"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	ErrorDriveUploadUnauthorized = "DRIVE_UPLOAD_UNAUTHORIZED"
	ErrorDriveFolderUnavailable  = "DRIVE_FOLDER_UNAVAILABLE"
	ErrorDriveUploadFailed       = "DRIVE_UPLOAD_FAILED"
	ActionRetryDriveUpload       = "RETRY_DRIVE_UPLOAD"
)

type DriveUploadResult struct {
	DriveFileID    string
	DriveFolderID  string
	RemoteETag     string
	AlreadyExisted bool
}

type DriveFileUploader interface {
	UploadFile(ctx context.Context, folderID string, fileName string, localPath string) (DriveUploadResult, error)
}

type DriveFileClient interface {
	DriveFolderClient
	FindFile(ctx context.Context, folderID, fileName string) ([]DriveUploadResult, error)
	UploadFile(ctx context.Context, folderID, fileName, localPath string) (DriveUploadResult, error)
}

type DriveFileClientFactory func(ctx context.Context) (DriveFileClient, error)

type DriveUploader struct {
	Client  DriveFileClient
	Factory DriveFileClientFactory
}

func (u DriveUploader) client(ctx context.Context) (DriveFileClient, error) {
	if u.Factory != nil {
		return u.Factory(ctx)
	}
	if u.Client != nil {
		return u.Client, nil
	}
	return nil, SafeUploadError{Code: ErrorDriveUploadUnauthorized, Action: ActionFixCredentials}
}

func (u DriveUploader) GetFolder(ctx context.Context, folderID string) (DriveFolder, error) {
	client, err := u.client(ctx)
	if err != nil {
		return DriveFolder{}, MapDriveUploadError(err)
	}
	return client.GetFolder(ctx, folderID)
}
func (u DriveUploader) FindFolder(ctx context.Context, parentID, name string) ([]DriveFolder, error) {
	client, err := u.client(ctx)
	if err != nil {
		return nil, MapDriveUploadError(err)
	}
	return client.FindFolder(ctx, parentID, name)
}
func (u DriveUploader) CreateFolder(ctx context.Context, parentID, name string) (DriveFolder, error) {
	client, err := u.client(ctx)
	if err != nil {
		return DriveFolder{}, MapDriveUploadError(err)
	}
	return client.CreateFolder(ctx, parentID, name)
}

func (u DriveUploader) UploadFile(ctx context.Context, folderID, fileName, localPath string) (DriveUploadResult, error) {
	if strings.TrimSpace(folderID) == "" || strings.TrimSpace(fileName) == "" {
		return DriveUploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	if _, err := os.Stat(localPath); err != nil {
		return DriveUploadResult{}, SafeUploadError{Code: ErrorUploadLocalFileMissing, Action: ActionCheckLocalOutput}
	}
	safeName, err := cloud.SafeFileName(fileName)
	if err != nil {
		return DriveUploadResult{}, SafeUploadError{Code: ErrorDriveUploadFailed, Action: ActionRetryDriveUpload}
	}
	client, err := u.client(ctx)
	if err != nil {
		return DriveUploadResult{}, MapDriveUploadError(err)
	}
	found, err := client.FindFile(ctx, folderID, safeName)
	if err != nil {
		return DriveUploadResult{}, MapDriveUploadError(err)
	}
	if len(found) > 0 {
		res := found[0]
		res.DriveFolderID = folderID
		res.AlreadyExisted = true
		return res, nil
	}
	res, err := client.UploadFile(ctx, folderID, safeName, localPath)
	if err != nil {
		return DriveUploadResult{}, MapDriveUploadError(err)
	}
	if res.DriveFolderID == "" {
		res.DriveFolderID = folderID
	}
	return res, nil
}

type GoogleDriveFileClient struct{ service *drive.Service }

func NewGoogleDriveFileClient(ctx context.Context, s cloud.Settings) (GoogleDriveFileClient, error) {
	opts := []option.ClientOption{option.WithScopes(drive.DriveFileScope, drive.DriveMetadataReadonlyScope)}
	if s.ServiceAccountJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(s.ServiceAccountJSON)))
	} else if s.CredentialFilePath != "" {
		opts = append(opts, option.WithCredentialsFile(s.CredentialFilePath))
	}
	svc, err := drive.NewService(ctx, opts...)
	if err != nil {
		return GoogleDriveFileClient{}, err
	}
	return GoogleDriveFileClient{service: svc}, nil
}

func (c GoogleDriveFileClient) GetFolder(ctx context.Context, folderID string) (DriveFolder, error) {
	return GoogleDriveFolderClient{service: c.service}.GetFolder(ctx, folderID)
}
func (c GoogleDriveFileClient) FindFolder(ctx context.Context, parentID, name string) ([]DriveFolder, error) {
	return GoogleDriveFolderClient{service: c.service}.FindFolder(ctx, parentID, name)
}
func (c GoogleDriveFileClient) CreateFolder(ctx context.Context, parentID, name string) (DriveFolder, error) {
	return GoogleDriveFolderClient{service: c.service}.CreateFolder(ctx, parentID, name)
}
func (c GoogleDriveFileClient) FindFile(ctx context.Context, folderID, fileName string) ([]DriveUploadResult, error) {
	q := "name = '" + escapeDriveQuery(fileName) + "' and '" + escapeDriveQuery(folderID) + "' in parents and mimeType = 'image/jpeg' and trashed = false"
	call := c.service.Files.List().Q(q).Fields("nextPageToken,files(id,parents,md5Checksum,createdTime)").OrderBy("createdTime asc,name asc").PageSize(100).SupportsAllDrives(true).IncludeItemsFromAllDrives(true).Context(ctx)
	out := []DriveUploadResult{}
	for {
		res, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, f := range res.Files {
			parent := folderID
			if len(f.Parents) > 0 {
				parent = f.Parents[0]
			}
			out = append(out, DriveUploadResult{DriveFileID: f.Id, DriveFolderID: parent, RemoteETag: f.Md5Checksum, AlreadyExisted: true})
		}
		if res.NextPageToken == "" {
			break
		}
		call.PageToken(res.NextPageToken)
	}
	return out, nil
}
func (c GoogleDriveFileClient) UploadFile(ctx context.Context, folderID, fileName, localPath string) (DriveUploadResult, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return DriveUploadResult{}, SafeUploadError{Code: ErrorUploadLocalFileMissing, Action: ActionCheckLocalOutput}
	}
	defer f.Close()
	created, err := c.service.Files.Create(&drive.File{Name: fileName, Parents: []string{folderID}, MimeType: "image/jpeg"}).Media(f, googleapi.ContentType("image/jpeg")).Fields("id,parents,md5Checksum").SupportsAllDrives(true).Context(ctx).Do()
	if err != nil {
		return DriveUploadResult{}, err
	}
	parent := folderID
	if len(created.Parents) > 0 {
		parent = created.Parents[0]
	}
	return DriveUploadResult{DriveFileID: created.Id, DriveFolderID: parent, RemoteETag: created.Md5Checksum}, nil
}

func MapDriveUploadError(err error) SafeUploadError {
	if err == nil {
		return SafeUploadError{}
	}
	var se SafeUploadError
	if AsSafeUploadError(err, &se) {
		return se
	}
	if errors.Is(err, os.ErrNotExist) {
		return SafeUploadError{Code: ErrorUploadLocalFileMissing, Action: ActionCheckLocalOutput}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) {
		return SafeUploadError{Code: ErrorDriveUploadFailed, Action: ActionRetryDriveUpload}
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "credential") || strings.Contains(msg, "private") || strings.Contains(msg, "json") || strings.Contains(msg, "oauth") || strings.Contains(msg, "token") || strings.Contains(msg, "unauthorized") {
		return SafeUploadError{Code: ErrorDriveUploadUnauthorized, Action: ActionFixCredentials}
	}
	if strings.Contains(msg, "403") || strings.Contains(msg, "404") || strings.Contains(msg, "permission") || strings.Contains(msg, "not found") || strings.Contains(msg, "folder") {
		return SafeUploadError{Code: ErrorDriveFolderUnavailable, Action: ActionFixDriveFolder}
	}
	return SafeUploadError{Code: ErrorDriveUploadFailed, Action: ActionRetryDriveUpload}
}
