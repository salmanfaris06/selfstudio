package processing

import (
	"strings"
	"sync"

	"selfstudio/agent/internal/photos"
)

const MaxGradedAttempts = 3

type RetryMode string

const (
	RetryModeAutomatic RetryMode = "automatic"
	RetryModeManual    RetryMode = "manual"
)

type RetryRejection struct {
	Code    string
	Message string
	Action  string
}

func (r RetryRejection) Error() string { return r.Code + ": " + r.Message }

func RetryEligibility(photo photos.Photo, mode RetryMode) error {
	if strings.TrimSpace(photo.PhotoID) == "" {
		return RetryRejection{Code: "PHOTO_NOT_FOUND", Message: "Photo tidak ditemukan.", Action: "Refresh queue lalu pilih photo yang masih tersedia."}
	}
	if photo.OriginalSaveStatus != photos.OriginalStatusSaved {
		return RetryRejection{Code: "ORIGINAL_NOT_SAVED", Message: "Original JPG belum tersimpan aman.", Action: "Ulangi ingestion atau periksa original-save sebelum retry processing."}
	}
	if strings.TrimSpace(photo.LocalOriginalPath) == "" || validOriginal(photo) != nil {
		return RetryRejection{Code: "ORIGINAL_INVALID", Message: "Saved original JPG hilang atau tidak valid.", Action: "Pulihkan original JPG lokal dari backup/source lalu retry."}
	}
	if photo.ProcessingStatus != photos.ProcessingStatusEligible {
		return RetryRejection{Code: "PHOTO_NOT_ELIGIBLE", Message: "Photo belum eligible untuk graded processing.", Action: "Pastikan original sudah tersimpan dan photo masuk session yang valid."}
	}
	switch photo.GradedProcessingStatus {
	case photos.GradedStatusFailed:
		// allowed, subject to attempt policy below
	case photos.GradedStatusProcessing:
		return RetryRejection{Code: "ALREADY_PROCESSING", Message: "Photo sedang diproses.", Action: "Tunggu proses saat ini selesai, lalu refresh queue."}
	case photos.GradedStatusProcessed:
		return RetryRejection{Code: "ALREADY_PROCESSED", Message: "Photo sudah memiliki graded output.", Action: "Tidak perlu retry. Jika output salah, tangani melalui workflow koreksi terpisah."}
	default:
		return RetryRejection{Code: "NOT_RETRYABLE", Message: "Status graded photo belum gagal sehingga tidak bisa di-retry.", Action: "Tunggu processing selesai atau jalankan scan/recovery yang sesuai."}
	}
	if mode == RetryModeAutomatic && photo.GradedAttemptCount >= MaxGradedAttempts {
		return RetryRejection{Code: "AUTOMATIC_RETRY_LIMIT_REACHED", Message: "Batas automatic retry sudah tercapai.", Action: "Operator perlu memperbaiki penyebab gagal lalu klik Retry processing manual."}
	}
	return nil
}

func IsAutomaticRetryEligible(photo photos.Photo) bool {
	return RetryEligibility(photo, RetryModeAutomatic) == nil
}

type ProcessingGuard struct {
	mu      sync.Mutex
	running map[string]struct{}
}

func NewProcessingGuard() *ProcessingGuard { return &ProcessingGuard{running: map[string]struct{}{}} }

func (g *ProcessingGuard) TryStart(photoID string) bool {
	if g == nil || strings.TrimSpace(photoID) == "" {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running == nil {
		g.running = map[string]struct{}{}
	}
	if _, ok := g.running[photoID]; ok {
		return false
	}
	g.running[photoID] = struct{}{}
	return true
}

func (g *ProcessingGuard) Done(photoID string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.running, photoID)
}
