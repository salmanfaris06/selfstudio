package upload

import (
	"context"
	"errors"
	"testing"
	"time"

	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/sessions"
)

type fakeCloudStore struct {
	settings cloud.Settings
	err      error
}

func (f fakeCloudStore) LoadOrDefault() (cloud.Settings, error) { return f.settings, f.err }

type fakeTargetPersistence struct {
	err   error
	saves int
}

func (f *fakeTargetPersistence) Save(*Store) error { f.saves++; return f.err }

type fakeDriveFolders struct {
	folders map[string][]DriveFolder
	creates int
	findErr error
}

func newFakeDriveFolders() *fakeDriveFolders {
	return &fakeDriveFolders{folders: map[string][]DriveFolder{}}
}
func key(parent, name string) string { return parent + "/" + name }
func (f *fakeDriveFolders) FindFolder(ctx context.Context, parentID, name string) ([]DriveFolder, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if d, ok := f.folders[key(parentID, name)]; ok {
		return d, nil
	}
	return nil, nil
}
func (f *fakeDriveFolders) CreateFolder(ctx context.Context, parentID, name string) (DriveFolder, error) {
	f.creates++
	d := DriveFolder{ID: "folder-" + name, Name: name, ParentID: parentID}
	f.folders[key(parentID, name)] = append(f.folders[key(parentID, name)], d)
	return d, nil
}
func (f *fakeDriveFolders) GetFolder(ctx context.Context, folderID string) (DriveFolder, error) {
	return DriveFolder{ID: folderID}, nil
}

func TestResolverSuccessIdempotent(t *testing.T) {
	store := NewStore()
	p := &fakeTargetPersistence{}
	drive := newFakeDriveFolders()
	r := Resolver{CloudStore: fakeCloudStore{settings: cloud.Settings{Provider: cloud.ProviderGoogleDrive, DriveRootFolderID: "drive-root-123", ServiceAccountJSON: "{}", ConnectionStatus: cloud.StatusAuthorized}}, Targets: store, Persistence: p, DriveFolders: drive, Now: func() time.Time { return time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC) }}
	s := sessions.Session{SessionID: "s1", StationID: "station-1", CustomerName: "Customer", OrderNumber: "Order", Status: sessions.StatusLocked, StartedAt: time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)}
	first, err := r.ResolveForSession(context.Background(), s)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	second, err := r.ResolveForSession(context.Background(), s)
	if err != nil {
		t.Fatalf("resolve retry: %v", err)
	}
	if first.DriveSessionFolderID == "" || first.RemoteIdentity != first.DriveSessionFolderID || first.RemoteIdentity != second.RemoteIdentity || second.AttemptCount != 1 {
		t.Fatalf("not idempotent: %#v %#v", first, second)
	}
	if len(first.DriveFolderChain) != 7 || drive.creates != 7 {
		t.Fatalf("expected folder chain created once, chain=%#v creates=%d", first.DriveFolderChain, drive.creates)
	}
	if p.saves != 1 {
		t.Fatalf("expected one save, got %d", p.saves)
	}
}

func TestResolverUnconfiguredPersistsFailedRetrySameRecord(t *testing.T) {
	store := NewStore()
	p := &fakeTargetPersistence{}
	r := Resolver{CloudStore: fakeCloudStore{settings: cloud.DefaultSettings()}, Targets: store, Persistence: p}
	s := sessions.Session{SessionID: "s1", StationID: "station-1", StartedAt: time.Now().UTC()}
	_, _ = r.ResolveForSession(context.Background(), s)
	target, _ := store.Get("s1")
	if target.Status != StatusFailed || target.LastErrorCode != ErrorNotConfigured || target.AttemptCount != 1 {
		t.Fatalf("failed target mismatch: %#v", target)
	}
	_, _ = r.ResolveForSession(context.Background(), s)
	target, _ = store.Get("s1")
	if target.AttemptCount != 2 {
		t.Fatalf("expected retry update same record: %#v", target)
	}
}

func TestResolverPreservesReadyDriveIdentityOnTransientFailure(t *testing.T) {
	store := NewStore()
	p := &fakeTargetPersistence{}
	drive := newFakeDriveFolders()
	r := Resolver{CloudStore: fakeCloudStore{settings: cloud.Settings{Provider: cloud.ProviderGoogleDrive, DriveRootFolderID: "drive-root-123", ServiceAccountJSON: "{}", ConnectionStatus: cloud.StatusAuthorized}}, Targets: store, Persistence: p, DriveFolders: drive}
	s := sessions.Session{SessionID: "s1", StationID: "station-1", CustomerName: "Customer", OrderNumber: "Order", Status: sessions.StatusLocked, StartedAt: time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)}
	ready, err := r.ResolveForSession(context.Background(), s)
	if err != nil {
		t.Fatalf("initial resolve: %v", err)
	}
	legacy := ready
	legacy.RemoteIdentity = "legacy-path"
	if err := store.Upsert(legacy); err != nil {
		t.Fatal(err)
	}
	drive.findErr = errors.New("transient list failure")
	got, err := r.ResolveForSession(context.Background(), s)
	if err != nil {
		t.Fatalf("transient failure should preserve ready target without surfacing failure: %v", err)
	}
	if got.Status != StatusReady || got.DriveSessionFolderID != ready.DriveSessionFolderID || got.RemoteIdentity != ready.RemoteIdentity {
		t.Fatalf("ready identity not preserved/repaired: %#v want %#v", got, ready)
	}
	saved, _ := store.Get("s1")
	if saved.Status != StatusReady || saved.LastErrorCode != "" || saved.DriveSessionFolderID != ready.DriveSessionFolderID {
		t.Fatalf("store overwritten by transient failure: %#v", saved)
	}
}

func TestResolveDriveFolderChainChoosesDeterministicDuplicate(t *testing.T) {
	drive := newFakeDriveFolders()
	older := DriveFolder{ID: "older", Name: "2026", ParentID: "root", CreatedTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	newer := DriveFolder{ID: "newer", Name: "2026", ParentID: "root", CreatedTime: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
	drive.folders[key("root", "2026")] = []DriveFolder{newer, older}
	chain, _, err := resolveDriveFolderChain(context.Background(), drive, "root", "2026/05/18/Customer/Order/station-1/s1")
	if err != nil {
		t.Fatalf("resolve chain: %v", err)
	}
	if chain[0].FolderID != "older" {
		t.Fatalf("expected oldest duplicate reused deterministically, got %#v", chain[0])
	}
}
