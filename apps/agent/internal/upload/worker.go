package upload

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
)

type Worker struct {
	Sessions      *sessions.Store
	Photos        *photos.Store
	Targets       *Store
	Jobs          *JobsStore
	Persistence   JobsPersistence
	Uploader      Uploader
	DriveUploader DriveFileUploader
	FolderClient  DriveFolderClient
	Events        chan FileUploadJob
	guard         *uploadGuard
	RecoveryMu    *sync.Mutex
}

type StartResult struct {
	SessionStatus string          `json:"session_upload_status"`
	Jobs          []FileUploadJob `json:"jobs"`
}

func (w *Worker) StartSession(ctx context.Context, sessionID string) (StartResult, error) {
	if w == nil {
		return StartResult{}, errors.New("upload worker not configured")
	}
	w.ensureGuard()
	s, err := w.Sessions.Get(sessionID)
	if err != nil {
		return StartResult{}, err
	}
	if s.Status != sessions.StatusLocked {
		return StartResult{SessionStatus: SessionUploadPendingLocalCompletion}, SafeUploadError{Code: ErrorUploadPendingLocalCompletion, Action: ActionWaitLocalCompletion}
	}
	t, ok := w.Targets.Get(sessionID)
	if !ok || t.Status != StatusReady {
		return StartResult{SessionStatus: SessionUploadTargetPending}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	jobs, complete, err := w.discover(ctx, s, t)
	if err != nil {
		return StartResult{}, err
	}
	if err := w.Persistence.Save(w.Jobs); err != nil {
		return StartResult{}, SafeUploadError{Code: ErrorUploadStateSaveFailed, Action: ActionRetryCloudUpload}
	}
	if !complete {
		return StartResult{SessionStatus: SessionUploadPendingLocalCompletion, Jobs: jobs}, SafeUploadError{Code: ErrorUploadPendingLocalCompletion, Action: ActionWaitLocalCompletion}
	}
	go w.run(ctx, jobs)
	return StartResult{SessionStatus: AggregateUploadStatus(t, w.Jobs.ListBySession(sessionID)), Jobs: jobs}, nil
}

func (w *Worker) discover(ctx context.Context, s sessions.Session, t SessionCloudTarget) ([]FileUploadJob, bool, error) {
	if w.Photos == nil || w.Jobs == nil {
		return nil, false, errors.New("upload worker not configured")
	}
	now := time.Now().UTC()
	complete := true
	out := []FileUploadJob{}
	for _, p := range w.Photos.ListBySession(s.SessionID, 0) {
		if p.OriginalSaveStatus == photos.OriginalStatusSaved && readable(p.LocalOriginalPath) {
			j, err := w.ensureJob(ctx, t, p.PhotoID, AssetKindOriginal, p.LocalOriginalPath, now, JobStatusPending, "", "")
			if err != nil {
				return nil, false, err
			}
			out = append(out, j)
		} else if p.OriginalSaveStatus == photos.OriginalStatusPending || p.OriginalSaveStatus == photos.OriginalStatusSaving {
			complete = false
		}
		switch p.GradedProcessingStatus {
		case photos.GradedStatusProcessed:
			if readable(p.LocalGradedPath) {
				j, err := w.ensureJob(ctx, t, p.PhotoID, AssetKindGraded, p.LocalGradedPath, now, JobStatusPending, "", "")
				if err != nil {
					return nil, false, err
				}
				out = append(out, j)
			} else {
				j, _ := w.ensureJob(ctx, t, p.PhotoID, AssetKindGraded, p.LocalGradedPath, now, JobStatusFailed, ErrorUploadLocalFileMissing, ActionCheckLocalOutput)
				out = append(out, j)
			}
		case photos.GradedStatusFailed, photos.GradedStatusNotEligible:
			j, _ := w.ensureJob(ctx, t, p.PhotoID, AssetKindGraded, p.LocalGradedPath, now, JobStatusNotEligible, "", "")
			out = append(out, j)
		case photos.GradedStatusPending, photos.GradedStatusProcessing, "":
			complete = false
		}
	}
	return out, complete, nil
}

func (w *Worker) ensureJob(ctx context.Context, t SessionCloudTarget, photoID, kind, localPath string, now time.Time, status, code, action string) (FileUploadJob, error) {
	id := JobID(t.SessionID, photoID, kind)
	if existing, ok := w.Jobs.Get(id); ok {
		return existing, nil
	}
	objectKey := ""
	if status != JobStatusNotEligible {
		k, err := BuildFileObjectKey(t, kind, localPath)
		if err != nil {
			return FileUploadJob{}, err
		}
		objectKey = k
	}
	driveFolderID := ""
	if status != JobStatusNotEligible && t.DriveSessionFolderID != "" {
		folder, err := w.resolveAssetFolder(ctx, t.DriveSessionFolderID, kind)
		if err != nil {
			return FileUploadJob{}, err
		}
		driveFolderID = folder
	}
	j := FileUploadJob{JobID: id, SessionID: t.SessionID, StationID: t.StationID, PhotoID: photoID, AssetKind: kind, LocalPath: localPath, BucketName: t.BucketName, ObjectKey: objectKey, DriveFolderID: driveFolderID, DedupeKey: id, Status: status, MaxAttempts: MaxAutoUploadAttempts, LastErrorCode: code, LastErrorAction: action, CreatedAt: now, UpdatedAt: now}
	return j, w.Jobs.Upsert(j)
}

func (w *Worker) run(ctx context.Context, jobs []FileUploadJob) {
	for _, j := range jobs {
		if j.Status != JobStatusPending {
			continue
		}
		w.uploadOne(ctx, j)
	}
}
func (w *Worker) uploadOne(ctx context.Context, j FileUploadJob) {
	w.ensureGuard()
	if !w.guard.begin(j.JobID) {
		return
	}
	defer w.guard.end(j.JobID)
	w.uploadOneGuarded(ctx, j)
}

func (w *Worker) uploadOneGuarded(ctx context.Context, j FileUploadJob) {
	now := time.Now().UTC()
	if j.Status != JobStatusRetrying {
		j.Status = JobStatusUploading
	}
	j.AttemptCount++
	j.LastAttemptAt = &now
	j.MaxAttempts = MaxAutoUploadAttempts
	j.UpdatedAt = now
	if err := w.Jobs.Upsert(j); err != nil {
		return
	}
	if err := w.Persistence.Save(w.Jobs); err != nil {
		return
	}
	w.publish(j)
	if !readable(j.LocalPath) {
		j.Status = JobStatusFailed
		j.LastErrorCode = ErrorUploadLocalFileMissing
		j.LastErrorAction = ActionCheckLocalOutput
	} else {
		fileName := filepathBase(j.LocalPath)
		res, err := w.uploadDriveOrLegacy(ctx, j, fileName)
		if err != nil {
			j.Status = JobStatusFailed
			j.LastErrorCode, j.LastErrorAction = MapUploadError(err)
			if CanAutoRetry(j) {
				j = ScheduleRetry(j, time.Now().UTC())
			}
		} else {
			uploadedAt := time.Now().UTC()
			j.Status = JobStatusUploaded
			j.DriveFileID = res.DriveFileID
			j.DriveFolderID = res.DriveFolderID
			if res.DriveFileID != "" {
				j.RemoteIdentity = res.DriveFileID
			} else {
				j.RemoteIdentity = res.RemoteIdentity
			}
			j.RemoteGeneration = res.RemoteGeneration
			j.RemoteMetageneration = res.RemoteMetageneration
			j.RemoteETag = res.RemoteETag
			j.UploadedAt = &uploadedAt
			j.LastErrorCode = ""
			j.LastErrorAction = ""
			j.NextRetryAt = nil
			j.RetryAfterSeconds = 0
		}
	}
	j.UpdatedAt = time.Now().UTC()
	if err := w.Jobs.Upsert(j); err != nil {
		return
	}
	if err := w.Persistence.Save(w.Jobs); err != nil {
		return
	}
	w.publish(j)
}
func (w *Worker) resolveAssetFolder(ctx context.Context, sessionFolderID, kind string) (string, error) {
	if kind != AssetKindOriginal && kind != AssetKindGraded {
		return "", errors.New("invalid asset kind")
	}
	if w.FolderClient == nil {
		return sessionFolderID, nil
	}
	found, err := w.FolderClient.FindFolder(ctx, sessionFolderID, kind)
	if err != nil {
		return "", MapDriveUploadError(err)
	}
	if len(found) > 0 {
		return found[0].ID, nil
	}
	created, err := w.FolderClient.CreateFolder(ctx, sessionFolderID, kind)
	if err != nil {
		return "", MapDriveUploadError(err)
	}
	return created.ID, nil
}

func (w *Worker) uploadDriveOrLegacy(ctx context.Context, j FileUploadJob, fileName string) (UploadResult, error) {
	if w.DriveUploader != nil || j.DriveFolderID != "" {
		if w.DriveUploader == nil {
			return UploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
		}
		res, err := w.DriveUploader.UploadFile(ctx, j.DriveFolderID, fileName, j.LocalPath)
		return UploadResult{RemoteIdentity: res.DriveFileID, RemoteETag: res.RemoteETag, AlreadyExisted: res.AlreadyExisted, DriveFileID: res.DriveFileID, DriveFolderID: res.DriveFolderID}, err
	}
	if w.Uploader == nil {
		return UploadResult{}, SafeUploadError{Code: ErrorCloudTargetNotReady, Action: ActionResolveCloudTarget}
	}
	return w.Uploader.Upload(ctx, j.BucketName, j.ObjectKey, j.LocalPath)
}

func (w *Worker) publish(j FileUploadJob) {
	if w.Events != nil {
		select {
		case w.Events <- j:
		default:
		}
	}
}
func (w *Worker) ensureGuard() {
	if w.guard == nil {
		w.guard = &uploadGuard{}
	}
}

func readable(p string) bool {
	if p == "" {
		return false
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
