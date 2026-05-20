package upload

import (
	"context"
	"sync"
	"time"
)

type RetryResult struct {
	SessionStatus string          `json:"session_upload_status"`
	Jobs          []FileUploadJob `json:"jobs"`
	Accepted      bool            `json:"accepted"`
	NoopReason    string          `json:"noop_reason,omitempty"`
}

type uploadGuard struct {
	mu      sync.Mutex
	running map[string]struct{}
}

func (g *uploadGuard) begin(jobID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running == nil {
		g.running = map[string]struct{}{}
	}
	if _, ok := g.running[jobID]; ok {
		return false
	}
	g.running[jobID] = struct{}{}
	return true
}
func (g *uploadGuard) end(jobID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.running, jobID)
}

func (w *Worker) RetryJob(ctx context.Context, jobID string, manual bool) (RetryResult, error) {
	if w.Jobs == nil {
		return RetryResult{}, SafeUploadError{Code: ErrorUploadJobNotFound, Action: ActionRetryCloudUpload}
	}
	j, ok := w.Jobs.Get(jobID)
	if !ok {
		return RetryResult{}, SafeUploadError{Code: ErrorUploadJobNotFound, Action: ActionRetryCloudUpload}
	}
	return w.retryJobs(ctx, []FileUploadJob{j}, manual, false)
}

func (w *Worker) ResumeRecoveredJob(ctx context.Context, jobID string) (RetryResult, error) {
	if w.Jobs == nil {
		return RetryResult{}, SafeUploadError{Code: ErrorUploadJobNotFound, Action: ActionRetryCloudUpload}
	}
	j, ok := w.Jobs.Get(jobID)
	if !ok {
		return RetryResult{}, SafeUploadError{Code: ErrorUploadJobNotFound, Action: ActionRetryCloudUpload}
	}
	return w.retryJobs(ctx, []FileUploadJob{j}, false, true)
}

func (w *Worker) RetrySession(ctx context.Context, sessionID string, manual bool) (RetryResult, error) {
	if w.Jobs == nil {
		return RetryResult{}, SafeUploadError{Code: ErrorUploadJobNotFound, Action: ActionRetryCloudUpload}
	}
	candidates := []FileUploadJob{}
	for _, j := range w.Jobs.ListBySession(sessionID) {
		if manual && (CanManualRetry(j) || j.Status == JobStatusUploaded || j.Status == JobStatusUploading || j.Status == JobStatusRetrying) {
			candidates = append(candidates, j)
		} else if !manual && CanAutoRetry(j) {
			candidates = append(candidates, j)
		}
	}
	if len(candidates) == 0 {
		return RetryResult{SessionStatus: w.sessionStatus(sessionID), Jobs: w.Jobs.ListBySession(sessionID), NoopReason: ErrorUploadNotRetryable}, SafeUploadError{Code: ErrorUploadNotRetryable, Action: ActionRetryCloudUpload}
	}
	return w.retryJobs(ctx, candidates, manual, false)
}

func (w *Worker) retryJobs(ctx context.Context, jobs []FileUploadJob, manual bool, allowPending bool) (RetryResult, error) {
	w.ensureGuard()
	accepted := []FileUploadJob{}
	for _, j := range jobs {
		switch j.Status {
		case JobStatusUploaded:
			continue
		case JobStatusUploading, JobStatusRetrying:
			continue
		}
		if manual {
			if !CanManualRetry(j) {
				continue
			}
		} else if !(CanAutoRetry(j) || (allowPending && j.Status == JobStatusPending)) {
			continue
		}
		if !w.guard.begin(j.JobID) {
			continue
		}
		now := time.Now().UTC()
		j.Status = JobStatusRetrying
		j.MaxAttempts = MaxAutoUploadAttempts
		j.NextRetryAt = nil
		j.RetryAfterSeconds = 0
		j.UpdatedAt = now
		if err := w.Jobs.Upsert(j); err != nil {
			w.guard.end(j.JobID)
			return RetryResult{}, err
		}
		if err := w.Persistence.Save(w.Jobs); err != nil {
			w.guard.end(j.JobID)
			return RetryResult{}, SafeUploadError{Code: ErrorUploadRetryStateSave, Action: ActionRetryCloudUpload}
		}
		accepted = append(accepted, j)
		w.publish(j)
		go func(job FileUploadJob) {
			defer w.guard.end(job.JobID)
			w.uploadOneGuarded(ctx, job)
		}(j)
	}
	if len(accepted) == 0 {
		reason := ErrorUploadNotRetryable
		action := ActionRetryCloudUpload
		for _, j := range jobs {
			if j.Status == JobStatusUploading || j.Status == JobStatusRetrying {
				reason = ErrorUploadAlreadyRunning
				break
			}
			if j.Status == JobStatusUploaded {
				reason = ErrorUploadAlreadyUploaded
				break
			}
			if j.LastErrorCode == ErrorUploadLocalFileMissing {
				reason = ErrorUploadLocalFileMissing
				action = ActionCheckLocalOutput
			}
		}
		res := RetryResult{SessionStatus: w.sessionStatus(jobs[0].SessionID), Jobs: w.Jobs.ListBySession(jobs[0].SessionID), Accepted: false, NoopReason: reason}
		if reason == ErrorUploadAlreadyRunning || reason == ErrorUploadAlreadyUploaded {
			return res, nil
		}
		return res, SafeUploadError{Code: reason, Action: action}
	}
	return RetryResult{SessionStatus: w.sessionStatus(accepted[0].SessionID), Jobs: accepted, Accepted: true}, nil
}

func (w *Worker) AutoRetryDue(ctx context.Context, now time.Time) {
	if w == nil || w.Jobs == nil {
		return
	}
	w.ensureGuard()
	if w.RecoveryMu != nil {
		w.RecoveryMu.Lock()
		defer w.RecoveryMu.Unlock()
	}
	for _, j := range w.Jobs.List() {
		if j.Status == JobStatusFailed && CanAutoRetry(j) {
			scheduled := ScheduleRetry(j, now)
			if w.Jobs.Upsert(scheduled) == nil && w.Persistence.Save(w.Jobs) == nil {
				w.publish(scheduled)
			}
			continue
		}
		if j.Status == JobStatusRetryScheduled && j.NextRetryAt != nil && !j.NextRetryAt.After(now) && CanAutoRetry(j) {
			_, _ = w.RetryJob(ctx, j.JobID, false)
		}
	}
}

func (w *Worker) sessionStatus(sessionID string) string {
	target := SessionCloudTarget{SessionID: sessionID}
	if w.Targets != nil {
		if t, ok := w.Targets.Get(sessionID); ok {
			target = t
		}
	}
	if w.Jobs == nil {
		return AggregateUploadStatus(target, nil)
	}
	return AggregateUploadStatus(target, w.Jobs.ListBySession(sessionID))
}
