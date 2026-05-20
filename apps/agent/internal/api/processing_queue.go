package api

import (
	"net/http"
	"strconv"
	"strings"

	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
)

type ProcessingQueueHandler struct {
	photoStore *photos.Store
}

func NewProcessingQueueHandler(photoStore *photos.Store) ProcessingQueueHandler {
	return ProcessingQueueHandler{photoStore: photoStore}
}

func (h ProcessingQueueHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.photoStore == nil {
		writeAPIError(w, http.StatusInternalServerError, "PROCESSING_QUEUE_UNAVAILABLE", "Status processing queue belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}
	filter, ok := parseProcessingQueueFilter(w, r)
	if !ok {
		return
	}
	writeData(w, http.StatusOK, processing.BuildQueueStatus(h.photoStore, filter))
}

func parseProcessingQueueFilter(w http.ResponseWriter, r *http.Request) (processing.QueueFilter, bool) {
	query := r.URL.Query()
	filter := processing.QueueFilter{StationID: strings.TrimSpace(query.Get("station_id")), SessionID: strings.TrimSpace(query.Get("session_id")), Status: strings.TrimSpace(query.Get("status"))}
	if filter.StationID != "" && !safeFilterID(filter.StationID) {
		writeAPIErrorWithDetails(w, http.StatusBadRequest, "INVALID_PROCESSING_QUEUE_FILTER", "Filter station_id tidak valid.", "Gunakan station_id aman dari konfigurasi station.", map[string]any{"field": "station_id"})
		return processing.QueueFilter{}, false
	}
	if filter.SessionID != "" && !safeFilterID(filter.SessionID) {
		writeAPIErrorWithDetails(w, http.StatusBadRequest, "INVALID_PROCESSING_QUEUE_FILTER", "Filter session_id tidak valid.", "Gunakan session_id dari API session.", map[string]any{"field": "session_id"})
		return processing.QueueFilter{}, false
	}
	if filter.Status != "" && !validQueueStatusFilter(filter.Status) {
		writeAPIErrorWithDetails(w, http.StatusBadRequest, "INVALID_PROCESSING_QUEUE_FILTER", "Filter status processing tidak valid.", "Gunakan not_eligible, pending, processing, processed, failed, atau retrying.", map[string]any{"field": "status"})
		return processing.QueueFilter{}, false
	}
	limitRaw := strings.TrimSpace(query.Get("limit"))
	if limitRaw != "" {
		limit, err := strconv.Atoi(limitRaw)
		if err != nil || limit <= 0 {
			writeAPIErrorWithDetails(w, http.StatusBadRequest, "INVALID_PROCESSING_QUEUE_FILTER", "Filter limit harus angka positif.", "Kirim limit lebih besar dari 0 atau kosongkan filter.", map[string]any{"field": "limit"})
			return processing.QueueFilter{}, false
		}
		filter.Limit = limit
	}
	return filter, true
}

func validQueueStatusFilter(status string) bool {
	switch status {
	case photos.GradedStatusNotEligible, photos.GradedStatusPending, photos.GradedStatusProcessing, photos.GradedStatusProcessed, photos.GradedStatusFailed, "retrying":
		return true
	default:
		return false
	}
}

func safeFilterID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}
