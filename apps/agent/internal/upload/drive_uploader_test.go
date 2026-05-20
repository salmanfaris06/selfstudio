package upload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recordingDriveFileClient struct {
	folders       map[string]string
	existingFiles map[string]DriveUploadResult
	uploads       []struct{ folderID, fileName, localPath string }
	err           error
}

func (c *recordingDriveFileClient) FindFolder(ctx context.Context, parentID, name string) ([]DriveFolder, error) {
	if c.folders == nil {
		c.folders = map[string]string{}
	}
	if id := c.folders[parentID+":"+name]; id != "" {
		return []DriveFolder{{ID: id, Name: name, ParentID: parentID}}, nil
	}
	return nil, nil
}
func (c *recordingDriveFileClient) CreateFolder(ctx context.Context, parentID, name string) (DriveFolder, error) {
	if c.folders == nil {
		c.folders = map[string]string{}
	}
	id := "folder_" + name
	c.folders[parentID+":"+name] = id
	return DriveFolder{ID: id, Name: name, ParentID: parentID}, nil
}
func (c *recordingDriveFileClient) GetFolder(ctx context.Context, folderID string) (DriveFolder, error) {
	return DriveFolder{ID: folderID}, nil
}
func (c *recordingDriveFileClient) FindFile(ctx context.Context, folderID, fileName string) ([]DriveUploadResult, error) {
	if c.existingFiles == nil {
		return nil, nil
	}
	if res, ok := c.existingFiles[folderID+":"+fileName]; ok {
		return []DriveUploadResult{res}, nil
	}
	return nil, nil
}
func (c *recordingDriveFileClient) UploadFile(ctx context.Context, folderID, fileName, localPath string) (DriveUploadResult, error) {
	c.uploads = append(c.uploads, struct{ folderID, fileName, localPath string }{folderID, fileName, localPath})
	if c.err != nil {
		return DriveUploadResult{}, c.err
	}
	return DriveUploadResult{DriveFileID: "file_" + folderID + "_" + fileName, DriveFolderID: folderID, RemoteETag: "etag"}, nil
}

func TestDriveUploaderUploadsWithFolderIDAndSafeFileName(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "My Original @ RAW.JPG")
	if err := os.WriteFile(p, []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &recordingDriveFileClient{}
	u := DriveUploader{Client: client}
	res, err := u.UploadFile(context.Background(), "folder_original", "My Original @ RAW.JPG", p)
	if err != nil {
		t.Fatal(err)
	}
	if res.DriveFileID == "" || res.DriveFolderID != "folder_original" {
		t.Fatalf("bad result: %#v", res)
	}
	if len(client.uploads) != 1 || client.uploads[0].folderID != "folder_original" || client.uploads[0].fileName != "my-original-raw.jpg" {
		t.Fatalf("bad upload: %#v", client.uploads)
	}
}

func TestDriveUploaderReusesExistingExactParentNameBeforeCreate(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(p, []byte("jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &recordingDriveFileClient{existingFiles: map[string]DriveUploadResult{
		"folder_original:photo.jpg": {DriveFileID: "existing_file", DriveFolderID: "folder_original", RemoteETag: "old"},
	}}
	u := DriveUploader{Client: client}
	res, err := u.UploadFile(context.Background(), "folder_original", "photo.jpg", p)
	if err != nil {
		t.Fatal(err)
	}
	if res.DriveFileID != "existing_file" || !res.AlreadyExisted {
		t.Fatalf("expected existing file reuse, got %#v", res)
	}
	if len(client.uploads) != 0 {
		t.Fatalf("expected no duplicate create, got %#v", client.uploads)
	}
}

func TestMapDriveUploadErrorIsSafe(t *testing.T) {
	code, action := MapUploadError(errors.New("oauth private_key token raw google body"))
	if code != ErrorDriveUploadUnauthorized || action != ActionFixCredentials {
		t.Fatalf("got %s/%s", code, action)
	}
	if strings.Contains(code+action, "private_key") || strings.Contains(code+action, "token") {
		t.Fatal("secret leaked")
	}
}

func TestDriveRetryReusesExistingRemoteFileAfterPartialCreate(t *testing.T) {
	w, sessionID, _, _ := testWorker(t, nil, true)
	local := mustTempFile(t)
	client := &recordingDriveFileClient{existingFiles: map[string]DriveUploadResult{
		"drive-folder:file.jpg": {DriveFileID: "remote-created-before-failure", DriveFolderID: "drive-folder", RemoteETag: "etag-existing"},
	}}
	w.DriveUploader = DriveUploader{Client: client}
	w.Uploader = nil
	now := time.Now().UTC()
	job := FileUploadJob{JobID: JobID(sessionID, "drive-photo", AssetKindOriginal), SessionID: sessionID, StationID: "station_1", PhotoID: "drive-photo", AssetKind: AssetKindOriginal, LocalPath: local, DriveFolderID: "drive-folder", DedupeKey: JobID(sessionID, "drive-photo", AssetKindOriginal), Status: JobStatusFailed, AttemptCount: 1, MaxAttempts: MaxAutoUploadAttempts, LastErrorCode: ErrorDriveUploadFailed, LastErrorAction: ActionRetryDriveUpload, CreatedAt: now, UpdatedAt: now}
	if err := w.Jobs.Upsert(job); err != nil {
		t.Fatal(err)
	}
	res, err := w.RetryJob(context.Background(), job.JobID, true)
	if err != nil || !res.Accepted {
		t.Fatalf("retry should be accepted: %+v %v", res, err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, _ := w.Jobs.Get(job.JobID)
		if got.Status == JobStatusUploaded {
			if got.DriveFileID != "remote-created-before-failure" || got.RemoteIdentity != "remote-created-before-failure" {
				t.Fatalf("expected existing Drive ID persisted, got %+v", got)
			}
			if len(client.uploads) != 0 {
				t.Fatalf("expected no duplicate Drive create, got %#v", client.uploads)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("retry did not complete")
}
