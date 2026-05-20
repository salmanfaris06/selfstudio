package processing

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
)

var ErrUnsafeOutputPath = errors.New("unsafe original output path")

// OriginalSaver owns local filesystem writes for Epic 5 original-first delivery.
// It updates original-save fields on photos.Photo directly because this story's
// lifecycle is per routed photo and must be persisted atomically with photo state.
type OriginalSaver struct {
	Photos   *photos.Store
	Sessions *sessions.Store
	Persist  func() error
	Now      func() time.Time
}

type SaveResult struct {
	Photo photos.Photo
	Err   error
}

func (s OriginalSaver) Save(photoID string) SaveResult {
	now := s.now()
	photo, ok := s.Photos.Get(photoID)
	if !ok {
		return SaveResult{Err: fmt.Errorf("photo not found: %s", photoID)}
	}
	if photo.OriginalSaveStatus == photos.OriginalStatusSaved {
		if validOriginal(photo) == nil {
			return SaveResult{Photo: photo}
		}
	}
	if s.Sessions == nil {
		return s.fail(photo, "session store unavailable", now)
	}
	session, err := s.Sessions.Get(photo.SessionID)
	if err != nil {
		return s.fail(photo, "session snapshot unavailable", now)
	}
	target, err := OriginalPath(session, photo)
	if err != nil {
		return s.fail(photo, err.Error(), now)
	}
	photo = s.Photos.MarkOriginalSaving(photo.PhotoID, target, now)
	if err := s.persist(); err != nil {
		return SaveResult{Photo: photo, Err: err}
	}
	if err := copyOriginal(photo.SourcePath, target, photo.SourceSizeBytes); err != nil {
		return s.fail(photo, safeError(err), s.now())
	}
	photo = s.Photos.MarkOriginalSaved(photo.PhotoID, target, s.now())
	if err := s.persist(); err != nil {
		failed := s.Photos.MarkOriginalFailed(photo.PhotoID, "original saved but state persistence failed; retry scan to reconcile", s.now())
		_ = s.persist()
		return SaveResult{Photo: failed, Err: err}
	}
	return SaveResult{Photo: photo}
}

func (s OriginalSaver) ReconcilePending() []SaveResult {
	results := []SaveResult{}
	for _, photo := range s.Photos.ListAll() {
		if photo.OriginalSaveStatus == photos.OriginalStatusSaved {
			if err := validOriginal(photo); err != nil {
				failed := s.Photos.MarkOriginalFailed(photo.PhotoID, actionableOriginalReason("saved original", err), s.now())
				if persistErr := s.persist(); persistErr != nil {
					results = append(results, SaveResult{Photo: failed, Err: fmt.Errorf("persist original recovery failure: %w", persistErr)})
				} else {
					results = append(results, SaveResult{Photo: failed, Err: err})
				}
			}
			continue
		}
		if photo.OriginalSaveStatus == "" || photo.OriginalSaveStatus == photos.OriginalStatusPending || photo.OriginalSaveStatus == photos.OriginalStatusSaving || photo.OriginalSaveStatus == photos.OriginalStatusFailed {
			results = append(results, s.Save(photo.PhotoID))
		}
	}
	return results
}

func (s OriginalSaver) fail(photo photos.Photo, message string, now time.Time) SaveResult {
	failed := s.Photos.MarkOriginalFailed(photo.PhotoID, message, now)
	if err := s.persist(); err != nil {
		return SaveResult{Photo: failed, Err: fmt.Errorf("persist original failure: %w", err)}
	}
	return SaveResult{Photo: failed, Err: errors.New(message)}
}

func (s OriginalSaver) persist() error {
	if s.Persist == nil {
		return nil
	}
	return s.Persist()
}

func (s OriginalSaver) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func OriginalPath(session sessions.Session, photo photos.Photo) (string, error) {
	root := filepath.Clean(session.StationSnapshot.OutputFolder)
	if root == "." || strings.TrimSpace(root) == "" {
		return "", ErrUnsafeOutputPath
	}
	folder := filepath.Join(root, sanitizeSegment(session.CustomerName)+"_"+sanitizeSegment(session.OrderNumber), sanitizeSegment(stationSegment(session)), "originals")
	ext := strings.ToLower(filepath.Ext(photo.SourcePath))
	if ext != ".jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(photo.SourcePath), filepath.Ext(photo.SourcePath))
	base = sanitizeSegment(base)
	target := filepath.Clean(filepath.Join(folder, base+"__"+photo.PhotoID+ext))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", ErrUnsafeOutputPath
	}
	return target, nil
}

func stationSegment(session sessions.Session) string {
	if strings.TrimSpace(session.StationSnapshot.StationName) != "" {
		return session.StationSnapshot.StationName
	}
	return session.StationID
}

func sanitizeSegment(in string) string {
	s := strings.TrimSpace(in)
	if s == "" {
		s = "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) || unicode.IsControl(r) {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ". ")
	if out == "" {
		out = "unknown"
	}
	reserved := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true, "COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true, "LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true}
	if reserved[strings.ToUpper(out)] {
		out = "_" + out
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

func copyOriginal(source, target string, expected int64) error {
	if expected <= 0 {
		return errors.New("invalid source size")
	}
	if samePath(source, target) {
		return validFile(target, expected)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info, err := os.Stat(target); err == nil {
		if info.IsDir() {
			return errors.New("target exists as directory")
		}
		if info.Size() != expected {
			return errors.New("target exists with different size")
		}
		if equal, err := sameFileContent(source, target); err != nil {
			return err
		} else if !equal {
			return errors.New("target exists with different content")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(target), ".original-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	written, err := io.Copy(tmp, src)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if written != expected {
		_ = tmp.Close()
		return fmt.Errorf("copy size mismatch")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return err
	}
	return validFile(target, expected)
}

func validOriginal(photo photos.Photo) error {
	return validFile(photo.LocalOriginalPath, photo.SourceSizeBytes)
}

func sameFileContent(left string, right string) (bool, error) {
	leftFile, err := os.Open(left)
	if err != nil {
		return false, err
	}
	defer leftFile.Close()
	rightFile, err := os.Open(right)
	if err != nil {
		return false, err
	}
	defer rightFile.Close()
	leftBuf := make([]byte, 32*1024)
	rightBuf := make([]byte, 32*1024)
	for {
		leftN, leftErr := leftFile.Read(leftBuf)
		rightN, rightErr := rightFile.Read(rightBuf)
		if leftN != rightN || !bytes.Equal(leftBuf[:leftN], rightBuf[:rightN]) {
			return false, nil
		}
		if errors.Is(leftErr, io.EOF) && errors.Is(rightErr, io.EOF) {
			return true, nil
		}
		if leftErr != nil && !errors.Is(leftErr, io.EOF) {
			return false, leftErr
		}
		if rightErr != nil && !errors.Is(rightErr, io.EOF) {
			return false, rightErr
		}
	}
}
func validFile(path string, expected int64) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("file is missing")
		}
		return fmt.Errorf("file cannot be inspected: %w", err)
	}
	if info.IsDir() {
		return errors.New("path points to a directory")
	}
	if info.Size() == 0 {
		return errors.New("file is empty")
	}
	if info.Size() != expected {
		return fmt.Errorf("file size mismatch: expected %d bytes, got %d bytes", expected, info.Size())
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("file is unreadable: %w", err)
	}
	return file.Close()
}
func actionableOriginalReason(label string, err error) string {
	if err == nil {
		return label + " verification failed after restart"
	}
	return label + " verification failed after restart: " + err.Error() + "; restore the original JPG or retry capture/import before processing"
}
func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
func safeError(err error) string {
	if err == nil {
		return ""
	}
	return "original save failed: " + err.Error()
}
