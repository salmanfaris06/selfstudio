package api

import (
	"errors"
	"net/http"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
)

type PhotoRetryHandler struct {
	photos    *photos.Store
	processor *processing.GradedProcessor
	guard     *processing.ProcessingGuard
	activity  *activity.Store
	broker    *events.Broker
}

type RetryProcessingData struct {
	Photo        photos.Photo `json:"photo"`
	RetryStarted bool         `json:"retry_started"`
}

func NewPhotoRetryHandler(photoStore *photos.Store, processor *processing.GradedProcessor, activityStore *activity.Store, broker *events.Broker, guard *processing.ProcessingGuard) PhotoRetryHandler {
	return PhotoRetryHandler{photos: photoStore, processor: processor, guard: guard, activity: activityStore, broker: broker}
}

func (h PhotoRetryHandler) Retry(w http.ResponseWriter, r *http.Request) {
	photoID := r.PathValue("photo_id")
	if h.photos == nil || h.processor == nil {
		writeAPIError(w, http.StatusInternalServerError, "PROCESSOR_UNAVAILABLE", "Processor retry belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	photo, ok := h.photos.Get(photoID)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "PHOTO_NOT_FOUND", "Photo tidak ditemukan.", "Refresh queue lalu pilih photo yang masih tersedia.")
		return
	}
	if err := processing.RetryEligibility(photo, processing.RetryModeManual); err != nil {
		writeRetryRejection(w, err)
		return
	}
	runner := newProcessingRunner(h.processor, h.guard, h.activity, h.broker)
	if !runner.enqueue(photo.PhotoID, true) {
		writeAPIError(w, http.StatusConflict, "ALREADY_PROCESSING", "Photo sedang diproses.", "Tunggu proses saat ini selesai, lalu refresh queue.")
		return
	}
	if h.activity != nil {
		stationID := photo.StationID
		sessionID := photo.SessionID
		h.activity.RecordWithRefs("photo.processing_retry", activity.ResultSuccess, "Operator memulai retry graded processing.", &stationID, &sessionID)
	}
	updated, _ := h.photos.Get(photo.PhotoID)
	writeData(w, http.StatusAccepted, RetryProcessingData{Photo: updated, RetryStarted: true})
}

func writeRetryRejection(w http.ResponseWriter, err error) {
	var rejection processing.RetryRejection
	if errors.As(err, &rejection) {
		status := http.StatusConflict
		if rejection.Code == "ORIGINAL_INVALID" || rejection.Code == "ORIGINAL_NOT_SAVED" || rejection.Code == "PHOTO_NOT_ELIGIBLE" || rejection.Code == "NOT_RETRYABLE" {
			status = http.StatusUnprocessableEntity
		}
		writeAPIError(w, status, rejection.Code, rejection.Message, rejection.Action)
		return
	}
	writeAPIError(w, http.StatusConflict, "NOT_RETRYABLE", "Photo tidak bisa di-retry.", "Periksa status photo lalu coba lagi.")
}
