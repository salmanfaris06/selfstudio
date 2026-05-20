package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/stations"
)

func TestIngestionScanQuarantinesNoActiveSessionWithEventAndActivity(t *testing.T) {
	manager, token, stationStore, activityStore, broker, sessionStore, persistence := sessionDeps(t)
	mux := photoRoutingMux(manager, stationStore, activityStore, broker, sessionStore, persistence, photos.NewStore())
	input := stationStore.List()[0].InputFolder
	if err := os.WriteFile(input+"/NOSESSION.JPG", []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	ch, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/ingestion/scan", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"photos"`) || !strings.Contains(body, `"routed_photos"`) || !strings.Contains(body, `"errors"`) || !strings.Contains(body, `"quarantined_photos"`) || !strings.Contains(body, `"reason":"no_active_session"`) || !strings.Contains(body, `"quarantine_id":"quar_`) {
		t.Fatalf("scan status=%d body=%s", rec.Code, body)
	}
	foundEvent := false
	for i := 0; i < 2; i++ {
		event := <-ch
		if event.EventType == "photo.quarantined" && event.EntityType == "quarantine" && strings.HasPrefix(event.EntityID, "quar_") {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Fatalf("photo.quarantined event not published")
	}
	entries := activityStore.Recent(10, "photo.quarantined")
	if len(entries) != 1 || entries[0].StationID == nil || *entries[0].StationID != stations.Station1ID || entries[0].SessionID != nil {
		t.Fatalf("activity entries=%+v", entries)
	}

	summary := httptest.NewRecorder()
	summaryReq := httptest.NewRequest(http.MethodGet, "/api/stations/"+stations.Station1ID+"/quarantine-summary", nil)
	summaryReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(summary, summaryReq)
	summaryBody := summary.Body.String()
	if summary.Code != http.StatusOK || !strings.Contains(summaryBody, `"station_quarantine_count":1`) || !strings.Contains(summaryBody, `"latest_quarantine_reason":"no_active_session"`) {
		t.Fatalf("summary status=%d body=%s", summary.Code, summaryBody)
	}
}
