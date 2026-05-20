"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { ActivityLog } from "@/features/activity/activity-log";
import { CloudSettingsPanel } from "@/features/cloud/cloud-settings";
import { IngestionScanPanel } from "@/features/ingestion/ingestion-scan-panel";
import { ProcessingQueuePanel } from "@/features/processing/processing-queue-panel";
import { processingQueueQueryKey } from "@/features/processing/use-processing-queue-query";
import { QuarantineReview } from "@/features/quarantine/quarantine-review";
import { quarantineQueryKey } from "@/features/quarantine/use-quarantine-query";
import { LiveStationCards } from "@/features/sessions/live-station-cards";
import { activityQueryKey } from "@/features/activity/use-activity-query";
import { sessionsQueryKey } from "@/features/sessions/use-sessions-query";
import { stationQuarantineSummaryQueryKey } from "@/features/sessions/use-station-quarantine-summary-query";
import { EventReadinessChecklist } from "@/features/readiness/event-readiness-checklist";
import { StationSettings } from "@/features/stations/station-settings";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ApiError, getApiBaseUrl, type HealthComponentStatus, type HealthData } from "@/lib/api/client";
import { createEventStream, parseEventEnvelope } from "@/lib/events/client";
import { eventReadinessQueryKey } from "@/features/readiness/use-event-readiness-query";
import { stationReadinessQueryKey } from "@/features/stations/use-station-readiness-query";
import { tetherListenerQueryKey } from "@/features/stations/use-tether-listener";
import { stationsQueryKey } from "@/features/stations/use-stations-query";
import { healthQueryKey, useHealthQuery } from "./use-health-query";

type EventStreamState = "connecting" | "open" | "error" | "closed";

type HealthDashboardProps = {
  onLogout: () => Promise<void>;
  onAuthExpired: () => Promise<void>;
  logoutDisabled: boolean;
  authErrorMessage: string | null;
};

const stationIds = ["station_1", "station_2", "station_3"] as const;

const eventStreamLabels: Record<EventStreamState, HealthComponentStatus> = {
  connecting: {
    status: "unknown",
    label: "Event stream sedang menghubungkan",
    action: "Dashboard tetap bisa dipakai lewat refresh data berkala.",
  },
  open: {
    status: "ok",
    label: "Event stream terhubung",
    action: "Health update real-time akan memicu refresh dashboard.",
  },
  error: {
    status: "warning",
    label: "Event stream belum stabil",
    action: "Refresh dashboard atau pastikan Go agent masih berjalan.",
  },
  closed: {
    status: "unknown",
    label: "Event stream ditutup",
    action: "Buka ulang dashboard jika update real-time dibutuhkan.",
  },
};

export function HealthDashboard({ onLogout, logoutDisabled, authErrorMessage }: HealthDashboardProps) {
  const queryClient = useQueryClient();
  const healthQuery = useHealthQuery();
  const [eventStreamState, setEventStreamState] = useState<EventStreamState>("connecting");

  useEffect(() => {
    let stream: EventSource;

    try {
      stream = createEventStream();
    } catch {
      setEventStreamState("error");
      return;
    }

    const handleOpen = () => setEventStreamState("open");
    const handleError = () => {
      setEventStreamState("error");
    };
    const handleHealthUpdated = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        if (event.event_type !== "health.updated" || event.entity_type !== "service") {
          return;
        }
        void queryClient.invalidateQueries({ queryKey: healthQueryKey });
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: healthQueryKey });
      }
    };

    const handleStationUpdated = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        if ((event.entity_type !== "station" || (event.event_type !== "station.updated" && event.event_type !== "station.readiness_checked" && event.event_type !== "station.validation_completed" && event.event_type !== "station.health_refreshed" && event.event_type !== "station.camera_test_capture_completed")) && (event.entity_type !== "station_config" || event.event_type !== "stations.restored")) {
          return;
        }
        void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
        void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
        if (event.event_type === "stations.restored" || event.entity_id === "all") {
          for (const stationId of stationIds) {
            void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
          }
          return;
        }
        if (event.entity_id) {
          void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(event.entity_id) });
        }
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
        void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      }
    };

    const invalidatePhotoAffectedQueries = (event: ReturnType<typeof parseEventEnvelope>) => {
      void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
      void queryClient.invalidateQueries({ queryKey: quarantineQueryKey });
      void queryClient.invalidateQueries({ predicate: (query) => Array.isArray(query.queryKey) && query.queryKey[0] === "sessions" });
      void queryClient.invalidateQueries({ queryKey: processingQueueQueryKey });
      const stationId = typeof event.data?.station_id === "string" ? event.data.station_id : null;
      if (stationId) {
        void queryClient.invalidateQueries({ queryKey: stationQuarantineSummaryQueryKey(stationId) });
      }
    };

    const handlePhotoDetected = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        const isDetected = event.event_type === "photo.detected" && event.entity_type === "photo";
        const isQuarantined = event.event_type === "photo.quarantined" && event.entity_type === "quarantine";
        const isRouted = event.event_type === "photo.routed" && event.entity_type === "photo";
        const isAssigned = event.event_type === "quarantine.assigned" && event.entity_type === "quarantine";
        const isRecovered = event.event_type === "ingestion.recovered" && event.entity_type === "ingestion";
        const isProcessingRecovered = event.event_type === "processing.recovered" && event.entity_type === "startup";
        const isCloudStatusUpdated = event.event_type === "cloud.status_updated" && event.entity_type === "cloud";
        const isQueueUpdated = event.event_type === "queue.updated" && (event.entity_type === "photo" || event.entity_type === "startup");
        const isProcessingEvent = (event.event_type === "photo.processing_started" || event.event_type === "photo.processed" || event.event_type === "photo.processing_failed") && event.entity_type === "photo";
        if (!isDetected && !isQuarantined && !isRouted && !isAssigned && !isRecovered && !isProcessingRecovered && !isCloudStatusUpdated && !isQueueUpdated && !isProcessingEvent) {
          return;
        }
        invalidatePhotoAffectedQueries(event);
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
        void queryClient.invalidateQueries({ predicate: (query) => Array.isArray(query.queryKey) && (query.queryKey[0] === "sessions" || query.queryKey[0] === "stations") });
      }
    };

    const handleCloudTargetUpdated = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        if ((event.event_type !== "cloud.target_resolved" && event.event_type !== "cloud.target_failed") || event.entity_type !== "cloud") {
          return;
        }
        void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
        void queryClient.invalidateQueries({ predicate: (query) => Array.isArray(query.queryKey) && query.queryKey[0] === "sessions" });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      }
    };

    const handleUploadUpdated = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        if (!event.event_type.startsWith("upload.") || event.entity_type !== "upload") {
          return;
        }
        void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
        void queryClient.invalidateQueries({ predicate: (query) => Array.isArray(query.queryKey) && (query.queryKey[0] === "session-detail" || query.queryKey[0] === "session-uploads") });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      }
    };

    const handleSessionStarted = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        if ((event.event_type !== "session.started" && event.event_type !== "session.ended") || event.entity_type !== "session") {
          return;
        }
        void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
        void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
        void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
        const stationId = typeof event.data?.session === "object" && event.data.session && "station_id" in event.data.session && typeof event.data.session.station_id === "string" ? event.data.session.station_id : null;
        if (stationId) {
          void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
        }
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: sessionsQueryKey });
      }
    };

    const handleReadinessChecked = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        if (event.event_type !== "readiness.checked" || event.entity_type !== "readiness") {
          return;
        }
        void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
        void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
        void queryClient.invalidateQueries({ queryKey: ["stations"] });
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: eventReadinessQueryKey });
      }
    };

    const handleTetherListenerUpdated = (message: MessageEvent<string>) => {
      try {
        const event = parseEventEnvelope(message);
        if ((event.event_type !== "camera.tether_listener_updated" && event.event_type !== "camera.tether_recovery_updated") || event.entity_type !== "station") {
          return;
        }
        const stationId = event.entity_id;
        if (stationId) {
          void queryClient.invalidateQueries({ queryKey: tetherListenerQueryKey(stationId) });
          void queryClient.invalidateQueries({ queryKey: stationReadinessQueryKey(stationId) });
        }
        void queryClient.invalidateQueries({ queryKey: stationsQueryKey });
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      } catch {
        setEventStreamState("error");
        void queryClient.invalidateQueries({ queryKey: activityQueryKey("") });
      }
    };

    stream.addEventListener("open", handleOpen);
    stream.addEventListener("error", handleError);
    stream.addEventListener("health.updated", handleHealthUpdated as EventListener);
    stream.addEventListener("station.updated", handleStationUpdated as EventListener);
    stream.addEventListener("station.readiness_checked", handleStationUpdated as EventListener);
    stream.addEventListener("stations.restored", handleStationUpdated as EventListener);
    stream.addEventListener("station.validation_completed", handleStationUpdated as EventListener);
    stream.addEventListener("station.camera_test_capture_completed", handleStationUpdated as EventListener);
    stream.addEventListener("station.health_refreshed", handleStationUpdated as EventListener);
    stream.addEventListener("session.started", handleSessionStarted as EventListener);
    stream.addEventListener("session.ended", handleSessionStarted as EventListener);
    stream.addEventListener("photo.detected", handlePhotoDetected as EventListener);
    stream.addEventListener("photo.quarantined", handlePhotoDetected as EventListener);
    stream.addEventListener("photo.routed", handlePhotoDetected as EventListener);
    stream.addEventListener("quarantine.assigned", handlePhotoDetected as EventListener);
    stream.addEventListener("ingestion.recovered", handlePhotoDetected as EventListener);
    stream.addEventListener("processing.recovered", handlePhotoDetected as EventListener);
    stream.addEventListener("cloud.status_updated", handlePhotoDetected as EventListener);
    stream.addEventListener("cloud.target_resolved", handleCloudTargetUpdated as EventListener);
    stream.addEventListener("cloud.target_failed", handleCloudTargetUpdated as EventListener);
    stream.addEventListener("upload.started", handleUploadUpdated as EventListener);
    stream.addEventListener("upload.file_uploaded", handleUploadUpdated as EventListener);
    stream.addEventListener("upload.file_failed", handleUploadUpdated as EventListener);
    stream.addEventListener("upload.session_updated", handleUploadUpdated as EventListener);
    stream.addEventListener("upload.recovered", handleUploadUpdated as EventListener);
    stream.addEventListener("queue.updated", handlePhotoDetected as EventListener);
    stream.addEventListener("photo.processing_started", handlePhotoDetected as EventListener);
    stream.addEventListener("photo.processed", handlePhotoDetected as EventListener);
    stream.addEventListener("photo.processing_failed", handlePhotoDetected as EventListener);
    stream.addEventListener("readiness.checked", handleReadinessChecked as EventListener);
    stream.addEventListener("camera.tether_listener_updated", handleTetherListenerUpdated as EventListener);
    stream.addEventListener("camera.tether_recovery_updated", handleTetherListenerUpdated as EventListener);

    return () => {
      stream.removeEventListener("open", handleOpen);
      stream.removeEventListener("error", handleError);
      stream.removeEventListener("health.updated", handleHealthUpdated as EventListener);
      stream.removeEventListener("station.updated", handleStationUpdated as EventListener);
      stream.removeEventListener("station.readiness_checked", handleStationUpdated as EventListener);
      stream.removeEventListener("stations.restored", handleStationUpdated as EventListener);
      stream.removeEventListener("station.validation_completed", handleStationUpdated as EventListener);
      stream.removeEventListener("station.camera_test_capture_completed", handleStationUpdated as EventListener);
      stream.removeEventListener("cloud.target_resolved", handleCloudTargetUpdated as EventListener);
      stream.removeEventListener("cloud.target_failed", handleCloudTargetUpdated as EventListener);
      stream.removeEventListener("upload.started", handleUploadUpdated as EventListener);
      stream.removeEventListener("upload.file_uploaded", handleUploadUpdated as EventListener);
      stream.removeEventListener("upload.file_failed", handleUploadUpdated as EventListener);
      stream.removeEventListener("upload.session_updated", handleUploadUpdated as EventListener);
      stream.removeEventListener("upload.recovered", handleUploadUpdated as EventListener);
      stream.removeEventListener("station.health_refreshed", handleStationUpdated as EventListener);
      stream.removeEventListener("session.started", handleSessionStarted as EventListener);
      stream.removeEventListener("session.ended", handleSessionStarted as EventListener);
      stream.removeEventListener("photo.detected", handlePhotoDetected as EventListener);
      stream.removeEventListener("photo.quarantined", handlePhotoDetected as EventListener);
      stream.removeEventListener("photo.routed", handlePhotoDetected as EventListener);
      stream.removeEventListener("quarantine.assigned", handlePhotoDetected as EventListener);
      stream.removeEventListener("ingestion.recovered", handlePhotoDetected as EventListener);
      stream.removeEventListener("processing.recovered", handlePhotoDetected as EventListener);
      stream.removeEventListener("cloud.status_updated", handlePhotoDetected as EventListener);
      stream.removeEventListener("queue.updated", handlePhotoDetected as EventListener);
      stream.removeEventListener("photo.processing_started", handlePhotoDetected as EventListener);
      stream.removeEventListener("photo.processed", handlePhotoDetected as EventListener);
      stream.removeEventListener("photo.processing_failed", handlePhotoDetected as EventListener);
      stream.removeEventListener("readiness.checked", handleReadinessChecked as EventListener);
      stream.removeEventListener("camera.tether_listener_updated", handleTetherListenerUpdated as EventListener);
      stream.removeEventListener("camera.tether_recovery_updated", handleTetherListenerUpdated as EventListener);
      stream.close();
    };
  }, [queryClient]);

  return (
    <main className="dashboard-shell">
      <header className="dashboard-header">
        <div>
          <p className="eyebrow">Selfstudio Local Admin</p>
          <h1>Health dashboard</h1>
          <p>Ringkasan kesiapan dasar aplikasi lokal sebelum operasional event berjalan.</p>
        </div>
        <Button className="secondary-button" disabled={logoutDisabled} onClick={onLogout}>
          Logout
        </Button>
      </header>

      {authErrorMessage ? <p className="error-message">{authErrorMessage}</p> : null}

      {healthQuery.isLoading ? (
        <Card aria-live="polite">
          <CardHeader>
            <CardTitle>Memuat health check</CardTitle>
            <CardDescription>Dashboard sedang mengambil status dari Go agent.</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      {healthQuery.isError ? (
        <Card aria-live="assertive">
          <CardHeader>
            <CardTitle>Health check gagal</CardTitle>
            <CardDescription>{formatError(healthQuery.error)}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button disabled={healthQuery.isFetching} onClick={() => void healthQuery.refetch()}>
              Coba health check lagi
            </Button>
          </CardContent>
        </Card>
      ) : null}

      <EventReadinessChecklist />

      {healthQuery.data ? (
        <section className="health-grid" aria-label="Indikator health aplikasi">
          <StatusCard
            title="Service health"
            status={{
              status: healthQuery.data.status,
              label: `Service ${healthQuery.data.service} ${healthQuery.data.status}`,
              action: "Jika status bukan ok, restart Go agent lalu cek ulang.",
            }}
          />
          <StatusCard title="Database reachability" status={healthQuery.data.database} />
          <StatusCard title="Worker placeholder" status={healthQuery.data.worker} />
          <StatusCard title="Disk placeholder" status={healthQuery.data.disk} />
          <StatusCard title="Event stream" status={eventStreamLabels[eventStreamState]} />
        </section>
      ) : null}

      <LiveStationCards />
      <ProcessingQueuePanel eventStreamState={eventStreamState} />
      <QuarantineReview />
      <IngestionScanPanel />
      <CloudSettingsPanel />
      <StationSettings />

      <section className="activity-section" aria-label="Activity log operator">
        <ActivityLog />
      </section>

      <footer className="dashboard-footer">
        <span>Agent API: {getApiBaseUrl()}</span>
        <span>Refresh: TanStack Query setiap 30 detik + SSE health.updated</span>
      </footer>
    </main>
  );
}

function StatusCard({ title, status }: { title: string; status: HealthComponentStatus }) {
  return (
    <Card>
      <CardHeader>
        <div className="status-card-heading">
          <CardTitle>{title}</CardTitle>
          <Badge>{status.status}</Badge>
        </div>
        <CardDescription>{status.label}</CardDescription>
      </CardHeader>
      <CardContent>
        <p>{status.action}</p>
      </CardContent>
    </Card>
  );
}

function formatError(error: Error): string {
  if (error instanceof ApiError) {
    return `${error.message} ${error.action}`;
  }

  return "Tidak bisa membaca health dashboard. Pastikan Go agent berjalan lalu coba lagi.";
}

export type { HealthData };
