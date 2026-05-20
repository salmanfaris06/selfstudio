package upload

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPersistenceDefaultSaveLoadUniqueSession(t *testing.T) {
	p := NewPersistence(t.TempDir())
	store, err := p.LoadOrDefault()
	if err != nil {
		t.Fatalf("default load: %v", err)
	}
	if len(store.List()) != 0 {
		t.Fatalf("expected empty store")
	}
	now := time.Now().UTC()
	target := SessionCloudTarget{SessionID: "s1", StationID: "station-1", BucketName: "bucket", ObjectPrefix: "root/2026/05/19/c/o/station-1/s1", RemoteIdentity: "gcs://bucket/root/2026/05/19/c/o/station-1/s1", Status: StatusReady, AttemptCount: 1, CreatedAt: now, UpdatedAt: now}
	if err := store.Upsert(target); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := p.Save(store); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := p.LoadOrDefault()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := loaded.Get("s1")
	if !ok || got.RemoteIdentity != target.RemoteIdentity {
		t.Fatalf("loaded target mismatch: %#v", got)
	}
	if filepath.Base(p.Path()) != "upload_targets.json" {
		t.Fatalf("unexpected path: %s", p.Path())
	}
}

func TestStoreRejectsDuplicateRecords(t *testing.T) {
	now := time.Now().UTC()
	records := []SessionCloudTarget{{SessionID: "s1", StationID: "st", Status: StatusPending, CreatedAt: now, UpdatedAt: now}, {SessionID: "s1", StationID: "st", Status: StatusPending, CreatedAt: now, UpdatedAt: now}}
	if _, err := NewStoreFromRecords(records); err == nil {
		t.Fatalf("expected duplicate error")
	}
}
