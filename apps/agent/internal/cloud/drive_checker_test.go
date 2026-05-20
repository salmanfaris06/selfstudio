package cloud

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/api/drive/v3"
)

type recordingDriveProbeClient struct {
	gotParentID string
	gotName     string
}

func (c *recordingDriveProbeClient) GetRootFolder(ctx context.Context, folderID string) (*drive.File, error) {
	return &drive.File{Id: folderID, Name: "Delivery", MimeType: driveFolderMimeType}, nil
}

func (c *recordingDriveProbeClient) CreateProbeFolder(ctx context.Context, parentID, name string) (*drive.File, error) {
	c.gotParentID = parentID
	c.gotName = name
	return &drive.File{Id: "probe-folder-id", Name: name, MimeType: driveFolderMimeType}, nil
}

func (c *recordingDriveProbeClient) DeleteProbe(ctx context.Context, fileID string) error { return nil }

func TestGoogleDriveCheckWithClientRequiresWritableProbe(t *testing.T) {
	probe := &recordingDriveProbeClient{}
	res := CheckGoogleDriveWithClient(context.Background(), DefaultSettingsWithDriveCredentialsForTest(), probe)
	if res.Status != StatusAuthorized {
		t.Fatalf("status = %+v", res)
	}
	if probe.gotParentID != "drive-root-123" {
		t.Fatalf("probe parent = %q", probe.gotParentID)
	}
	if probe.gotName == "" {
		t.Fatal("expected probe folder name")
	}
}

func TestGoogleDriveCheckWithClientMapsProbePermissionFailureToDriveFolderAction(t *testing.T) {
	probe := &failingDriveProbeClient{createErr: errDriveAPIPermissionForTest()}
	res := CheckGoogleDriveWithClient(context.Background(), DefaultSettingsWithDriveCredentialsForTest(), probe)
	if res.Status != StatusFailed {
		t.Fatalf("status = %+v", res)
	}
	if res.ErrorAction != ActionFixDriveFolder {
		t.Fatalf("action = %q, want %q", res.ErrorAction, ActionFixDriveFolder)
	}
}

type failingDriveProbeClient struct{ createErr error }

func (c *failingDriveProbeClient) GetRootFolder(ctx context.Context, folderID string) (*drive.File, error) {
	return &drive.File{Id: folderID, Name: "Delivery", MimeType: driveFolderMimeType}, nil
}

func (c *failingDriveProbeClient) CreateProbeFolder(ctx context.Context, parentID, name string) (*drive.File, error) {
	return nil, c.createErr
}

func (c *failingDriveProbeClient) DeleteProbe(ctx context.Context, fileID string) error { return nil }

func DefaultSettingsWithDriveCredentialsForTest() Settings {
	s := DefaultSettings()
	s.DriveRootFolderID = "drive-root-123"
	s.ServiceAccountJSON = `{"type":"service_account"}`
	return s
}

func errDriveAPIPermissionForTest() error { return errors.New("403 insufficient permissions") }
