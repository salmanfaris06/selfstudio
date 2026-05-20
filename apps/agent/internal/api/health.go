package api

import (
	"context"
	"net/http"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/cameras"
)

type HealthData struct {
	Service  string                `json:"service"`
	Status   string                `json:"status"`
	Database HealthComponentStatus `json:"database"`
	Worker   HealthComponentStatus `json:"worker"`
	Disk     HealthComponentStatus `json:"disk"`
}

type HealthComponentStatus struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Action string `json:"action"`
}

func NewMux(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler) http.Handler {
	return NewMuxWithStations(authHandler, eventsHandler, activityHandler, StationsHandler{})
}

func NewMuxWithStations(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler) http.Handler {
	return NewMuxWithReadiness(authHandler, eventsHandler, activityHandler, stationsHandler, ReadinessHandler{})
}

func NewMuxWithReadiness(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler) http.Handler {
	return NewMuxWithEventReadiness(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, EventReadinessHandler{})
}

func NewMuxWithEventReadiness(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler) http.Handler {
	return NewMuxWithStationConfig(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, StationConfigHandler{})
}

func NewMuxWithStationConfig(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler) http.Handler {
	return NewMuxWithWatchValidation(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, WatchValidationHandler{})
}

func NewMuxWithWatchValidation(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler) http.Handler {
	return NewMuxWithSessions(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, SessionsHandler{})
}

func NewMuxWithSessions(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler) http.Handler {
	return NewMuxWithIngestion(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, IngestionHandler{})
}

func NewMuxWithIngestion(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler) http.Handler {
	return NewMuxWithQuarantine(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, ingestionHandler, QuarantineHandler{})
}

func NewMuxWithQuarantine(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler, quarantineHandler QuarantineHandler) http.Handler {
	return NewMuxWithProcessingQueue(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, ingestionHandler, quarantineHandler, ProcessingQueueHandler{})
}

func NewMuxWithProcessingQueue(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler, quarantineHandler QuarantineHandler, processingQueueHandler ProcessingQueueHandler) http.Handler {
	return NewMuxWithPhotoRetry(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, ingestionHandler, quarantineHandler, processingQueueHandler, PhotoRetryHandler{})
}

func NewMuxWithPhotoRetry(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler, quarantineHandler QuarantineHandler, processingQueueHandler ProcessingQueueHandler, photoRetryHandler PhotoRetryHandler) http.Handler {
	return NewMuxWithCloudSettings(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, ingestionHandler, quarantineHandler, processingQueueHandler, photoRetryHandler, CloudSettingsHandler{})
}

func NewMuxWithCloudSettings(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler, quarantineHandler QuarantineHandler, processingQueueHandler ProcessingQueueHandler, photoRetryHandler PhotoRetryHandler, cloudSettingsHandler CloudSettingsHandler) http.Handler {
	return NewMuxWithCloudTargets(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, ingestionHandler, quarantineHandler, processingQueueHandler, photoRetryHandler, cloudSettingsHandler, CloudTargetHandler{})
}

func NewMuxWithCloudTargets(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler, quarantineHandler QuarantineHandler, processingQueueHandler ProcessingQueueHandler, photoRetryHandler PhotoRetryHandler, cloudSettingsHandler CloudSettingsHandler, cloudTargetHandler CloudTargetHandler) http.Handler {
	return NewMuxWithUploads(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, ingestionHandler, quarantineHandler, processingQueueHandler, photoRetryHandler, cloudSettingsHandler, cloudTargetHandler, SessionUploadsHandler{})
}

func NewMuxWithUploads(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler, quarantineHandler QuarantineHandler, processingQueueHandler ProcessingQueueHandler, photoRetryHandler PhotoRetryHandler, cloudSettingsHandler CloudSettingsHandler, cloudTargetHandler CloudTargetHandler, sessionUploadsHandler SessionUploadsHandler) http.Handler {
	return NewMuxWithUploadsAndCamera(authHandler, eventsHandler, activityHandler, stationsHandler, readinessHandler, eventReadinessHandler, stationConfigHandler, watchValidationHandler, sessionsHandler, ingestionHandler, quarantineHandler, processingQueueHandler, photoRetryHandler, cloudSettingsHandler, cloudTargetHandler, sessionUploadsHandler, cameras.NewDiscoveryService(nil), cameras.NewTetherSupervisor(nil), nil)
}

func NewMuxWithUploadsAndCamera(authHandler AuthHandler, eventsHandler EventsHandler, activityHandler ActivityHandler, stationsHandler StationsHandler, readinessHandler ReadinessHandler, eventReadinessHandler EventReadinessHandler, stationConfigHandler StationConfigHandler, watchValidationHandler WatchValidationHandler, sessionsHandler SessionsHandler, ingestionHandler IngestionHandler, quarantineHandler QuarantineHandler, processingQueueHandler ProcessingQueueHandler, photoRetryHandler PhotoRetryHandler, cloudSettingsHandler CloudSettingsHandler, cloudTargetHandler CloudTargetHandler, sessionUploadsHandler SessionUploadsHandler, discovery cameras.DiscoveryService, tetherSupervisor *cameras.TetherSupervisor, testCaptureService *cameras.TestCaptureService, tetherExtras ...any) http.Handler {
	camerasHandler := NewCamerasHandler(stationsHandler.store, stationsHandler.persistence, activityHandler.store, eventsHandler.broker, discovery)
	if tetherSupervisor == nil {
		tetherSupervisor = cameras.NewTetherSupervisor(nil)
	}
	var tetherStateStore *cameras.TetherStateStore
	var tetherRecovery *cameras.TetherRecoveryCoordinator
	for _, extra := range tetherExtras {
		if store, ok := extra.(*cameras.TetherStateStore); ok {
			tetherStateStore = store
		}
		if recovery, ok := extra.(*cameras.TetherRecoveryCoordinator); ok {
			tetherRecovery = recovery
		}
	}
	tetherHandler := NewCameraTetherHandlerWithRecovery(stationsHandler.store, tetherSupervisor, tetherStateStore, tetherRecovery, &camerasHandler)
	cameraTestCaptureHandler := NewCameraTestCaptureHandler(stationsHandler.store, activityHandler.store, eventsHandler.broker, tetherSupervisor, testCaptureService)
	smokeVerifier := DefaultHardwareSmokeVerifier{Discovery: discovery, Tether: tetherSupervisor, PhotoStore: sessionsHandler.photoStore, SessionStore: sessionsHandler.sessionStore, UploadTargets: cloudTargetHandler.Targets, UploadJobs: sessionUploadsHandler.Jobs}
	smokeRunner := &cameras.HardwareSmokeRunner{Verifier: smokeVerifier, Writer: &cameras.HardwareSmokeReportWriter{}}
	hardwareSmokeHandler := NewHardwareSmokeHandler(stationsHandler.store, activityHandler.store, eventsHandler.broker, smokeRunner)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", HealthHandler)
	mux.HandleFunc("GET /api/health", HealthHandler)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.Handle("POST /api/auth/logout", RequireTrustedOrigin(http.HandlerFunc(authHandler.Logout)))
	mux.HandleFunc("GET /api/auth/session", authHandler.Session)
	mux.Handle("GET /api/activity", RequireAuth(authHandler.manager, http.HandlerFunc(activityHandler.List)))
	mux.Handle("GET /api/cloud/settings", RequireAuth(authHandler.manager, http.HandlerFunc(cloudSettingsHandler.Get)))
	mux.Handle("PUT /api/cloud/settings", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(cloudSettingsHandler.Put))))
	mux.Handle("POST /api/cloud/settings/check", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(cloudSettingsHandler.Check))))
	mux.Handle("POST /api/cloud/settings/object-key-preview", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(cloudSettingsHandler.Preview))))
	mux.Handle("POST /api/cloud/settings/folder-preview", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(cloudSettingsHandler.FolderPreview))))
	mux.Handle("GET /api/sessions/{session_id}/cloud-target", RequireAuth(authHandler.manager, http.HandlerFunc(cloudTargetHandler.Get)))
	mux.Handle("POST /api/sessions/{session_id}/cloud-target/resolve", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(cloudTargetHandler.Resolve))))
	mux.Handle("GET /api/sessions/{session_id}/uploads", RequireAuth(authHandler.manager, http.HandlerFunc(sessionUploadsHandler.Get)))
	mux.Handle("POST /api/sessions/{session_id}/uploads/start", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(sessionUploadsHandler.Start))))
	mux.Handle("POST /api/uploads/{job_id}/retry", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(sessionUploadsHandler.RetryJob))))
	mux.Handle("POST /api/sessions/{session_id}/uploads/retry", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(sessionUploadsHandler.RetrySession))))
	mux.Handle("POST /api/config/placeholder-action", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(activityHandler.ConfigPlaceholderAction))))
	mux.Handle("GET /api/stations", RequireAuth(authHandler.manager, http.HandlerFunc(stationsHandler.List)))
	mux.Handle("PUT /api/stations/{station_id}", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(stationsHandler.Update))))
	mux.Handle("POST /api/cameras/gphoto2/discover", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(camerasHandler.Discover))))
	mux.Handle("PUT /api/stations/{station_id}/camera-assignment", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(camerasHandler.Assign))))
	mux.Handle("GET /api/stations/{station_id}/tether-listener", RequireAuth(authHandler.manager, http.HandlerFunc(tetherHandler.Get)))
	mux.Handle("POST /api/stations/{station_id}/tether-listener/start", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(tetherHandler.Start))))
	mux.Handle("POST /api/stations/{station_id}/tether-listener/stop", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(tetherHandler.Stop))))
	mux.Handle("POST /api/stations/{station_id}/tether-listener/retry", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(tetherHandler.Retry))))
	mux.Handle("PUT /api/stations/{station_id}/tether-listener/settings", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(tetherHandler.PutSettings))))
	mux.Handle("POST /api/stations/{station_id}/camera-test-capture", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(cameraTestCaptureHandler.Run))))
	mux.Handle("POST /api/stations/{station_id}/hardware-smoke-tests", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(hardwareSmokeHandler.Run))))
	mux.Handle("GET /api/stations/{station_id}/readiness", RequireAuth(authHandler.manager, http.HandlerFunc(readinessHandler.Get)))
	mux.Handle("POST /api/stations/{station_id}/readiness/check", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(readinessHandler.Check))))
	mux.Handle("POST /api/stations/{station_id}/health/refresh", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(readinessHandler.RefreshHealth))))
	mux.Handle("GET /api/readiness", RequireAuth(authHandler.manager, http.HandlerFunc(eventReadinessHandler.Get)))
	mux.Handle("POST /api/readiness/check", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(eventReadinessHandler.Check))))
	mux.Handle("POST /api/stations/backup", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(stationConfigHandler.Backup))))
	mux.Handle("POST /api/stations/restore", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(stationConfigHandler.Restore))))
	mux.Handle("POST /api/stations/{station_id}/validation/watch-test", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(watchValidationHandler.Run))))
	mux.Handle("GET /api/sessions", RequireAuth(authHandler.manager, http.HandlerFunc(sessionsHandler.List)))
	mux.Handle("GET /api/sessions/{session_id}", RequireAuth(authHandler.manager, http.HandlerFunc(sessionsHandler.Get)))
	mux.Handle("GET /api/stations/{station_id}/quarantine-summary", RequireAuth(authHandler.manager, http.HandlerFunc(sessionsHandler.StationQuarantineSummary)))
	mux.Handle("POST /api/stations/{station_id}/sessions", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(sessionsHandler.Start))))
	mux.Handle("POST /api/sessions/{session_id}/end", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(sessionsHandler.End))))
	mux.Handle("POST /api/ingestion/scan", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(ingestionHandler.Scan))))
	mux.Handle("GET /api/processing/queue", RequireAuth(authHandler.manager, http.HandlerFunc(processingQueueHandler.Get)))
	mux.Handle("POST /api/photos/{photo_id}/retry-processing", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(photoRetryHandler.Retry))))
	mux.Handle("GET /api/quarantine", RequireAuth(authHandler.manager, http.HandlerFunc(quarantineHandler.List)))
	mux.Handle("GET /api/quarantine/{quarantine_id}/eligible-sessions", RequireAuth(authHandler.manager, http.HandlerFunc(quarantineHandler.EligibleSessions)))
	mux.Handle("POST /api/quarantine/{quarantine_id}/assign", RequireTrustedOrigin(RequireAuth(authHandler.manager, http.HandlerFunc(quarantineHandler.Assign))))
	mux.Handle("GET /events", RequireAuth(authHandler.manager, http.HandlerFunc(eventsHandler.Stream)))
	return WithCORS(WithActivityStore(mux, activityHandler.store))
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, HealthData{
		Service: "selfstudio-agent",
		Status:  "ok",
		Database: HealthComponentStatus{
			Status: "placeholder",
			Label:  "Database belum dikonfigurasi",
			Action: "Konfigurasi Supabase pada story database berikutnya.",
		},
		Worker: HealthComponentStatus{
			Status: "placeholder",
			Label:  "Worker siap sebagai placeholder",
			Action: "Queue worker akan aktif saat pipeline foto dibuat.",
		},
		Disk: HealthComponentStatus{
			Status: "placeholder",
			Label:  "Disk check belum aktif",
			Action: "Disk usage check akan ditambahkan sebelum event readiness.",
		},
	})
}

type activityStoreContextKey struct{}

func WithActivityStore(next http.Handler, store *activity.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(contextWithActivityStore(r, store)))
	})
}

func contextWithActivityStore(r *http.Request, store *activity.Store) context.Context {
	return context.WithValue(r.Context(), activityStoreContextKey{}, store)
}
