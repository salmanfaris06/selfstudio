import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  type Session,
  type Station,
  type StationReadiness,
} from "@/lib/api/client";
import { useSessionsQuery } from "./use-sessions-query";
import { useStationsQuery } from "@/features/stations/use-stations-query";
import { useStationReadinessQuery } from "@/features/stations/use-station-readiness-query";
import { Button } from "@/components/ui/button";
import { useEndSessionMutation } from "./use-end-session-mutation";
import { useSessionDetailQuery } from "./use-session-detail-query";
import { useStationQuarantineSummaryQuery } from "./use-station-quarantine-summary-query";
import { useResolveCloudTargetMutation } from "./use-resolve-cloud-target-mutation";
import { useStartCloudUploadMutation } from "./use-start-cloud-upload-mutation";
import { useRetryCloudUploadMutation } from "./use-retry-cloud-upload-mutation";
import { CameraTetherStatus } from "./camera-tether-status";

const stationIds = ["station_1", "station_2", "station_3"] as const;

export function LiveStationCards() {
  const stationsQuery = useStationsQuery();
  const sessionsQuery = useSessionsQuery();

  if (stationsQuery.isLoading || sessionsQuery.isLoading)
    return <p>Memuat live station cards...</p>;
  if (stationsQuery.isError)
    return (
      <p className="error-message">
        Station gagal dimuat. Refresh dashboard lalu coba lagi.
      </p>
    );
  if (sessionsQuery.isError)
    return (
      <p className="error-message">
        Session gagal dimuat. Pastikan Go agent berjalan lalu refresh.
      </p>
    );

  const sessions = sessionsQuery.data?.sessions ?? [];
  const stations = stationsQuery.data?.stations ?? [];
  return (
    <section
      className="live-station-section"
      aria-label="Live camera station cards"
    >
      <div className="section-heading">
        <div>
          <p className="eyebrow">Live Control Room</p>
          <h2>Three station live cards</h2>
          <p>
            Monitor status station, session aktif, readiness, jumlah foto, dan
            alert quarantine.
          </p>
          {sessionsQuery.data?.recovered ? (
            <p className="success-message">
              Session state dipulihkan dari local durable state setelah
              refresh/restart.
            </p>
          ) : null}
        </div>
      </div>
      <div className="station-grid">
        {stationIds.map((stationId) => (
          <LiveStationCard
            key={stationId}
            station={stations.find((item) => item.station_id === stationId)}
            session={selectStationSession(sessions, stationId)}
          />
        ))}
      </div>
    </section>
  );
}

function LiveStationCard({
  station,
  session,
}: {
  station?: Station;
  session?: Session;
}) {
  const readinessQuery = useStationReadinessQuery(
    station?.station_id ?? "station_1",
  );
  const endMutation = useEndSessionMutation(
    station?.station_id ?? "station_1",
    session?.session_id ?? "",
  );
  const detailQuery = useSessionDetailQuery(session?.session_id ?? null);
  const quarantineSummaryQuery = useStationQuarantineSummaryQuery(
    station?.station_id ?? null,
  );
  const cloudTargetMutation = useResolveCloudTargetMutation(
    session?.session_id ?? "",
  );
  const cloudUploadMutation = useStartCloudUploadMutation(
    session?.session_id ?? "",
  );
  const retryCloudUploadMutation = useRetryCloudUploadMutation(
    session?.session_id ?? "",
  );
  const readiness = station ? readinessQuery.data?.readiness : undefined;
  const isActiveSession = session?.status === "active";
  const isLockedSession = session?.status === "locked";
  const stationQuarantineCount = session
    ? (detailQuery.data?.summary.station_quarantine_count ??
      quarantineSummaryQuery.data?.summary.station_quarantine_count ??
      0)
    : (quarantineSummaryQuery.data?.summary.station_quarantine_count ?? 0);
  const latestQuarantineReason = session
    ? (detailQuery.data?.summary.latest_quarantine_reason ??
      quarantineSummaryQuery.data?.summary.latest_quarantine_reason)
    : quarantineSummaryQuery.data?.summary.latest_quarantine_reason;
  const uploadStatus =
    detailQuery.data?.summary.upload_status ?? "not_configured";
  const driveTargetStatus =
    detailQuery.data?.summary.drive_target_status ?? "pending";
  const canRetryCloudUpload =
    isLockedSession &&
    (uploadStatus === "failed" || uploadStatus === "partial_failed");
  return (
    <Card>
      <CardHeader>
        <div className="status-card-heading">
          <CardTitle>{station?.name ?? "Station belum tersedia"}</CardTitle>
          <Badge>{station?.station_id ?? "missing"}</Badge>
        </div>
        <CardDescription>{statusLabel(readiness, session)}</CardDescription>
      </CardHeader>
      <CardContent>
        {station ? (
          <CameraTetherStatus station={station} readiness={readiness} />
        ) : null}
        <dl className="health-list">
          <div>
            <dt>Status</dt>
            <dd>
              {session
                ? sessionStatusLabel(session.status)
                : (readiness?.status.toUpperCase() ?? "UNKNOWN")}
            </dd>
          </div>
          <div>
            <dt>Customer</dt>
            <dd>
              {session?.customer_name ?? "Belum ada session aktif/locked"}
            </dd>
          </div>
          <div>
            <dt>Order</dt>
            <dd>{session?.order_number ?? "-"}</dd>
          </div>
          <div>
            <dt>Timer</dt>
            <dd>
              {isActiveSession
                ? `${Math.max(0, Math.floor((new Date(session.ends_at).getTime() - Date.now()) / 1000))} detik tersisa`
                : isLockedSession
                  ? "Session locked - Drive folder bisa di-resolve"
                  : "-"}
            </dd>
          </div>
          <div>
            <dt>LUT</dt>
            <dd>{station?.default_lut_path ?? "-"}</dd>
          </div>
          <div>
            <dt>Background</dt>
            <dd>{station?.background_name ?? "-"}</dd>
          </div>
          <div>
            <dt>Photo count</dt>
            <dd>{detailQuery.data?.summary.photo_count ?? 0} routed foto</dd>
          </div>
          <div>
            <dt>Station quarantine</dt>
            <dd>
              {stationQuarantineCount} quarantine •{" "}
              {quarantineReasonLabel(latestQuarantineReason)}
            </dd>
          </div>
        </dl>
        {session ? (
          <div className="session-detail-summary">
            <p>
              Local output:{" "}
              {detailQuery.data?.summary.local_output_folder ??
                session.station_snapshot.output_folder}
            </p>
            <p>
              Summary: {detailQuery.data?.summary.photo_count ?? 0} foto routed,{" "}
              {detailQuery.data?.summary.failures ?? 0} failure,{" "}
              {detailQuery.data?.summary.quarantine_count ?? 0} quarantine
              terkait session, {stationQuarantineCount} quarantine station,
              latest reason {quarantineReasonLabel(latestQuarantineReason)}
            </p>
            <p>
              Drive folder target: {driveTargetStatusLabel(driveTargetStatus)}
              {detailQuery.data?.summary.drive_session_folder_id
                ? ` • ID ${detailQuery.data.summary.drive_session_folder_id}`
                : ""}
              {detailQuery.data?.summary.drive_folder_path
                ? ` • path ${detailQuery.data.summary.drive_folder_path}`
                : ""}
            </p>
            <p>
              Google Drive upload:{" "}
              {cloudStatusLabel(
                detailQuery.data?.summary.upload_status ?? "not_configured",
              )}
              . Status upload ini terpisah dari local save/processing dan Drive folder target.
            </p>
            <p>
              Retry Drive Upload: retry count dan next action berasal dari server; gunakan tombol retry hanya saat status failed/partial_failed. Saat retry berjalan, label tombol berubah menjadi “Retrying Drive Upload…”.
            </p>
            {detailQuery.data?.photos?.[0] ? (
              <p>
                Foto terakhir: {detailQuery.data.photos[0].status} •{" "}
                {detailQuery.data.photos[0].source_size_bytes} bytes •{" "}
                {new Date(
                  detailQuery.data.photos[0].routed_at,
                ).toLocaleTimeString("id-ID")}
              </p>
            ) : (
              <p>Belum ada foto routed untuk session ini.</p>
            )}
          </div>
        ) : (
          <div className="session-detail-summary">
            <p>
              Alert quarantine station: {stationQuarantineCount} foto
              dikarantina pada station ini.
            </p>
            <p>
              Latest reason: {quarantineReasonLabel(latestQuarantineReason)}
            </p>
            <p>
              {stationQuarantineCount > 0
                ? "Ada quarantine nyata dari data agent; cek activity log untuk detail aman."
                : "Belum ada quarantine tercatat untuk station ini."}
            </p>
          </div>
        )}
        {session ? (
          <div className="button-row">
            {isActiveSession ? (
              <Button
                disabled={endMutation.isPending}
                onClick={() => {
                  if (
                    window.confirm(
                      "Akhiri session aktif ini? Foto setelah lock akan dianggap late/unassigned pada story ingestion.",
                    )
                  )
                    void endMutation.mutateAsync();
                }}
                type="button"
              >
                {endMutation.isPending ? "Mengakhiri..." : "End session"}
              </Button>
            ) : null}
            <Button
              disabled={!isLockedSession || cloudTargetMutation.isPending}
              onClick={() => void cloudTargetMutation.mutateAsync()}
              type="button"
            >
              {cloudTargetMutation.isPending ? "Resolving Drive Folder..." : driveTargetStatus === "failed" ? "Retry Drive Folder" : "Resolve Drive Folder"}
            </Button>
            <Button
              disabled={
                !isLockedSession ||
                cloudUploadMutation.isPending ||
                uploadStatus === "not_configured" ||
                uploadStatus === "target_pending" ||
                uploadStatus === "pending_local_completion"
              }
              onClick={() => void cloudUploadMutation.mutateAsync()}
              type="button"
            >
              {cloudUploadMutation.isPending
                ? "Starting Drive Upload..."
                : "Start Drive Upload"}
            </Button>
            <Button
              disabled={
                !canRetryCloudUpload || retryCloudUploadMutation.isPending
              }
              onClick={() => void retryCloudUploadMutation.mutateAsync()}
              type="button"
            >
              {retryCloudUploadMutation.isPending
                ? "Retrying Drive Upload..."
                : "Retry Drive Upload"}
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

function selectStationSession(sessions: Session[], stationId: string) {
  return (
    sessions.find(
      (item) => item.station_id === stationId && item.status === "active",
    ) ??
    sessions.find(
      (item) => item.station_id === stationId && item.status === "locked",
    )
  );
}

function statusLabel(readiness?: StationReadiness, session?: Session) {
  if (session?.status === "active")
    return "Session aktif - JPG stabil akan diroute oleh server ke session ini.";
  if (session?.status === "locked")
    return "Session locked - local capture selesai; Drive folder dapat di-resolve/retry tanpa mencampur status local processing.";
  if (!readiness) return "Readiness belum dimuat.";
  return `${readiness.label} ${readiness.action}`;
}

function sessionStatusLabel(status: Session["status"]) {
  if (status === "active") return "LIVE";
  return "LOCKED";
}

function driveTargetStatusLabel(status: string) {
  if (status === "ready") return "Drive folder ready";
  if (status === "failed") return "Drive folder failed";
  if (status === "not_configured") return "Drive not configured";
  if (status === "resolving") return "Drive folder resolving";
  return "Drive folder pending";
}

function cloudStatusLabel(status: string) {
  if (status === "uploaded")
    return "uploaded - semua file eligible sudah ter-upload";
  if (status === "uploading")
    return "uploading - worker background sedang upload/retry";
  if (status === "partial_failed")
    return "partial_failed - sebagian file gagal, local tetap aman";
  if (status === "failed") return "failed - cek action aman lalu retry upload";
  if (status === "pending") return "pending - job upload siap/menunggu start";
  if (status === "target_pending")
    return "target_pending - resolve Drive folder dulu";
  if (status === "pending_local_completion")
    return "pending_local_completion - tunggu local save/processing selesai";
  if (status === "not_configured")
    return "not_configured - Google Drive belum dikonfigurasi";
  return "placeholder - service Google Drive upload belum tersambung";
}

function quarantineReasonLabel(reason?: string) {
  if (reason === "late_photo")
    return "late_photo (foto datang setelah session locked/timer expired)";
  if (reason === "no_active_session")
    return "no_active_session (tidak ada session aktif yang eligible)";
  return "belum ada quarantine";
}
