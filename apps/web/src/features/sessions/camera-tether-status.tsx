import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  ApiError,
  type CameraAssignment,
  type ReadinessCheck,
  type Station,
  type StationReadiness,
  type TetherListener,
} from "@/lib/api/client";
import { useCameraTestCaptureMutation } from "@/features/stations/use-camera-test-capture-mutation";
import {
  useRetryTetherListenerMutation,
  useStartTetherListenerMutation,
  useStopTetherListenerMutation,
  useTetherListenerQuery,
  useUpdateTetherSettingsMutation,
} from "@/features/stations/use-tether-listener";

type CameraTetherStatusProps = {
  station: Station;
  readiness?: StationReadiness;
};

export function CameraTetherStatus({ station, readiness }: CameraTetherStatusProps) {
  const tetherQuery = useTetherListenerQuery(station.station_id);
  const startMutation = useStartTetherListenerMutation(station.station_id);
  const retryMutation = useRetryTetherListenerMutation(station.station_id);
  const stopMutation = useStopTetherListenerMutation(station.station_id);
  const settingsMutation = useUpdateTetherSettingsMutation(station.station_id);
  const testCaptureMutation = useCameraTestCaptureMutation(station.station_id);
  const [message, setMessage] = useState<{ kind: "success" | "error"; text: string } | null>(null);
  const listener = tetherQuery.data?.listener;
  const settings = tetherQuery.data?.settings;
  const recovery = tetherQuery.data?.recovery;
  const listenerStatus = listener?.status ?? "stopped";
  const isMutationPending = startMutation.isPending || retryMutation.isPending || stopMutation.isPending || settingsMutation.isPending || testCaptureMutation.isPending;
  const canStart = listenerStatus === "stopped" || listenerStatus === "error";
  const canStop = listenerStatus === "running" || listenerStatus === "starting";
  const isRetry = listenerStatus === "error";
  const nextAction = resolveNextAction(readiness, listener);

  async function handleStart() {
    setMessage(null);
    try {
      const data = await (isRetry ? retryMutation.mutateAsync() : startMutation.mutateAsync());
      setMessage({
        kind: "success",
        text: data.listener.already_running
          ? "Tether listener sudah berjalan untuk station ini."
          : "Tether listener berhasil dimulai.",
      });
    } catch (error) {
      setMessage({ kind: "error", text: formatSafeError(error) });
    }
  }

  async function handleStop() {
    setMessage(null);
    try {
      await stopMutation.mutateAsync();
      setMessage({ kind: "success", text: "Tether listener dihentikan untuk station ini." });
    } catch (error) {
      setMessage({ kind: "error", text: formatSafeError(error) });
    }
  }

  async function handleAutoRestartToggle(enabled: boolean) {
    setMessage(null);
    try {
      await settingsMutation.mutateAsync(enabled);
      setMessage({ kind: "success", text: enabled ? "Auto-restart tether diaktifkan." : "Auto-restart tether dimatikan." });
    } catch (error) {
      setMessage({ kind: "error", text: formatSafeError(error) });
    }
  }

  async function handleTestCapture() {
    setMessage(null);
    try {
      const data = await testCaptureMutation.mutateAsync();
      const fileName = data.test_capture.file_name ? ` (${safeFileName(data.test_capture.file_name)})` : "";
      const action = data.test_capture.status === "success" ? "" : ` ${cameraActionLabel(data.test_capture.action)}`;
      setMessage({
        kind: data.test_capture.status === "success" ? "success" : "error",
        text: `${safeText(data.test_capture.label)}${fileName}${action}`,
      });
    } catch (error) {
      setMessage({ kind: "error", text: formatSafeError(error) });
    }
  }

  return (
    <section className="readiness-panel" aria-label={`Camera dan tether ${station.name}`}>
      <div className="status-card-heading">
        <strong>Camera/Tether</strong>
        <Badge>{listenerStatusLabel(listenerStatus)}</Badge>
      </div>
      {tetherQuery.isError ? <p className="error-message">{formatSafeError(tetherQuery.error)}</p> : null}
      <dl className="health-list">
        <div>
          <dt>Assignment</dt>
          <dd>{assignmentLabel(station.camera_assignment)}</dd>
        </div>
        <div>
          <dt>Connection</dt>
          <dd>{connectionLabel(readiness)}</dd>
        </div>
        <div>
          <dt>Listener</dt>
          <dd>{listenerCopy(listener)}</dd>
        </div>
        <div>
          <dt>Last capture time</dt>
          <dd>{listener?.last_capture_at ? new Date(listener.last_capture_at).toLocaleString("id-ID") : "Belum ada capture"}</dd>
        </div>
        <div>
          <dt>Last downloaded filename</dt>
          <dd>{listener?.last_downloaded_file_name ? safeFileName(listener.last_downloaded_file_name) : "-"}</dd>
        </div>
        <div>
          <dt>Desired listener</dt>
          <dd>{settings?.desired_state === "running" ? "Running desired" : "Stopped desired"}</dd>
        </div>
        <div>
          <dt>Auto-restart</dt>
          <dd>{settings?.auto_restart_enabled ? "Enabled" : "Disabled"}</dd>
        </div>
        <div>
          <dt>Recovery</dt>
          <dd>{recoveryCopy(recovery)}</dd>
        </div>
        <div>
          <dt>Next action</dt>
          <dd>{cameraActionLabel(nextAction)}</dd>
        </div>
      </dl>
      {listener?.message ? <p>{safeText(listener.message)}</p> : null}
      {message ? <p className={message.kind === "success" ? "success-message" : "error-message"}>{message.text}</p> : null}
      <div className="button-row">
        <Button disabled={!canStart || isMutationPending} onClick={() => void handleStart()} type="button">
          {startMutation.isPending || retryMutation.isPending ? "Starting listener..." : isRetry ? "Retry listener" : "Start listener"}
        </Button>
        <Button disabled={!canStop || isMutationPending} onClick={() => void handleStop()} type="button">
          {stopMutation.isPending ? "Stopping listener..." : "Stop listener"}
        </Button>
        <Button disabled={isMutationPending} onClick={() => void handleAutoRestartToggle(!settings?.auto_restart_enabled)} type="button">
          {settingsMutation.isPending ? "Menyimpan..." : settings?.auto_restart_enabled ? "Disable auto-restart" : "Enable auto-restart"}
        </Button>
        <Button disabled={isMutationPending} onClick={() => void handleTestCapture()} type="button">
          {testCaptureMutation.isPending ? "Menjalankan test capture..." : "Run test capture"}
        </Button>
      </div>
      <p>Camera/tether hanya mengatur download JPG ke input folder station; local processing dan Google Drive upload tetap terpisah.</p>
    </section>
  );
}

function assignmentLabel(assignment?: CameraAssignment): string {
  if (!assignment) return "Kamera belum di-assign";
  return `Kamera assigned: ${safeCameraLabel(assignment)}`;
}

export function safeCameraLabel(assignment?: Pick<CameraAssignment, "camera_name">): string {
  const cleaned = (assignment?.camera_name ?? "").replace(/[\\/]/g, " ").replace(/[|:;]/g, " ").replace(/\s+/g, " ").trim();
  return cleaned ? cleaned.slice(0, 80) : "Assigned camera";
}

export function safeFileName(name: string): string {
  const parts = name.split(/[\\/]+/).filter(Boolean);
  const fileName = parts.at(-1) ?? "-";
  return fileName.replace(/[\r\n<>]/g, "").slice(0, 120) || "-";
}

function safeText(value: string): string {
  return value
    .replace(/\b(?:stdout|stderr)\s*[:=][^\r\n]*/gi, "diagnostic omitted safely")
    .replace(/\b(?:bash|sh)\s+-(?:lc|l|c)\b[^\r\n]*/gi, "diagnostic omitted safely")
    .replace(/\/(?:bin|usr\/bin|usr\/local\/bin)\/(?:env|bash|sh|zsh|fish)\b(?::|\s+[^\r\n]*)?/gi, "diagnostic omitted safely")
    .replace(/\b(?:gphoto2|wsl(?:\.exe)?|usbipd|powershell(?:\.exe)?|cmd(?:\.exe)?)\b[^\r\n]*/gi, "diagnostic omitted safely")
    .replace(/[A-Za-z]:\\[^\s]+/g, "[path]")
    .replace(/\/(?:Users|Volumes|workspace|mnt|home|tmp|var|usr|etc|opt|media|run|dev)\/[^\s]+/gi, "[path]")
    .replace(/(^|[\s(=:;,])\/(?!\/)(?:[^\s/<>:"'`|]+\/)+[^\s<>:"'`|]*/g, "$1[path]")
    .replace(/\busb:\d{1,3},\d{1,3}\b/gi, "[camera port]")
    .replace(/\b(?:identity_key|identity key|device_path|device path|bus_id|bus id)\b\s*[:=]?\s*[^\s,;)]*/gi, "camera identifier omitted safely")
    .replace(/\b(?:token|secret|password|credential|api[_-]?key|access[_-]?key|refresh[_-]?token)\b\s*[:=]?\s*[^\s,;)]*/gi, "credential detail omitted safely")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, 220);
}

function connectionLabel(readiness?: StationReadiness): string {
  const check = findCheck(readiness, "camera_connected");
  if (!check) return "Unknown";
  if (check.status === "ready") return "Connected";
  if (check.status === "failed" || check.status === "warning") return "Disconnected / needs attention";
  return "Unknown";
}

function listenerStatusLabel(status: string): string {
  if (status === "running") return "Running";
  if (status === "stopped") return "Stopped";
  if (status === "error") return "Error";
  if (status === "starting") return "Starting";
  if (status === "stopping") return "Stopping";
  return "Unknown";
}

function recoveryCopy(recovery?: { status: string; attempt_count: number; next_attempt_at?: string; message?: string }): string {
  if (!recovery) return "Belum ada recovery aktif";
  const next = recovery.next_attempt_at ? `, next ${new Date(recovery.next_attempt_at).toLocaleTimeString("id-ID")}` : "";
  const base = `${recoveryStatusLabel(recovery.status)} (attempt ${recovery.attempt_count}${next})`;
  return recovery.message ? `${base} — ${safeText(recovery.message)}` : base;
}

function recoveryStatusLabel(status: string): string {
  if (status === "scheduled") return "Scheduled";
  if (status === "attempting") return "Attempting";
  if (status === "succeeded") return "Succeeded";
  if (status === "failed") return "Failed";
  if (status === "paused") return "Paused";
  return "Idle";
}

function listenerCopy(listener?: TetherListener): string {
  if (!listener) return "Status listener belum dimuat.";
  if (listener.status === "running") return "Running — shutter fisik akan didownload ke input folder station";
  if (listener.status === "stopped") return "Stopped";
  if (listener.status === "error") return "Error — gunakan Retry listener atau ikuti next action";
  if (listener.status === "starting") return "Starting...";
  if (listener.status === "stopping") return "Stopping...";
  return "Unknown";
}

function resolveNextAction(readiness?: StationReadiness, listener?: TetherListener): string {
  if (listener?.last_error_action) return listener.last_error_action;
  const priority = [
    "camera_assignment",
    "gphoto2_availability",
    "camera_connected",
    "tether_listener",
    "input_folder_writable",
    "camera_test_capture",
  ];
  for (const key of priority) {
    const check = findCheck(readiness, key);
    if (check && check.status !== "ready" && check.action) return check.action;
  }
  return listener?.status === "running" ? "STOP_TETHER_LISTENER" : "START_TETHER_LISTENER";
}

function findCheck(readiness: StationReadiness | undefined, key: string): ReadinessCheck | undefined {
  return readiness?.checks.find((check) => check.check_key === key);
}

export function cameraActionLabel(action?: string): string {
  switch (action) {
    case "ASSIGN_CAMERA":
      return "Assign kamera di Station Settings";
    case "CONNECT_CAMERA":
      return "Hubungkan kamera / cek USB mode";
    case "CHECK_USBIPD":
      return "Cek USB attach ke WSL secara manual";
    case "CHECK_WSL":
      return "Cek WSL runtime";
    case "INSTALL_GPHOTO2":
      return "Install/aktifkan gPhoto2 secara manual";
    case "START_TETHER_LISTENER":
      return "Start tether listener";
    case "STOP_TETHER_LISTENER":
      return "Stop tether listener";
    case "RETRY_TETHER_LISTENER":
      return "Retry tether listener";
    case "CHECK_STATION_INPUT_FOLDER":
      return "Cek input folder station";
    case "RETRY_TEST_CAPTURE":
      return "Retry test capture";
    case "RECHECK_CAMERA_READINESS":
      return "Recheck camera readiness";
    case "NONE":
      return "Tidak ada aksi kamera yang dibutuhkan";
    default:
      return "Recheck camera readiness";
  }
}

function formatSafeError(error: unknown): string {
  if (error instanceof ApiError) {
    return `${safeText(error.message)} ${cameraActionLabel(error.action)}`;
  }
  if (error instanceof Error) {
    return safeText(error.message);
  }
  return "Aksi camera/tether gagal. Recheck camera readiness.";
}
