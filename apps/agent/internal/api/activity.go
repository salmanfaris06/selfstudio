package api

import (
	"net/http"
	"regexp"
	"strconv"

	"selfstudio/agent/internal/activity"
)

var actionTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)?$`)

type ActivityHandler struct {
	store *activity.Store
}

type ActivityListData struct {
	Entries []activity.Entry `json:"entries"`
}

type ConfigPlaceholderData struct {
	Recorded bool `json:"recorded"`
}

func NewActivityHandler(store *activity.Store) ActivityHandler {
	return ActivityHandler{store: store}
}

func (h ActivityHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusInternalServerError, "ACTIVITY_UNAVAILABLE", "Activity log belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Limit activity log tidak valid.", "Gunakan limit angka antara 1 dan 100.")
			return
		}
		limit = parsed
	}

	actionType := r.URL.Query().Get("action_type")
	if actionType != "" && (len(actionType) > 80 || !actionTypePattern.MatchString(actionType)) {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "Filter action_type tidak valid.", "Gunakan action_type yang tersedia di dashboard.")
		return
	}
	writeData(w, http.StatusOK, ActivityListData{Entries: h.store.Recent(limit, actionType)})
}

func (h ActivityHandler) ConfigPlaceholderAction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusInternalServerError, "ACTIVITY_UNAVAILABLE", "Activity log belum siap.", "Restart aplikasi lalu coba lagi.")
		return
	}

	h.store.Record("config.placeholder", activity.ResultSuccess, "Config placeholder action dicatat.")
	writeData(w, http.StatusOK, ConfigPlaceholderData{Recorded: true})
}
