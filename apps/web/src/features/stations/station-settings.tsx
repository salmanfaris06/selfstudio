"use client";

import { useEffect, useState } from "react";
import { ApiError, type DetectedGPhoto2Camera, type GPhoto2DiscoveryData, type Station, type StationReadiness, type StartSessionRequest, type UpdateStationRequest, type WatchValidation } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useStartSessionMutation } from "@/features/sessions/use-start-session-mutation";
import { useCameraAssignmentMutation } from "./use-camera-assignment-mutation";
import { useCameraTestCaptureMutation } from "./use-camera-test-capture-mutation";
import { useGPhoto2DiscoveryMutation } from "./use-gphoto2-discovery-mutation";
import { useRefreshStationHealthMutation } from "./use-refresh-station-health-mutation";
import { useRunReadinessCheckMutation } from "./use-run-readiness-check-mutation";
import { useRunWatchValidationMutation } from "./use-run-watch-validation-mutation";
import { useStationBackupMutation } from "./use-station-backup-mutation";
import { useStationReadinessQuery } from "./use-station-readiness-query";
import { useStationRestoreMutation } from "./use-station-restore-mutation";
import { useStationsQuery } from "./use-stations-query";
import { useStartTetherListenerMutation, useStopTetherListenerMutation, useTetherListenerQuery } from "./use-tether-listener";
import { useUpdateStationMutation } from "./use-update-station-mutation";

export function StationSettings() {
  const stationsQuery = useStationsQuery();
  const backupMutation = useStationBackupMutation();
  const restoreMutation = useStationRestoreMutation();
  const [restoreFilename, setRestoreFilename] = useState("");
  const [configMessage, setConfigMessage] = useState<{ kind: "success" | "error"; text: string } | null>(null);
  const [watchResults, setWatchResults] = useState<Record<string, WatchValidation>>({});
  const [discovery, setDiscovery] = useState<GPhoto2DiscoveryData | null>(null);
  const [discoveryMessage, setDiscoveryMessage] = useState<{ kind: "success" | "error"; text: string } | null>(null);
  const discoveryMutation = useGPhoto2DiscoveryMutation();

  async function handleBackup() {
    setConfigMessage(null);
    try {
      const data = await backupMutation.mutateAsync();
      setConfigMessage({ kind: "success", text: `Backup dibuat: ${data.backup.filename}` });
    } catch (error) {
      setConfigMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleRestore() {
    setConfigMessage(null);
    try {
      await restoreMutation.mutateAsync(restoreFilename);
      setConfigMessage({ kind: "success", text: "Station config berhasil direstore." });
    } catch (error) {
      setConfigMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleDiscovery() {
    setDiscoveryMessage(null);
    try {
      const data = await discoveryMutation.mutateAsync();
      setDiscovery(data);
      setDiscoveryMessage({ kind: data.status === "ready" ? "success" : "error", text: cameraActionText(data.action) });
    } catch (error) {
      setDiscoveryMessage({ kind: "error", text: formatError(error) });
    }
  }

  return (
    <section className="station-settings" aria-label="Konfigurasi camera station">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Station Setup</p>
          <h2>Camera station configuration</h2>
          <p>Atur tiga sumber foto sebelum readiness check dan session event dijalankan.</p>
        </div>
        <div className="station-config-actions">
          <Button disabled={stationsQuery.isFetching} onClick={() => void stationsQuery.refetch()}>
            Refresh stations
          </Button>
          <Button disabled={backupMutation.isPending} onClick={() => void handleBackup()}>
            {backupMutation.isPending ? "Membuat backup..." : "Backup config"}
          </Button>
        </div>
      </div>

      <div className="restore-panel">
        <p>Restore akan mengganti ketiga konfigurasi station. Gunakan filename backup dari folder local-data/config/backups.</p>
        <input value={restoreFilename} onChange={(event) => setRestoreFilename(event.target.value)} placeholder="stations-20260518-103000.json" />
        <Button disabled={restoreMutation.isPending || restoreFilename.trim() === ""} onClick={() => void handleRestore()}>
          {restoreMutation.isPending ? "Merestore..." : "Restore backup"}
        </Button>
      </div>
      {configMessage ? <p className={configMessage.kind === "success" ? "success-message" : "error-message"}>{configMessage.text}</p> : null}

      <GPhoto2DiscoveryPanel discovery={discovery} isPending={discoveryMutation.isPending} message={discoveryMessage} onDiscover={() => void handleDiscovery()} />

      {stationsQuery.isLoading ? <p>Memuat konfigurasi station...</p> : null}
      {stationsQuery.isError ? (
        <Card aria-live="assertive">
          <CardHeader>
            <CardTitle>Station config gagal dimuat</CardTitle>
            <CardDescription>{formatError(stationsQuery.error)}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button disabled={stationsQuery.isFetching} onClick={() => void stationsQuery.refetch()}>
              Coba lagi
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {stationsQuery.data ? (
        <div className="station-grid">
          {stationsQuery.data.stations.map((station) => (
            <StationForm key={station.station_id} station={station} discovery={discovery} validation={watchResults[station.station_id]} onValidationComplete={(result) => setWatchResults((current) => ({ ...current, [station.station_id]: result }))} />
          ))}
        </div>
      ) : null}
    </section>
  );
}

function StationForm({ station, discovery, validation, onValidationComplete }: { station: Station; discovery: GPhoto2DiscoveryData | null; validation?: WatchValidation; onValidationComplete: (result: WatchValidation) => void }) {
  const mutation = useUpdateStationMutation(station.station_id);
  const readinessQuery = useStationReadinessQuery(station.station_id);
  const readinessMutation = useRunReadinessCheckMutation(station.station_id);
  const healthRefreshMutation = useRefreshStationHealthMutation(station.station_id);
  const watchValidationMutation = useRunWatchValidationMutation(station.station_id);
  const startSessionMutation = useStartSessionMutation(station.station_id);
  const cameraAssignmentMutation = useCameraAssignmentMutation(station.station_id);
  const cameraTestCaptureMutation = useCameraTestCaptureMutation(station.station_id);
  const tetherQuery = useTetherListenerQuery(station.station_id);
  const startTetherMutation = useStartTetherListenerMutation(station.station_id);
  const stopTetherMutation = useStopTetherListenerMutation(station.station_id);
  const [form, setForm] = useState<UpdateStationRequest>(() => toForm(station));
  const [sessionForm, setSessionForm] = useState<StartSessionRequest>({ customer_name: "", order_number: "", timer_seconds: 900 });
  const [isDirty, setIsDirty] = useState(false);
  const [message, setMessage] = useState<{ kind: "success" | "error"; text: string } | null>(null);

  useEffect(() => {
    if (!isDirty && !mutation.isPending) {
      setForm(toForm(station));
    }
  }, [isDirty, mutation.isPending, station]);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage(null);
    try {
      await mutation.mutateAsync(form);
      setIsDirty(false);
      setMessage({ kind: "success", text: "Konfigurasi station berhasil disimpan." });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleHealthRefresh() {
    setMessage(null);
    try {
      const data = await healthRefreshMutation.mutateAsync();
      setMessage({ kind: data.readiness.status === "failed" ? "error" : "success", text: `${data.readiness.label} ${data.readiness.action}` });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleWatchValidation() {
    setMessage(null);
    try {
      const data = await watchValidationMutation.mutateAsync();
      onValidationComplete(data.validation);
      setMessage({ kind: data.validation.status === "success" ? "success" : "error", text: data.validation.label });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleReadinessCheck() {
    setMessage(null);
    try {
      await readinessMutation.mutateAsync();
      setMessage({ kind: "success", text: "Readiness station selesai diperiksa." });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleAssignCamera(identityKey: string) {
    const camera = discovery?.cameras.find((item) => item.identity_key === identityKey);
    if (!camera) return;
    setMessage(null);
    try {
      await cameraAssignmentMutation.mutateAsync({ identity_key: camera.identity_key, camera_name: camera.name, port: camera.port, runtime: camera.runtime, connected: camera.connected });
      setMessage({ kind: "success", text: `Kamera ${camera.name} tersimpan untuk ${station.station_id}.` });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleCameraTestCapture() {
    setMessage(null);
    try {
      const data = await cameraTestCaptureMutation.mutateAsync();
      setMessage({ kind: data.test_capture.status === "success" ? "success" : "error", text: `${data.test_capture.label}${data.test_capture.file_name ? ` (${data.test_capture.file_name})` : ""}` });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleStartTether() {
    setMessage(null);
    try {
      const data = await startTetherMutation.mutateAsync();
      setMessage({ kind: "success", text: data.listener.already_running ? "Tether listener sudah berjalan." : "Tether listener berhasil dimulai." });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleStopTether() {
    setMessage(null);
    try {
      await stopTetherMutation.mutateAsync();
      setMessage({ kind: "success", text: "Tether listener dihentikan untuk station ini." });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  async function handleStartSession() {
    setMessage(null);
    try {
      const data = await startSessionMutation.mutateAsync(sessionForm);
      setMessage({ kind: "success", text: `Session ${data.session.session_id} aktif sampai ${new Date(data.session.ends_at).toLocaleString()}.` });
    } catch (error) {
      setMessage({ kind: "error", text: formatError(error) });
    }
  }

  function updateField(field: keyof UpdateStationRequest, value: string) {
    setIsDirty(true);
    setForm((current) => ({ ...current, [field]: value }));
  }

  return (
    <Card>
      <CardHeader>
        <div className="status-card-heading">
          <CardTitle>{station.name}</CardTitle>
          <Badge>{station.station_id}</Badge>
        </div>
        <CardDescription>Konfigurasi logical station untuk satu sumber foto.</CardDescription>
      </CardHeader>
      <CardContent>
        <StationReadinessPanel readiness={readinessQuery.data?.readiness} isLoading={readinessQuery.isLoading} error={readinessQuery.error} />
        <StationCameraAssignment station={station} discovery={discovery} isPending={cameraAssignmentMutation.isPending} onAssign={(identityKey) => void handleAssignCamera(identityKey)} />
        <TetherListenerPanel
          listener={tetherQuery.data?.listener}
          isLoading={tetherQuery.isLoading}
          isStarting={startTetherMutation.isPending}
          isStopping={stopTetherMutation.isPending}
          onRefresh={() => void tetherQuery.refetch()}
          onStart={() => void handleStartTether()}
          onStop={() => void handleStopTether()}
        />
        {isDirty ? <p className="error-message">Simpan perubahan station sebelum menjalankan readiness check.</p> : null}
        <Button disabled={readinessMutation.isPending || isDirty} onClick={() => void handleReadinessCheck()} type="button">
          {readinessMutation.isPending ? "Memeriksa..." : "Run readiness check"}
        </Button>
        <Button disabled={healthRefreshMutation.isPending || isDirty} onClick={() => void handleHealthRefresh()} type="button">
          {healthRefreshMutation.isPending ? "Refresh health..." : "Refresh station health"}
        </Button>
        <Button disabled={watchValidationMutation.isPending || isDirty} onClick={() => void handleWatchValidation()} type="button">
          {watchValidationMutation.isPending ? "Menjalankan validation..." : "Run test watch validation"}
        </Button>
        <Button disabled={cameraTestCaptureMutation.isPending || isDirty} onClick={() => void handleCameraTestCapture()} type="button">
          {cameraTestCaptureMutation.isPending ? "Menjalankan camera test capture..." : "Run camera test capture"}
        </Button>
        <p>Camera test capture menulis JPG validation-only ke input folder dan memvalidasi watcher path; tidak upload Drive langsung.</p>
        {validation ? <WatchValidationSummary validation={validation} /> : null}
        <div className="start-session-panel">
          <h3>Start session</h3>
          <input value={sessionForm.customer_name} onChange={(event) => setSessionForm((current) => ({ ...current, customer_name: event.target.value }))} placeholder="Customer name" />
          <input value={sessionForm.order_number} onChange={(event) => setSessionForm((current) => ({ ...current, order_number: event.target.value }))} placeholder="Order number" />
          <input value={String(sessionForm.timer_seconds)} onChange={(event) => setSessionForm((current) => ({ ...current, timer_seconds: Number(event.target.value) }))} placeholder="Timer seconds" type="number" min={60} max={86400} />
          <Button disabled={startSessionMutation.isPending || isDirty || sessionForm.customer_name.trim() === "" || sessionForm.order_number.trim() === "" || !Number.isFinite(sessionForm.timer_seconds) || sessionForm.timer_seconds < 60 || sessionForm.timer_seconds > 86400} onClick={() => void handleStartSession()} type="button">
            {startSessionMutation.isPending ? "Memulai session..." : "Start session"}
          </Button>
        </div>
        <form className="station-form" onSubmit={handleSubmit}>
          <TextField label="Nama station" value={form.name} onChange={(value) => updateField("name", value)} />
          <TextField
            label="Device identifier"
            value={form.device_identifier}
            onChange={(value) => updateField("device_identifier", value)}
          />
          <TextField
            label="Input folder"
            value={form.input_folder}
            onChange={(value) => updateField("input_folder", value)}
          />
          <TextField
            label="Background name"
            value={form.background_name}
            onChange={(value) => updateField("background_name", value)}
          />
          <TextField
            label="Default LUT path"
            value={form.default_lut_path}
            onChange={(value) => updateField("default_lut_path", value)}
          />
          <TextField
            label="Output rule"
            value={form.output_rule}
            onChange={(value) => updateField("output_rule", value)}
          />
          {message ? <p className={message.kind === "error" ? "error-message" : "success-message"}>{message.text}</p> : null}
          <Button disabled={mutation.isPending} type="submit">
            {mutation.isPending ? "Menyimpan..." : "Simpan station"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function GPhoto2DiscoveryPanel({ discovery, isPending, message, onDiscover }: { discovery: GPhoto2DiscoveryData | null; isPending: boolean; message: { kind: "success" | "error"; text: string } | null; onDiscover: () => void }) {
  return (
    <Card>
      <CardHeader>
        <div className="status-card-heading">
          <CardTitle>gPhoto2 camera discovery</CardTitle>
          <Badge>{discovery?.status ?? "not_scanned"}</Badge>
        </div>
        <CardDescription>Discovery hanya mendeteksi dan meng-assign kamera. gPhoto2 tetap harus menjatuhkan JPG ke input folder station; folder watcher tetap source of truth.</CardDescription>
      </CardHeader>
      <CardContent>
        <Button disabled={isPending} onClick={onDiscover} type="button">
          {isPending ? "Mendeteksi kamera..." : "Run gPhoto2 discovery"}
        </Button>
        {message ? <p className={message.kind === "success" ? "success-message" : "error-message"}>{message.text}</p> : null}
        {discovery ? (
          <div className="readiness-panel">
            <p>Runtime: {discovery.runtime}. Scanned: {new Date(discovery.scanned_at).toLocaleString()}.</p>
            <p>Action: {cameraActionText(discovery.action)}</p>
            {discovery.diagnostics.length > 0 ? <ul>{discovery.diagnostics.map((item) => <li key={item}>{item}</li>)}</ul> : null}
            {discovery.cameras.length === 0 ? <p>Tidak ada kamera gPhoto2 terdeteksi.</p> : null}
            {discovery.cameras.map((camera, index) => (
              <div key={camera.identity_key} className="restore-panel">
                <strong>{safeCameraLabel(camera.name, index)}</strong>
                <span>{camera.runtime} · {safeTransportLabel(camera.transport)}</span>
                <Badge>{camera.connected ? "connected" : "disconnected"}</Badge>
              </div>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

function StationCameraAssignment({ station, discovery, isPending, onAssign }: { station: Station; discovery: GPhoto2DiscoveryData | null; isPending: boolean; onAssign: (identityKey: string) => void }) {
  const assigned = station.camera_assignment;
  return (
    <div className="readiness-panel">
      <div className="status-card-heading">
        <strong>gPhoto2 camera assignment</strong>
        <Badge>{assigned ? (assigned.connected ? "assigned_connected" : "assigned") : "unassigned"}</Badge>
      </div>
      {assigned ? <p>{safeCameraLabel(assigned.camera_name)} · {assigned.runtime}. Assigned: {new Date(assigned.assigned_at).toLocaleString()}</p> : <p>Belum ada kamera gPhoto2 ter-assign untuk station ini.</p>}
      <select disabled={isPending || !discovery || discovery.cameras.length === 0} defaultValue="" onChange={(event) => event.target.value && onAssign(event.target.value)}>
        <option value="">Pilih kamera hasil discovery...</option>
        {(discovery?.cameras ?? []).map((camera: DetectedGPhoto2Camera, index) => (
          <option key={camera.identity_key} value={camera.identity_key}>{safeCameraLabel(camera.name, index)} ({camera.runtime})</option>
        ))}
      </select>
      <p>Manual approval required untuk install, driver, atau usbipd bind/attach. Selfstudio tidak menjalankan command privileged otomatis.</p>
    </div>
  );
}

function TetherListenerPanel({ listener, isLoading, isStarting, isStopping, onRefresh, onStart, onStop }: { listener?: { status: string; message: string; camera_name?: string; runtime?: string; started_at?: string; stopped_at?: string; last_capture_at?: string; last_downloaded_file_name?: string; last_error_action?: string }; isLoading: boolean; isStarting: boolean; isStopping: boolean; onRefresh: () => void; onStart: () => void; onStop: () => void }) {
  const isRunning = listener?.status === "running" || listener?.status === "starting";
  return (
    <div className="readiness-panel">
      <div className="status-card-heading">
        <strong>gPhoto2 tether listener</strong>
        <Badge>{isLoading ? "loading" : listener?.status ?? "unknown"}</Badge>
      </div>
      <p>{listener?.message ?? "Status listener belum dimuat."}</p>
      {listener?.camera_name ? <p>{listener.camera_name} · {listener.runtime}</p> : null}
      {listener?.last_downloaded_file_name ? <p>Last JPG: {listener.last_downloaded_file_name}</p> : null}
      {listener?.last_capture_at ? <p>Last capture: {new Date(listener.last_capture_at).toLocaleString()}</p> : null}
      {listener?.last_error_action ? <p>Action: {cameraActionText(listener.last_error_action)}</p> : null}
      <div className="station-config-actions">
        <Button disabled={isLoading} onClick={onRefresh} type="button">Refresh tether</Button>
        <Button disabled={isStarting || isRunning} onClick={onStart} type="button">{isStarting ? "Starting..." : "Start tether"}</Button>
        <Button disabled={isStopping || !isRunning} onClick={onStop} type="button">{isStopping ? "Stopping..." : "Stop tether"}</Button>
      </div>
      <p>Listener hanya men-download JPG ke input folder station. Ingestion tetap lewat folder watcher existing.</p>
    </div>
  );
}

function StationReadinessPanel({ readiness, isLoading, error }: { readiness?: StationReadiness; isLoading: boolean; error: Error | null }) {
  if (isLoading) return <p>Memuat readiness station...</p>;
  if (error) return <p className="error-message">{formatError(error)}</p>;
  if (!readiness) return null;

  return (
    <div className="readiness-panel">
      <div className="status-card-heading">
        <strong>{readiness.label}</strong>
        <Badge>{readiness.status}</Badge>
      </div>
      <p>{readiness.action}</p>
      <ul>
        {readiness.checks.map((check) => (
          <li key={check.check_key}>
            <strong>{check.check_key}</strong>: {check.status} — {check.label}. {check.action}
          </li>
        ))}
      </ul>
    </div>
  );
}

function WatchValidationSummary({ validation }: { validation: WatchValidation }) {
  return (
    <div className="readiness-panel">
      <div className="status-card-heading">
        <strong>{validation.label}</strong>
        <Badge>{validation.status}</Badge>
      </div>
      <p>{validation.action}</p>
      <p>Validation-only: tidak membuat session, photo record, atau customer output.</p>
      {validation.source_path ? <p>Source file: {safeFileName(validation.source_path)}</p> : null}
      <p>Validated: {new Date(validation.validated_at).toLocaleString()}</p>
    </div>
  );
}

function TextField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="form-field">
      <span>{label}</span>
      <input required value={value} onChange={(event) => onChange(event.target.value)} />
    </label>
  );
}

function toForm(station: Station): UpdateStationRequest {
  return {
    name: station.name,
    device_identifier: station.device_identifier,
    input_folder: station.input_folder,
    background_name: station.background_name,
    default_lut_path: station.default_lut_path,
    output_rule: station.output_rule,
  };
}

function safeFileName(path: string): string {
  const parts = path.split(/[\\/]+/).filter(Boolean);
  const fileName = parts.at(-1) ?? "file";
  return fileName.replace(/[\r\n<>]/g, "").slice(0, 120) || "file";
}

function safeCameraLabel(name?: string, index = 0): string {
  const cleaned = (name ?? "").replace(/[\\/]/g, " ").replace(/[|:;]/g, " ").replace(/\s+/g, " ").trim();
  return cleaned ? cleaned.slice(0, 80) : `Kamera ${index + 1}`;
}

function safeTransportLabel(transport?: string): string {
  const value = (transport ?? "").toLowerCase();
  if (value.includes("usb")) return "USB camera";
  if (value.includes("ptp")) return "PTP camera";
  if (value.includes("mtp")) return "MTP camera";
  return "Camera connection";
}

function cameraActionText(action: string): string {
  switch (action) {
    case "NONE":
      return "Kamera siap diassign.";
    case "INSTALL_GPHOTO2":
      return "Install gPhoto2 di runtime yang dipilih, lalu jalankan discovery ulang.";
    case "CHECK_WSL":
      return "Periksa WSL dan distro default. Jalankan pengecekan manual di terminal jika Anda setuju.";
    case "CHECK_USBIPD":
      return "Periksa usbipd dan attach kamera ke WSL secara manual bila disetujui operator/admin.";
    case "CONNECT_CAMERA":
      return "Hubungkan kamera dan cek mode USB/PTP/PC Remote, lalu discovery ulang.";
    case "RETRY_CAMERA_DISCOVERY":
      return "Discovery gagal sementara atau timeout. Coba lagi setelah kamera tidak busy.";
    case "CHECK_CAMERA_USB_MODE":
      return "Kamera mungkin terlihat OS-level tetapi belum siap gPhoto2; cek USB mode kamera.";
    case "ASSIGN_CAMERA":
      return "Assign kamera hasil discovery ke station ini.";
    case "START_TETHER_LISTENER":
      return "Start tether listener sebelum event.";
    case "STOP_TETHER_LISTENER":
      return "Stop tether listener sebelum command test capture terpisah.";
    case "CHECK_STATION_INPUT_FOLDER":
      return "Periksa input folder station dan permission tulis.";
    case "RETRY_TEST_CAPTURE":
      return "Jalankan ulang camera test capture setelah masalah kamera/folder selesai.";
    case "RECHECK_CAMERA_READINESS":
      return "Jalankan ulang camera readiness check.";
    default:
      return action;
  }
}

function formatError(error: unknown): string {
  if (error instanceof ApiError) {
    return `${error.message} ${error.action}`;
  }
  return "Konfigurasi station gagal diproses. Coba lagi atau restart aplikasi.";
}
