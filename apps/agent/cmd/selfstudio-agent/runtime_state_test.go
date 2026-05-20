package main

import (
	"os"
	"testing"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
)

func TestSaveRuntimeStateTransactionalRollsBackPhotoSaveWhenQuarantineSaveFails(t *testing.T) {
	localDataDir := t.TempDir()
	photoPersistence := photos.NewPersistence(localDataDir)
	quarantinePersistence := quarantine.NewPersistence(localDataDir)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	originalTime := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	originalPhoto := photoStore.Route("station_1", "session_original", "C:/camera/original.jpg", 10, originalTime, originalTime, originalTime)
	originalQuarantine := quarantineStore.Quarantine("station_1", "", "C:/camera/original-quarantine.jpg", 11, originalTime, originalTime, originalTime, quarantine.ReasonNoActiveSession)
	if err := photoPersistence.Save(photoStore); err != nil {
		t.Fatalf("save original photos: %v", err)
	}
	if err := quarantinePersistence.Save(quarantineStore); err != nil {
		t.Fatalf("save original quarantine: %v", err)
	}

	mutatedTime := originalTime.Add(time.Hour)
	photoStore.Route("station_1", "session_new", "C:/camera/new.jpg", 12, mutatedTime, mutatedTime, mutatedTime)
	quarantineStore.Quarantine("station_1", "", "C:/camera/new-quarantine.jpg", 13, mutatedTime, mutatedTime, mutatedTime, quarantine.ReasonNoActiveSession)
	if err := saveRuntimeStateTransactionalWithHook(photoPersistence, photoStore, quarantinePersistence, quarantineStore, func() error {
		return os.ErrPermission
	}); err == nil {
		t.Fatal("expected quarantine save failure")
	}
	reloadedPhotos, err := photoPersistence.LoadOrDefault()
	if err != nil {
		t.Fatalf("reload photos: %v", err)
	}
	reloadedQuarantine, err := quarantinePersistence.LoadOrDefault()
	if err != nil {
		t.Fatalf("reload quarantine: %v", err)
	}
	if _, ok := reloadedPhotos.GetBySourceIdentity("station_1", originalPhoto.SourcePath, originalPhoto.SourceSizeBytes); !ok {
		t.Fatalf("original photo missing after rollback")
	}
	if _, ok := reloadedPhotos.GetBySourceIdentity("station_1", "C:/camera/new.jpg", 12); ok {
		t.Fatalf("new photo was durably visible after failed second save")
	}
	if _, err := reloadedQuarantine.Get(originalQuarantine.QuarantineID); err != nil {
		t.Fatalf("original quarantine missing after rollback: %v", err)
	}
	if reloadedQuarantine.CountByStation("station_1") != 1 {
		t.Fatalf("unexpected durable quarantine count after rollback: %d", reloadedQuarantine.CountByStation("station_1"))
	}
}

func TestSaveRuntimeStateTransactionalSerializesConcurrentSaves(t *testing.T) {
	localDataDir := t.TempDir()
	photoPersistence := photos.NewPersistence(localDataDir)
	quarantinePersistence := quarantine.NewPersistence(localDataDir)
	photoStore := photos.NewStore()
	quarantineStore := quarantine.NewStore()
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	photoStore.Route("station_1", "session_1", "C:/camera/one.jpg", 10, now, now, now)

	if err := photoPersistence.Save(photoStore); err != nil {
		t.Fatalf("initial photo save: %v", err)
	}
	if err := quarantinePersistence.Save(quarantineStore); err != nil {
		t.Fatalf("initial quarantine save: %v", err)
	}

	errCh := make(chan error, 2)
	go func() {
		errCh <- saveRuntimeStateTransactional(photoPersistence, photoStore, quarantinePersistence, quarantineStore)
	}()
	go func() {
		errCh <- saveRuntimeStateTransactional(photoPersistence, photoStore, quarantinePersistence, quarantineStore)
	}()
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent save %d: %v", i, err)
		}
	}

	if _, err := photoPersistence.LoadOrDefault(); err != nil {
		t.Fatalf("reload photos: %v", err)
	}
	if _, err := quarantinePersistence.LoadOrDefault(); err != nil {
		t.Fatalf("reload quarantine: %v", err)
	}
}
