package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/api"
	"selfstudio/agent/internal/auth"
	"selfstudio/agent/internal/cameras"
	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/config"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/ingestion"
	"selfstudio/agent/internal/photos"
	"selfstudio/agent/internal/processing"
	"selfstudio/agent/internal/quarantine"
	"selfstudio/agent/internal/recovery"
	"selfstudio/agent/internal/sessions"
	"selfstudio/agent/internal/stations"
	"selfstudio/agent/internal/upload"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	authManager, err := auth.NewManager(cfg.AuthPIN)
	if err != nil {
		log.Fatalf("auth configuration error: %v", err)
	}

	activityStore := activity.NewStore(activity.DefaultMaxEntries)
	eventBroker := events.NewBroker()
	stationPersistence := stations.NewPersistence(cfg.LocalDataDir)
	stationStore, err := stationPersistence.LoadOrDefault()
	if err != nil {
		log.Fatalf("station config error: %v", err)
	}
	sessionPersistence := sessions.NewPersistence(cfg.LocalDataDir)
	sessionStore, err := sessionPersistence.LoadOrDefault()
	if err != nil {
		log.Fatalf("session state error: %v", err)
	}

	photoPersistence := photos.NewPersistence(cfg.LocalDataDir)
	photoStore, err := photoPersistence.LoadOrDefault()
	if err != nil {
		log.Fatalf("photo state error: %v", err)
	}
	quarantinePersistence := quarantine.NewPersistence(cfg.LocalDataDir)
	quarantineStore, err := quarantinePersistence.LoadOrDefault()
	if err != nil {
		log.Fatalf("quarantine state error: %v", err)
	}
	ingestionScanner := ingestion.NewScanner(stationStore)
	photoRouter := ingestion.NewRouterWithQuarantine(sessionStore, photoStore, quarantineStore)
	originalSaver := &processing.OriginalSaver{Photos: photoStore, Sessions: sessionStore}
	gradedProcessor := &processing.GradedProcessor{Photos: photoStore, Sessions: sessionStore, Processor: processing.ImageMagickLUTProcessor{}}
	processingGuard := processing.NewProcessingGuard()
	recoverySummary := recovery.NewService(ingestionScanner, photoRouter, activityStore, eventBroker).Run(context.Background(), time.Now().UTC())
	if err := photoPersistence.Save(photoStore); err != nil {
		log.Fatalf("photo state save error: %v", err)
	}
	if err := quarantinePersistence.Save(quarantineStore); err != nil {
		log.Fatalf("quarantine state save error: %v", err)
	}
	log.Printf("ingestion recovery complete: routed=%d quarantine=%d duplicates=%d conflicts=%d errors=%d", recoverySummary.RecoveredRoutedPhotos, recoverySummary.RecoveredQuarantineItems, recoverySummary.SkippedDuplicates, recoverySummary.UnresolvedConflicts, recoverySummary.Errors)

	saveRuntimeState := func() error {
		return saveRuntimeStateTransactional(photoPersistence, photoStore, quarantinePersistence, quarantineStore)
	}
	originalSaver.Persist = saveRuntimeState
	gradedProcessor.Persist = saveRuntimeState
	processingRecovery := processing.StartupRecovery{OriginalSaver: originalSaver, Processor: gradedProcessor, Activity: activityStore, Broker: eventBroker}

	rollbackRuntimeState := func() error {
		reloadedPhotos, photoErr := photoPersistence.LoadOrDefault()
		reloadedQuarantine, quarantineErr := quarantinePersistence.LoadOrDefault()
		if photoErr != nil {
			return photoErr
		}
		if quarantineErr != nil {
			return quarantineErr
		}
		if err := photoStore.ReplaceAll(reloadedPhotos.ListAll()); err != nil {
			return err
		}
		return quarantineStore.ReplaceAll(reloadedQuarantine.ListAll())
	}

	cloudPersistence := cloud.NewPersistence(cfg.LocalDataDir)
	uploadPersistence := upload.NewPersistence(cfg.LocalDataDir)
	uploadStore, err := uploadPersistence.LoadOrDefault()
	if err != nil {
		log.Fatalf("upload target state error: %v", err)
	}
	uploadResolver := upload.Resolver{CloudStore: cloudPersistence, Targets: uploadStore, Persistence: uploadPersistence}
	jobsPersistence := upload.NewJobsPersistence(cfg.LocalDataDir)
	jobsStore, err := jobsPersistence.LoadOrDefault()
	if err != nil {
		log.Fatalf("upload jobs state error: %v", err)
	}
	driveClientFactory := func(ctx context.Context) (upload.DriveFileClient, error) {
		cloudSettings, err := cloudPersistence.LoadOrDefault()
		if err != nil {
			return nil, err
		}
		client, err := upload.NewGoogleDriveFileClient(ctx, cloudSettings)
		if err != nil {
			return nil, err
		}
		return client, nil
	}
	driveUploader := upload.DriveUploader{Factory: driveClientFactory}
	uploadRecoveryMu := &sync.Mutex{}
	uploadWorker := &upload.Worker{Sessions: sessionStore, Photos: photoStore, Targets: uploadStore, Jobs: jobsStore, Persistence: jobsPersistence, DriveUploader: driveUploader, FolderClient: driveUploader, Events: make(chan upload.FileUploadJob, 64), RecoveryMu: uploadRecoveryMu}
	uploadRecovery := upload.StartupRecovery{Worker: uploadWorker, Activity: activityStore, Broker: eventBroker}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			uploadWorker.AutoRetryDue(context.Background(), now.UTC())
		}
	}()

	discoveryService := cameras.NewDiscoveryService(nil)
	tetherSupervisor := cameras.NewTetherSupervisor(nil)
	tetherStateStore := cameras.NewTetherStateStore(cfg.LocalDataDir)
	tetherRecovery := cameras.NewTetherRecoveryCoordinator(tetherSupervisor, tetherStateStore, func(stationID string) (cameras.TetherStationConfig, bool) {
		station, err := stationStore.Get(stationID)
		if err != nil {
			return cameras.TetherStationConfig{}, false
		}
		var assignment *cameras.TetherAssignment
		if station.CameraAssignment != nil {
			assignment = &cameras.TetherAssignment{IdentityKey: station.CameraAssignment.IdentityKey, CameraName: station.CameraAssignment.CameraName, Port: station.CameraAssignment.Port, Runtime: cameras.Runtime(station.CameraAssignment.Runtime)}
		}
		return cameras.TetherStationConfig{StationID: station.StationID, InputFolder: station.InputFolder, Assignment: assignment}, true
	}, api.TetherRecoveryNotifier{Activity: activityStore, Broker: eventBroker})
	tetherSupervisor.SetUnexpectedExitHandler(tetherRecovery.OnUnexpectedExit)
	testCaptureService := cameras.NewTestCaptureService(nil)
	cameraValidator := stations.NewReadinessValidatorWithCamera(filepath.Join(cfg.LocalDataDir, "output"), stations.CameraReadinessOptions{Required: cfg.CameraReadinessRequired, Discovery: cameras.DiscoveryReadinessAdapter{Service: discoveryService}, Tether: cameras.TetherReadinessAdapter{Supervisor: tetherSupervisor}, TestCaptureResults: testCaptureService})

	server := &http.Server{
		Addr: cfg.Address(),
		Handler: api.NewMuxWithUploadsAndCamera(
			api.NewAuthHandlerWithActivity(authManager, activityStore),
			api.NewEventsHandler(eventBroker),
			api.NewActivityHandler(activityStore),
			api.NewPersistentStationsHandler(stationStore, stationPersistence, activityStore, eventBroker),
			api.NewReadinessHandlerWithValidator(stationStore, activityStore, eventBroker, cameraValidator),
			api.NewEventReadinessHandlerWithValidator(stationStore, activityStore, eventBroker, filepath.Join(cfg.LocalDataDir, "output"), cameraValidator),
			api.NewStationConfigHandler(stationStore, stationPersistence, activityStore, eventBroker),
			api.NewWatchValidationHandler(stationStore, activityStore, eventBroker),
			api.NewSessionsHandlerWithPhotosQuarantineUploadJobs(stationStore, sessionStore, sessionPersistence, activityStore, eventBroker, filepath.Join(cfg.LocalDataDir, "output"), photoStore, quarantineStore, uploadStore, jobsStore).WithReadinessValidator(cameraValidator),
			api.NewPersistentIngestionHandlerWithProcessingGuard(ingestionScanner, photoRouter, activityStore, eventBroker, saveRuntimeState, rollbackRuntimeState, originalSaver, gradedProcessor, processingGuard),
			api.NewPersistentQuarantineHandlerWithProcessingGuard(quarantineStore, sessionStore, photoStore, activityStore, eventBroker, saveRuntimeState, rollbackRuntimeState, originalSaver, gradedProcessor, processingGuard),
			api.NewProcessingQueueHandler(photoStore),
			api.NewPhotoRetryHandler(photoStore, gradedProcessor, activityStore, eventBroker, processingGuard),
			api.NewCloudSettingsHandler(cloudPersistence, cloud.GoogleDriveChecker{}, activityStore, eventBroker),
			api.NewCloudTargetHandler(sessionStore, uploadResolver, uploadStore, activityStore, eventBroker),
			api.NewSessionUploadsHandler(sessionStore, uploadStore, jobsStore, uploadWorker, activityStore, eventBroker),
			discoveryService,
			tetherSupervisor,
			testCaptureService,
			tetherStateStore,
			tetherRecovery,
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
	}

	listener, err := net.Listen("tcp", cfg.Address())
	if err != nil {
		log.Fatalf("agent server failed: %v", err)
	}

	go tetherRecovery.StartupRecover()

	fmt.Printf("Selfstudio agent listening on http://%s\n", cfg.Address())
	fmt.Printf("Health endpoint: http://%s/health\n", cfg.Address())

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("agent server failed: %v", err)
		}
	}()

	recoveryResult := processingRecovery.Recover()
	log.Printf("processing recovery complete: verified=%d resumed=%d failed=%d skipped_retry_limit=%d errors=%d", recoveryResult.Summary.Verified, recoveryResult.Summary.Resumed, recoveryResult.Summary.Failed, recoveryResult.Summary.SkippedRetryLimit, recoveryResult.Summary.Errors)
	for _, photoID := range recoveryResult.EnqueueIDs {
		api.EnqueueProcessing(gradedProcessor, processingGuard, activityStore, eventBroker, photoID, false)
	}
	go func() {
		uploadRecoveryResult, err := uploadRecovery.Recover(context.Background())
		if err != nil {
			log.Printf("upload recovery failed: %v", err)
		} else {
			log.Printf("upload recovery complete: recovered_pending=%d resumed=%d failed_missing_local=%d verified_uploaded=%d unverified_uploaded=%d requires_cloud_check=%d errors=%d", uploadRecoveryResult.Summary.RecoveredPending, uploadRecoveryResult.Summary.Resumed, uploadRecoveryResult.Summary.FailedMissingLocal, uploadRecoveryResult.Summary.VerifiedUploaded, uploadRecoveryResult.Summary.UnverifiedUploaded, uploadRecoveryResult.Summary.RequiresCloudCheck, uploadRecoveryResult.Summary.Errors)
		}
	}()

	select {}
}
