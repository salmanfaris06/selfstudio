package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/quarantine"
)

var runtimeStateSaveMu sync.Mutex

func saveRuntimeStateTransactional(photoPersistence photos.Persistence, photoStore *photos.Store, quarantinePersistence quarantine.Persistence, quarantineStore *quarantine.Store) error {
	return saveRuntimeStateTransactionalWithHook(photoPersistence, photoStore, quarantinePersistence, quarantineStore, nil)
}

func saveRuntimeStateTransactionalWithHook(photoPersistence photos.Persistence, photoStore *photos.Store, quarantinePersistence quarantine.Persistence, quarantineStore *quarantine.Store, beforeQuarantineSave func() error) error {
	runtimeStateSaveMu.Lock()
	defer runtimeStateSaveMu.Unlock()

	photoPath := photoPersistence.Path()
	quarantinePath := quarantinePersistence.Path()
	backupDir, err := os.MkdirTemp(filepath.Dir(photoPath), ".runtime-state-rollback-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(backupDir)

	photoBackup := filepath.Join(backupDir, "photos.json")
	quarantineBackup := filepath.Join(backupDir, "quarantine.json")
	photoExisted, err := backupStateFile(photoPath, photoBackup)
	if err != nil {
		return err
	}
	quarantineExisted, err := backupStateFile(quarantinePath, quarantineBackup)
	if err != nil {
		return err
	}

	if err := photoPersistence.Save(photoStore); err != nil {
		return err
	}
	if beforeQuarantineSave != nil {
		if err := beforeQuarantineSave(); err != nil {
			if restoreErr := restoreStateFiles(photoPath, photoBackup, photoExisted, quarantinePath, quarantineBackup, quarantineExisted); restoreErr != nil {
				return fmt.Errorf("%w; rollback failed: %v", err, restoreErr)
			}
			return err
		}
	}
	if err := quarantinePersistence.Save(quarantineStore); err != nil {
		if restoreErr := restoreStateFiles(photoPath, photoBackup, photoExisted, quarantinePath, quarantineBackup, quarantineExisted); restoreErr != nil {
			return fmt.Errorf("%w; rollback failed: %v", err, restoreErr)
		}
		return err
	}
	return nil
}

func backupStateFile(source string, backup string) (bool, error) {
	info, err := os.Stat(source)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("state path is directory: %s", source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func restoreStateFiles(photoPath string, photoBackup string, photoExisted bool, quarantinePath string, quarantineBackup string, quarantineExisted bool) error {
	if err := restoreStateFile(photoPath, photoBackup, photoExisted); err != nil {
		return err
	}
	return restoreStateFile(quarantinePath, quarantineBackup, quarantineExisted)
}

func restoreStateFile(target string, backup string, existed bool) error {
	if !existed {
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}
