package upload

import (
	"context"
	"fmt"
	"strings"
	"time"

	"selfstudio/agent/internal/cloud"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const DriveFolderMimeType = "application/vnd.google-apps.folder"

type DriveFolder struct {
	ID          string
	Name        string
	ParentID    string
	CreatedTime time.Time
}

type DriveFolderClient interface {
	FindFolder(ctx context.Context, parentID, name string) ([]DriveFolder, error)
	CreateFolder(ctx context.Context, parentID, name string) (DriveFolder, error)
	GetFolder(ctx context.Context, folderID string) (DriveFolder, error)
}

type GoogleDriveFolderClient struct{ service *drive.Service }

func NewGoogleDriveFolderClient(ctx context.Context, s cloud.Settings) (GoogleDriveFolderClient, error) {
	opts := []option.ClientOption{option.WithScopes(drive.DriveFileScope, drive.DriveMetadataReadonlyScope)}
	if s.ServiceAccountJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(s.ServiceAccountJSON)))
	} else if s.CredentialFilePath != "" {
		opts = append(opts, option.WithCredentialsFile(s.CredentialFilePath))
	}
	svc, err := drive.NewService(ctx, opts...)
	if err != nil {
		return GoogleDriveFolderClient{}, err
	}
	return GoogleDriveFolderClient{service: svc}, nil
}

func (c GoogleDriveFolderClient) GetFolder(ctx context.Context, folderID string) (DriveFolder, error) {
	f, err := c.service.Files.Get(folderID).Fields("id,name,createdTime,trashed").SupportsAllDrives(true).Context(ctx).Do()
	if err != nil {
		return DriveFolder{}, err
	}
	if f.Trashed {
		return DriveFolder{}, fmt.Errorf("drive folder trashed")
	}
	return driveFileToFolder(f, ""), nil
}

func (c GoogleDriveFolderClient) FindFolder(ctx context.Context, parentID, name string) ([]DriveFolder, error) {
	q := fmt.Sprintf("name = '%s' and '%s' in parents and mimeType = '%s' and trashed = false", escapeDriveQuery(name), escapeDriveQuery(parentID), DriveFolderMimeType)
	call := c.service.Files.List().Q(q).Fields("nextPageToken,files(id,name,parents,createdTime)").OrderBy("createdTime asc,name asc").PageSize(100).SupportsAllDrives(true).IncludeItemsFromAllDrives(true).Context(ctx)
	out := []DriveFolder{}
	for {
		res, err := call.Do()
		if err != nil {
			return nil, err
		}
		for _, f := range res.Files {
			out = append(out, driveFileToFolder(f, parentID))
		}
		if res.NextPageToken == "" {
			break
		}
		call.PageToken(res.NextPageToken)
	}
	return out, nil
}

func (c GoogleDriveFolderClient) CreateFolder(ctx context.Context, parentID, name string) (DriveFolder, error) {
	f, err := c.service.Files.Create(&drive.File{Name: name, MimeType: DriveFolderMimeType, Parents: []string{parentID}}).Fields("id,name,parents,createdTime").SupportsAllDrives(true).Context(ctx).Do()
	if err != nil {
		return DriveFolder{}, err
	}
	return driveFileToFolder(f, parentID), nil
}

func driveFileToFolder(f *drive.File, fallbackParent string) DriveFolder {
	parent := fallbackParent
	if len(f.Parents) > 0 {
		parent = f.Parents[0]
	}
	created, _ := time.Parse(time.RFC3339, f.CreatedTime)
	return DriveFolder{ID: f.Id, Name: f.Name, ParentID: parent, CreatedTime: created}
}

func escapeDriveQuery(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\\`, `\\\\`), `'`, `\\'`)
}
