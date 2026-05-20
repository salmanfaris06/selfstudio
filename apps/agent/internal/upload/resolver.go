package upload

import (
	"context"
	"sort"
	"strings"
	"time"

	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/sessions"
)

type CloudSettingsStore interface {
	LoadOrDefault() (cloud.Settings, error)
}
type TargetPersistence interface{ Save(*Store) error }

type Resolver struct {
	CloudStore      CloudSettingsStore
	Targets         *Store
	Persistence     TargetPersistence
	DriveFolders    DriveFolderClient
	NewDriveFolders func(context.Context, cloud.Settings) (DriveFolderClient, error)
	Now             func() time.Time
}

func (r Resolver) ResolveForSession(ctx context.Context, session sessions.Session) (SessionCloudTarget, error) {
	if r.Targets == nil {
		r.Targets = NewStore()
	}
	now := nowUTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}
	if existing, ok := r.Targets.Get(session.SessionID); ok && existing.Status == StatusReady && existing.DriveSessionFolderID != "" && existing.RemoteIdentity == existing.DriveSessionFolderID {
		return existing, nil
	}
	settings, err := r.CloudStore.LoadOrDefault()
	if err != nil {
		return r.fail(session, now, ErrorPrefixResolveFailed, ActionRetryCheck)
	}
	if settings.Provider == "" {
		settings.Provider = cloud.ProviderGoogleDrive
	}
	if settings.Provider != cloud.ProviderGoogleDrive || settings.DriveRootFolderID == "" {
		return r.fail(session, now, ErrorNotConfigured, ActionFixCredentials)
	}
	if !settings.CredentialsConfigured() {
		return r.fail(session, now, ErrorNotConfigured, ActionFixCredentials)
	}
	if settings.ConnectionStatus != cloud.StatusAuthorized {
		return r.fail(session, now, ErrorNotAuthorized, ActionRetryCheck)
	}
	if err := cloud.ValidateSettings(settings); err != nil {
		return r.fail(session, now, ErrorTargetRuleInvalid, ActionFixRules)
	}
	folderPath, _, err := BuildSessionDriveFolderPath(settings, session)
	if err != nil {
		return r.fail(session, now, ErrorTargetRuleInvalid, ActionFixRules)
	}
	client := r.DriveFolders
	if client == nil {
		factory := r.NewDriveFolders
		if factory == nil {
			factory = func(ctx context.Context, s cloud.Settings) (DriveFolderClient, error) {
				c, err := NewGoogleDriveFolderClient(ctx, s)
				if err != nil {
					return nil, err
				}
				return c, nil
			}
		}
		client, err = factory(ctx, settings)
		if err != nil {
			return r.fail(session, now, ErrorNotAuthorized, ActionFixCredentials)
		}
	}
	chain, finalID, err := resolveDriveFolderChain(ctx, client, settings.DriveRootFolderID, folderPath)
	if err != nil {
		if preserved, ok := r.preserveKnownReadyTarget(session, now); ok {
			return preserved, nil
		}
		return r.fail(session, now, ErrorPrefixResolveFailed, ActionRetryTarget)
	}
	t := r.baseTarget(session, now)
	t.DriveRootFolderID = settings.DriveRootFolderID
	t.DriveRootFolderName = settings.DriveRootFolderName
	t.DriveFolderPath = folderPath
	t.ObjectPrefix = folderPath
	t.BucketName = "google-drive"
	t.DriveFolderChain = chain
	t.DriveSessionFolderID = finalID
	t.RemoteIdentity = finalID
	t.Status = StatusReady
	t.LastErrorCode = ""
	t.LastErrorAction = ""
	t.ResolvedAt = &now
	if existing, ok := r.Targets.Get(session.SessionID); ok {
		t.AttemptCount = existing.AttemptCount + 1
		t.CreatedAt = existing.CreatedAt
	} else {
		t.AttemptCount = 1
	}
	if err := r.Targets.Upsert(t); err != nil {
		return t, err
	}
	if r.Persistence != nil {
		if err := r.Persistence.Save(r.Targets); err != nil {
			t.Status = StatusFailed
			t.LastErrorCode = ErrorTargetSaveFailed
			t.LastErrorAction = ActionRetryTarget
			return t, err
		}
	}
	return t, nil
}

func resolveDriveFolderChain(ctx context.Context, client DriveFolderClient, rootID, folderPath string) ([]DriveFolderRef, string, error) {
	parent := rootID
	parts := strings.Split(folderPath, "/")
	chain := make([]DriveFolderRef, 0, len(parts))
	levels := []string{"yyyy", "mm", "dd", "customer", "order", "station", "session"}
	for i, name := range parts {
		found, err := client.FindFolder(ctx, parent, name)
		if err != nil {
			return nil, "", err
		}
		var folder DriveFolder
		if len(found) > 0 {
			folder = chooseDriveFolder(found)
		} else {
			folder, err = client.CreateFolder(ctx, parent, name)
			if err != nil {
				return nil, "", err
			}
		}
		level := "segment"
		if i < len(levels) {
			level = levels[i]
		}
		chain = append(chain, DriveFolderRef{Level: level, Name: name, FolderID: folder.ID, ParentID: parent})
		parent = folder.ID
	}
	return chain, parent, nil
}

func chooseDriveFolder(found []DriveFolder) DriveFolder {
	sorted := append([]DriveFolder(nil), found...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].CreatedTime.Equal(sorted[j].CreatedTime) {
			if sorted[i].CreatedTime.IsZero() {
				return false
			}
			if sorted[j].CreatedTime.IsZero() {
				return true
			}
			return sorted[i].CreatedTime.Before(sorted[j].CreatedTime)
		}
		return sorted[i].ID < sorted[j].ID
	})
	return sorted[0]
}

func (r Resolver) preserveKnownReadyTarget(session sessions.Session, now time.Time) (SessionCloudTarget, bool) {
	existing, ok := r.Targets.Get(session.SessionID)
	if !ok || existing.Status != StatusReady || existing.DriveSessionFolderID == "" {
		return SessionCloudTarget{}, false
	}
	existing.UpdatedAt = now
	existing.LastCheckedAt = &now
	existing.LastErrorCode = ""
	existing.LastErrorAction = ""
	existing.RemoteIdentity = existing.DriveSessionFolderID
	_ = r.Targets.Upsert(existing)
	if r.Persistence != nil {
		if err := r.Persistence.Save(r.Targets); err != nil {
			return existing, false
		}
	}
	return existing, true
}

func (r Resolver) fail(session sessions.Session, now time.Time, code, action string) (SessionCloudTarget, error) {
	t := r.baseTarget(session, now)
	if existing, ok := r.Targets.Get(session.SessionID); ok {
		t = existing
		t.UpdatedAt = now
		t.LastCheckedAt = &now
		t.AttemptCount = existing.AttemptCount + 1
	} else {
		t.AttemptCount = 1
	}
	t.Status = StatusFailed
	t.LastErrorCode = code
	t.LastErrorAction = action
	_ = r.Targets.Upsert(t)
	if r.Persistence != nil {
		if err := r.Persistence.Save(r.Targets); err != nil {
			t.LastErrorCode = ErrorTargetSaveFailed
			t.LastErrorAction = ActionRetryTarget
			return t, err
		}
	}
	return t, CloudTargetError{Code: code, Action: action}
}

func (r Resolver) baseTarget(session sessions.Session, now time.Time) SessionCloudTarget {
	return SessionCloudTarget{SessionID: session.SessionID, StationID: session.StationID, Status: StatusPending, CreatedAt: now, UpdatedAt: now, LastCheckedAt: &now}
}

type CloudTargetError struct{ Code, Action string }

func (e CloudTargetError) Error() string { return e.Code }
