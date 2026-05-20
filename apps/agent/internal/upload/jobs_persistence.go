package upload

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const jobsStateVersion = 1

type jobsStateFile struct {
	Version int             `json:"version"`
	SavedAt time.Time       `json:"saved_at"`
	Jobs    []FileUploadJob `json:"jobs"`
}

type JobsPersistence struct{ path string }

var jobsStateSaveMu sync.Mutex

func NewJobsPersistence(localDataDir string) JobsPersistence {
	return JobsPersistence{path: filepath.Join(localDataDir, "state", "upload_jobs.json")}
}
func (p JobsPersistence) Path() string { return p.path }
func (p JobsPersistence) LoadOrDefault() (*JobsStore, error) {
	if _, err := os.Stat(p.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewJobsStore(), nil
		}
		return nil, err
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, err
	}
	var f jobsStateFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Version != jobsStateVersion {
		return nil, errors.New("invalid upload jobs state version")
	}
	return NewJobsStoreFromRecords(f.Jobs)
}
func (p JobsPersistence) Save(store *JobsStore) error {
	jobsStateSaveMu.Lock()
	defer jobsStateSaveMu.Unlock()
	if store == nil {
		return errors.New("upload jobs store is nil")
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(jobsStateFile{Version: jobsStateVersion, SavedAt: time.Now().UTC(), Jobs: store.List()}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".upload-jobs-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	_ = tmp.Chmod(0o600)
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(p.path)
	}
	return os.Rename(name, p.path)
}

type JobsStore struct {
	jobs map[string]FileUploadJob
	mu   sync.RWMutex
}

func NewJobsStore() *JobsStore { return &JobsStore{jobs: map[string]FileUploadJob{}} }
func NewJobsStoreFromRecords(records []FileUploadJob) (*JobsStore, error) {
	s := NewJobsStore()
	seen := map[string]bool{}
	for _, r := range records {
		if err := validateJob(r); err != nil {
			return nil, err
		}
		if seen[r.JobID] {
			return nil, errors.New("duplicate upload job")
		}
		seen[r.JobID] = true
		s.jobs[r.JobID] = r
	}
	return s, nil
}
func (s *JobsStore) Get(jobID string) (FileUploadJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[jobID]
	return j, ok
}
func (s *JobsStore) GetByPhotoAsset(sessionID, photoID, assetKind string) (FileUploadJob, bool) {
	return s.Get(JobID(sessionID, photoID, assetKind))
}
func (s *JobsStore) Upsert(j FileUploadJob) error {
	if err := validateJob(j); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.JobID] = j
	return nil
}
func (s *JobsStore) List() []FileUploadJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FileUploadJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}
func (s *JobsStore) ListBySession(sessionID string) []FileUploadJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []FileUploadJob{}
	for _, j := range s.jobs {
		if j.SessionID == sessionID {
			out = append(out, j)
		}
	}
	return out
}

func AggregateUploadStatus(target SessionCloudTarget, jobs []FileUploadJob) string {
	if target.SessionID == "" {
		return SessionUploadNotConfigured
	}
	if target.Status != StatusReady {
		return SessionUploadTargetPending
	}
	if len(jobs) == 0 {
		return SessionUploadPending
	}
	uploading, pending, failed, uploaded, eligible := 0, 0, 0, 0, 0
	for _, j := range jobs {
		if j.Status != JobStatusNotEligible {
			eligible++
		}
		switch j.Status {
		case JobStatusUploading, JobStatusRetrying:
			uploading++
		case JobStatusPending, JobStatusRetryScheduled:
			pending++
		case JobStatusFailed:
			failed++
		case JobStatusUploaded:
			uploaded++
		}
	}
	if uploading > 0 {
		return SessionUploadUploading
	}
	if failed > 0 && uploaded > 0 {
		return SessionUploadPartialFailed
	}
	if failed > 0 && uploaded == 0 {
		return SessionUploadFailed
	}
	if pending > 0 {
		return SessionUploadPending
	}
	if eligible > 0 && uploaded == eligible {
		return SessionUploadUploaded
	}
	return SessionUploadPending
}
