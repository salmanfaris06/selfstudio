package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
	"selfstudio/agent/internal/sessions"
)

type failingRecoveryLUT struct {
	calls int
}

func (f *failingRecoveryLUT) Apply(ctx context.Context, inputPath, lutPath, outputPath string) error {
	f.calls++
	return errors.New("boom")
}

func TestRecoveryAutoRetryAtAttemptTwoStopsAtMaxAndPublishesQueueRefresh(t *testing.T) {
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
	for i := 0; i < processing.MaxGradedAttempts-1; i++ {
		photoStore.MarkGradedProcessing(photo.PhotoID, filepath.Join(root, "graded.jpg"), lutPath, time.Now())
		photoStore.MarkGradedFailed(photo.PhotoID, "previous failure", time.Now())
	}
	sessionStore := sessions.NewStore()
	_ = sessionStore.ReplaceAll([]sessions.Session{{SessionID: "session_1", StationID: "station_1", Status: sessions.StatusActive, CustomerName: "Alice", OrderNumber: "1", TimerSeconds: 300, StartedAt: time.Now(), EndsAt: time.Now().Add(time.Hour), StationSnapshot: sessions.StationSnapshot{StationName: "Station 1", DefaultLUTPath: lutPath, OutputFolder: root}}})
	lut := &failingRecoveryLUT{}
	processor := &processing.GradedProcessor{Photos: photoStore, Sessions: sessionStore, Processor: lut}
	broker := events.NewBroker()
	ch, unsub := broker.Subscribe()
	defer unsub()

	started := EnqueueProcessing(processor, processing.NewProcessingGuard(), activity.NewStore(20), broker, photo.PhotoID, false)
	if !started {
		t.Fatal("recovery enqueue did not start")
	}

	seenFailure := false
	seenQueue := false
	deadline := time.After(2 * time.Second)
	for !seenFailure || !seenQueue {
		select {
		case ev := <-ch:
			if ev.EventType == "photo.processing_failed" && ev.EntityID == photo.PhotoID {
				seenFailure = true
			}
			if ev.EventType == "queue.updated" && ev.EntityID == photo.PhotoID {
				seenQueue = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for terminal recovery events, failure=%v queue=%v", seenFailure, seenQueue)
		}
	}
	updated, _ := photoStore.Get(photo.PhotoID)
	if updated.GradedProcessingStatus != photos.GradedStatusFailed || updated.GradedAttemptCount != processing.MaxGradedAttempts {
		t.Fatalf("queue state should remain failed/manual at max: %+v", updated)
	}
	if lut.calls != 1 {
		t.Fatalf("attempt two should consume exactly one automatic attempt, got %d", lut.calls)
	}
	if processing.IsAutomaticRetryEligible(updated) {
		t.Fatal("photo should no longer be automatic retry eligible at max")
	}
}
