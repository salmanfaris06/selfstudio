package processing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/sessions"
)

var ErrUnsafeGradedOutputPath = errors.New("unsafe graded output path")

type GradedProcessor struct {
	Photos    *photos.Store
	Sessions  *sessions.Store
	Processor LUTProcessor
	Persist   func() error
	Now       func() time.Time
}

type GradedResult struct {
	Photo photos.Photo
	Err   error
}

func (p GradedProcessor) Process(ctx context.Context, photoID string) GradedResult {
	now := p.now()
	photo, ok := p.Photos.Get(photoID)
	if !ok {
		return GradedResult{Err: fmt.Errorf("photo not found: %s", photoID)}
	}
	if photo.GradedProcessingStatus == photos.GradedStatusProcessed {
		if validGraded(photo) == nil {
			return GradedResult{Photo: photo}
		}
	}
	if photo.OriginalSaveStatus != photos.OriginalStatusSaved || photo.LocalOriginalPath == "" || validOriginal(photo) != nil {
		return p.fail(photo, "graded processing requires verified saved original", now)
	}
	if p.Sessions == nil {
		return p.fail(photo, "session store unavailable", now)
	}
	session, err := p.Sessions.Get(photo.SessionID)
	if err != nil {
		return p.fail(photo, "session snapshot unavailable", now)
	}
	lutPath, err := validateLUTPath(session.StationSnapshot.DefaultLUTPath)
	if err != nil {
		return p.fail(photo, err.Error(), now)
	}
	target, err := GradedPath(session, photo)
	if err != nil {
		return p.fail(photo, err.Error(), now)
	}
	photo = p.Photos.MarkGradedProcessing(photo.PhotoID, target, lutPath, now)
	if err := p.persist(); err != nil {
		return GradedResult{Photo: photo, Err: err}
	}
	if err := writeGraded(ctx, p.Processor, photo.LocalOriginalPath, lutPath, target); err != nil {
		return p.fail(photo, safeGradedError(err), p.now())
	}
	if err := validGradedPath(target); err != nil {
		return p.fail(photo, "graded output verification failed", p.now())
	}
	photo = p.Photos.MarkGradedProcessed(photo.PhotoID, target, lutPath, p.now())
	if err := p.persist(); err != nil {
		failed := p.Photos.MarkGradedFailed(photo.PhotoID, "graded output created but state persistence failed; retry to reconcile", p.now())
		_ = p.persist()
		return GradedResult{Photo: failed, Err: err}
	}
	return GradedResult{Photo: photo}
}

func (p GradedProcessor) Reconcile(ctx context.Context) []GradedResult {
	results := []GradedResult{}
	for _, photo := range p.Photos.ListAll() {
		switch photo.GradedProcessingStatus {
		case photos.GradedStatusProcessed:
			if err := validGraded(photo); err != nil {
				failed := p.Photos.MarkGradedFailed(photo.PhotoID, actionableGradedReason(err), p.now())
				if persistErr := p.persist(); persistErr != nil {
					results = append(results, GradedResult{Photo: failed, Err: fmt.Errorf("persist graded recovery failure: %w", persistErr)})
				} else {
					results = append(results, GradedResult{Photo: failed, Err: err})
				}
			}
		case "", photos.GradedStatusPending, photos.GradedStatusProcessing:
			if photo.OriginalSaveStatus == photos.OriginalStatusSaved {
				results = append(results, p.Process(ctx, photo.PhotoID))
			}
		case photos.GradedStatusFailed:
			if photo.OriginalSaveStatus == photos.OriginalStatusSaved && IsAutomaticRetryEligible(photo) {
				results = append(results, p.Process(ctx, photo.PhotoID))
			}
		}
	}
	_ = p.persist()
	return results
}

func (p GradedProcessor) fail(photo photos.Photo, message string, now time.Time) GradedResult {
	failed := p.Photos.MarkGradedFailed(photo.PhotoID, message, now)
	if err := p.persist(); err != nil {
		return GradedResult{Photo: failed, Err: fmt.Errorf("persist graded failure: %w", err)}
	}
	return GradedResult{Photo: failed, Err: errors.New(message)}
}

func (p GradedProcessor) persist() error {
	if p.Persist == nil {
		return nil
	}
	return p.Persist()
}
func (p GradedProcessor) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func GradedPath(session sessions.Session, photo photos.Photo) (string, error) {
	root := filepath.Clean(session.StationSnapshot.OutputFolder)
	if root == "." || strings.TrimSpace(root) == "" {
		return "", ErrUnsafeGradedOutputPath
	}
	folder := filepath.Join(root, sanitizeSegment(session.CustomerName)+"_"+sanitizeSegment(session.OrderNumber), sanitizeSegment(stationSegment(session)), "graded")
	base := strings.TrimSuffix(filepath.Base(photo.SourcePath), filepath.Ext(photo.SourcePath))
	base = sanitizeSegment(base)
	target := filepath.Clean(filepath.Join(folder, base+"__"+photo.PhotoID+".jpg"))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", ErrUnsafeGradedOutputPath
	}
	return target, nil
}

func validateLUTPath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == "" {
		return "", errors.New("LUT_MISSING: session snapshot LUT path is empty")
	}
	if strings.ToLower(filepath.Ext(clean)) != ".cube" {
		return "", errors.New("LUT_INVALID: session snapshot LUT must be a .cube file")
	}
	info, err := os.Stat(clean)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return "", errors.New("LUT_UNREADABLE: session snapshot LUT is missing or unreadable")
	}
	return clean, nil
}

func writeGraded(ctx context.Context, processor LUTProcessor, inputPath string, lutPath string, target string) error {
	if processor == nil {
		return fmt.Errorf("%w: LUT processor adapter is not configured", ErrLUTProcessorUnavailable)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info, err := os.Stat(target); err == nil {
		if info.IsDir() || info.Size() == 0 {
			return errors.New("existing graded target invalid")
		}
		return errors.New("existing graded target already exists for this deterministic job path; refusing to accept or overwrite unverified output")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".graded-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpName)
	if err := processor.Apply(ctx, inputPath, lutPath, tmpName); err != nil {
		return err
	}
	if err := validGradedPath(tmpName); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return err
	}
	return validGradedPath(target)
}

func validGraded(photo photos.Photo) error { return validGradedPath(photo.LocalGradedPath) }
func validGradedPath(path string) error {
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
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("file is unreadable: %w", err)
	}
	return f.Close()
}
func safeGradedError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if errors.Is(err, ErrLUTProcessorUnavailable) || strings.Contains(msg, "LUT_PROCESSOR_UNAVAILABLE") {
		return "LUT_PROCESSOR_UNAVAILABLE: Install ImageMagick 7 (`magick`) lalu retry processing."
	}
	if strings.HasPrefix(msg, "LUT_") {
		return msg
	}
	return "LUT_PROCESSING_FAILED: Graded JPG gagal dibuat; original tetap aman dan bisa di-retry."
}
