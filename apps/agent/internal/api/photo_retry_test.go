package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
	"selfstudio/agent/internal/sessions"
)

type blockingRetryLUT struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	calls   int
	mu      sync.Mutex
}

func (f *blockingRetryLUT) Apply(ctx context.Context, inputPath, lutPath, outputPath string) error {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	f.once.Do(func() { close(f.started) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.release:
	}
	return os.WriteFile(outputPath, []byte("graded"), 0o644)
}

func TestPhotoRetryEndpointRequiresAuthAndTrustedOrigin(t *testing.T) {
	mux, _, photoID, _ := photoRetryTestMux(t, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/photos/"+photoID+"/retry-processing", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status=%d body=%s", rec.Code, rec.Body.String())
	}

	_, token, _ := photoRetryAuth(t)
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/photos/"+photoID+"/retry-processing", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("untrusted origin status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPhotoRetryEndpointStartsManualRetryWithDataAndSafeEventsActivity(t *testing.T) {
	lut := &blockingRetryLUT{started: make(chan struct{}), release: make(chan struct{})}
	mux, token, photoID, deps := photoRetryTestMux(t, lut)
	eventsCh, unsubscribe := deps.broker.Subscribe()
	defer unsubscribe()

	rec := httptest.NewRecorder()
	req := authorizedRetryRequest(photoID, token)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data RetryProcessingData `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Data.RetryStarted || body.Data.Photo.PhotoID != photoID {
		t.Fatalf("unexpected data response: %+v", body.Data)
	}

	<-lut.started
	close(lut.release)
	time.Sleep(50 * time.Millisecond)

	eventNames := map[string]bool{}
	deadline := time.After(500 * time.Millisecond)
	for len(eventNames) < 3 {
		select {
		case event := <-eventsCh:
			if event.EntityID != photoID {
				t.Fatalf("event %s used unsafe entity id %q", event.EventType, event.EntityID)
			}
			if strings.Contains(event.EntityID, string(filepath.Separator)) {
				t.Fatalf("event entity id leaked path: %q", event.EntityID)
			}
			eventNames[event.EventType] = true
		case <-deadline:
			t.Fatalf("timed out waiting for retry events, got %+v", eventNames)
		}
	}
	for _, name := range []string{"photo.processing_started", "photo.processed", "queue.updated"} {
		if !eventNames[name] {
			t.Fatalf("missing event %s in %+v", name, eventNames)
		}
	}

	foundManual := false
	for _, entry := range deps.activity.Recent(20, "") {
		if strings.Contains(entry.Message, deps.root) || strings.Contains(entry.Message, "orig.jpg") || strings.Contains(entry.Message, "look.cube") {
			t.Fatalf("activity leaked raw path: %+v", entry)
		}
		if entry.ActionType == "photo.processing_retry" {
			foundManual = true
		}
	}
	if !foundManual {
		t.Fatal("manual retry activity was not recorded")
	}
}

func TestPhotoRetryEndpointReturnsActionableErrorShape(t *testing.T) {
	mux, token, photoID, deps := photoRetryTestMux(t, nil)
	processed := deps.photos.MarkGradedProcessed(photoID, filepath.Join(deps.root, "done.jpg"), filepath.Join(deps.root, "look.cube"), time.Now())
	_ = os.WriteFile(processed.LocalGradedPath, []byte("graded"), 0o644)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, authorizedRetryRequest(photoID, token))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Action  string `json:"action"`
			Details any    `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "ALREADY_PROCESSED" || body.Error.Message == "" || body.Error.Action == "" {
		t.Fatalf("unexpected error shape: %+v", body.Error)
	}
}

func TestPhotoRetryDuplicateClicksUseSharedGuard(t *testing.T) {
	lut := &blockingRetryLUT{started: make(chan struct{}), release: make(chan struct{})}
	mux, token, photoID, _ := photoRetryTestMux(t, lut)

	first := httptest.NewRecorder()
	mux.ServeHTTP(first, authorizedRetryRequest(photoID, token))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	<-lut.started

	second := httptest.NewRecorder()
	mux.ServeHTTP(second, authorizedRetryRequest(photoID, token))
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "ALREADY_PROCESSING") {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	close(lut.release)
}

type photoRetryDeps struct {
	photos   *photos.Store
	activity *activity.Store
	broker   *events.Broker
	root     string
}

func photoRetryTestMux(t *testing.T, lut processing.LUTProcessor) (http.Handler, string, string, photoRetryDeps) {
	t.Helper()
	manager, token, activityStore := photoRetryAuth(t)
	broker := events.NewBroker()
	root := t.TempDir()
	orig := filepath.Join(root, "orig.jpg")
	lutPath := filepath.Join(root, "look.cube")
	if err := os.WriteFile(orig, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lutPath, []byte("cube"), 0o644); err != nil {
		t.Fatal(err)
	}
	photoStore := photos.NewStore()
	photo := photoStore.Route("station_1", "session_1", filepath.Join(root, "camera.jpg"), int64(len("original")), time.Now(), time.Now(), time.Now())
	photoStore.MarkOriginalSaved(photo.PhotoID, orig, time.Now())
	photoStore.MarkGradedFailed(photo.PhotoID, "LUT_PROCESSING_FAILED", time.Now())
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{{SessionID: "session_1", StationID: "station_1", Status: sessions.StatusActive, CustomerName: "Alice", OrderNumber: "1", TimerSeconds: 300, StartedAt: time.Now(), EndsAt: time.Now().Add(time.Hour), StationSnapshot: sessions.StationSnapshot{StationName: "Station 1", DefaultLUTPath: lutPath, OutputFolder: root}}})
	if lut == nil {
		lut = &blockingRetryLUT{started: make(chan struct{}), release: make(chan struct{})}
	}
	processor := &processing.GradedProcessor{Photos: photoStore, Sessions: sessionStore, Processor: lut}
	guard := processing.NewProcessingGuard()
	mux := NewMuxWithPhotoRetry(NewAuthHandlerWithActivity(manager, activityStore), NewEventsHandler(broker), NewActivityHandler(activityStore), StationsHandler{}, ReadinessHandler{}, EventReadinessHandler{}, StationConfigHandler{}, WatchValidationHandler{}, SessionsHandler{}, IngestionHandler{}, QuarantineHandler{}, ProcessingQueueHandler{}, NewPhotoRetryHandler(photoStore, processor, activityStore, broker, guard))
	return mux, token, photo.PhotoID, photoRetryDeps{photos: photoStore, activity: activityStore, broker: broker, root: root}
}

func photoRetryAuth(t *testing.T) (*auth.Manager, string, *activity.Store) {
	t.Helper()
	manager, err := auth.NewManager("123456")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.Login("123456")
	if err != nil {
		t.Fatal(err)
	}
	return manager, token, activity.NewStore(50)
}

func authorizedRetryRequest(photoID string, token string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/photos/"+photoID+"/retry-processing", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	return req
}
