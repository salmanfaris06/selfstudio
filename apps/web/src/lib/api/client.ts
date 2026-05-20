export type CameraAssignment = {
  identity_key: string;
  camera_name: string;
  port: string;
  runtime: "native_windows" | "wsl" | "unknown" | string;
  assigned_at: string;
  last_seen_at?: string;
  connected: boolean;
};

export type Station = {
  station_id: string;
  name: string;
  device_identifier: string;
  input_folder: string;
  background_name: string;
  default_lut_path: string;
  output_rule: string;
  camera_assignment?: CameraAssignment;
};

export type StationListData = {
  stations: Station[];
};

export type ReadinessStatus = "ready" | "warning" | "failed" | "unknown";

export type ReadinessCheck = {
  check_key:
    | "input_folder"
    | "output_folder"
    | "default_lut"
    | "device"
    | string;
  status: ReadinessStatus;
  label: string;
  action: string;
};

export type StationReadiness = {
  station_id: string;
  status: ReadinessStatus;
  label: string;
  action: string;
  checked_at: string;
  checks: ReadinessCheck[];
};

export type StationReadinessData = {
  readiness: StationReadiness;
};

export type EventReadinessStatus = ReadinessStatus | "placeholder";

export type EventReadinessItem = {
  category: "stations" | "storage" | "cloud" | "operator" | string;
  item_key: string;
  status: EventReadinessStatus;
  label: string;
  action: string;
};

export type EventReadiness = {
  status: EventReadinessStatus;
  label: string;
  action: string;
  checked_at: string;
  session_start_available: boolean;
  session_start_action: string;
  items: EventReadinessItem[];
};

export type EventReadinessData = {
  readiness: EventReadiness;
};

export type StationData = {
  station: Station;
};

export type DetectedGPhoto2Camera = {
  identity_key: string;
  name: string;
  model?: string;
  port: string;
  device_path?: string;
  bus_id?: string;
  transport: "usb" | "ptp" | "unknown" | string;
  runtime: "native_windows" | "wsl" | "unknown" | string;
  connected: boolean;
  diagnostics?: string[];
  detected_at: string;
};

export type GPhoto2DiscoveryData = {
  status: "ready" | "no_cameras" | "gphoto2_missing" | "wsl_missing" | "usbipd_check_needed" | "error" | string;
  action: "NONE" | "INSTALL_GPHOTO2" | "CHECK_WSL" | "CHECK_USBIPD" | "CONNECT_CAMERA" | "RETRY_CAMERA_DISCOVERY" | "CHECK_CAMERA_USB_MODE" | string;
  runtime: "native_windows" | "wsl" | "unknown" | string;
  cameras: DetectedGPhoto2Camera[];
  diagnostics: string[];
  scanned_at: string;
};

export type CameraAssignmentRequest = {
  identity_key: string;
  camera_name: string;
  port: string;
  runtime: string;
  connected: boolean;
};

export type TetherListenerStatus =
  | "stopped"
  | "starting"
  | "running"
  | "stopping"
  | "error";

export type TetherListener = {
  station_id: string;
  status: TetherListenerStatus;
  runtime?: "native_windows" | "wsl" | "unknown" | string;
  camera_name?: string;
  started_at?: string;
  stopped_at?: string;
  last_capture_at?: string;
  last_downloaded_file_name?: string;
  last_error_code?: string;
  last_error_action?: string;
  message: string;
  already_running?: boolean;
};

export type TetherListenerSettings = {
  station_id: string;
  desired_state: "running" | "stopped" | string;
  auto_restart_enabled: boolean;
  last_started_at?: string;
  last_stopped_at?: string;
  last_recovery_attempt_at?: string;
  recovery_attempt_count?: number;
  updated_at: string;
};

export type TetherRecoveryStatus = {
  station_id: string;
  status: "idle" | "scheduled" | "attempting" | "succeeded" | "failed" | "paused" | string;
  attempt_count: number;
  next_attempt_at?: string;
  last_error_code?: string;
  last_error_action?: string;
  message: string;
  updated_at: string;
};

export type TetherListenerData = {
  listener: TetherListener;
  settings: TetherListenerSettings;
  recovery: TetherRecoveryStatus;
};

export type CameraTestCaptureStatus = "not_run" | "running" | "success" | "warning" | "failed";

export type CameraTestCaptureResult = {
  station_id: string;
  status: CameraTestCaptureStatus;
  label: string;
  action: string;
  file_name?: string;
  captured_at?: string;
  detected_at?: string;
  stable_at?: string;
  validation_only: boolean;
};

export type CameraTestCaptureData = {
  test_capture: CameraTestCaptureResult;
};

export type StationBackupData = {
  backup: {
    filename: string;
    created_at: string;
    station_count: number;
  };
};

export type StationRestoreData = {
  restored: boolean;
  station_count: number;
};

export type StationSnapshot = {
  station_name: string;
  background_name: string;
  default_lut_path: string;
  input_folder: string;
  output_rule: string;
  output_folder: string;
  device_identifier: string;
};

export type Session = {
  session_id: string;
  station_id: string;
  status: "active" | "locked";
  customer_name: string;
  order_number: string;
  timer_seconds: number;
  started_at: string;
  ends_at: string;
  station_snapshot: StationSnapshot;
  ended_at: string | null;
  end_reason: "manual" | "timer" | null;
};

export type QuarantineReason = "no_active_session" | "late_photo";

export type CloudTargetStatus =
  | "not_configured"
  | "pending"
  | "resolving"
  | "ready"
  | "failed"
  | "placeholder";
export type SessionUploadStatus =
  | "not_configured"
  | "target_pending"
  | "pending"
  | "uploading"
  | "uploaded"
  | "partial_failed"
  | "failed"
  | "pending_local_completion";
export type FileUploadStatus =
  | "pending"
  | "uploading"
  | "retrying"
  | "retry_scheduled"
  | "uploaded"
  | "failed"
  | "not_eligible";

export type DriveFolderRef = {
  level: string;
  name: string;
  folder_id: string;
  parent_id?: string;
};

export type SessionCloudTarget = {
  session_id: string;
  station_id: string;
  bucket_name?: string;
  target_root_prefix?: string;
  object_prefix?: string;
  drive_root_folder_id?: string;
  drive_root_folder_name?: string;
  drive_folder_path?: string;
  drive_session_folder_id?: string;
  drive_folder_chain?: DriveFolderRef[];
  remote_identity?: string;
  status: CloudTargetStatus;
  attempt_count?: number;
  last_error_code?: string;
  last_error_action?: string;
  last_checked_at?: string;
  resolved_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type SessionSummary = {
  local_output_folder: string;
  photo_count: number;
  failures: number;
  quarantine_count: number;
  station_quarantine_count?: number;
  latest_quarantine_reason?: QuarantineReason | "";
  upload_status: SessionUploadStatus;
  drive_target_status: CloudTargetStatus;
  drive_target_identity?: string;
  drive_session_folder_id?: string;
  drive_folder_path?: string;
  drive_root_folder_id?: string;
  drive_root_folder_name?: string;
  drive_last_error_code?: string;
  drive_last_error_action?: string;
};

export type FileUploadJob = {
  job_id: string;
  session_id: string;
  station_id: string;
  photo_id: string;
  asset_kind: "original" | "graded";
  local_file_name?: string;
  bucket_name?: string;
  object_key?: string;
  drive_folder_id?: string;
  drive_file_id?: string;
  remote_identity?: string;
  remote_generation?: number;
  remote_metageneration?: number;
  remote_etag?: string;
  dedupe_key?: string;
  status: FileUploadStatus;
  attempt_count: number;
  max_attempts?: number;
  last_error_code?: string;
  last_error_action?: string;
  last_attempt_at?: string;
  next_retry_at?: string;
  retry_after?: number;
  created_at: string;
  updated_at: string;
  uploaded_at?: string;
};

export type StationQuarantineSummary = {
  station_id: string;
  station_quarantine_count: number;
  latest_quarantine_reason?: QuarantineReason | "";
};

export type StationQuarantineSummaryData = {
  summary: StationQuarantineSummary;
};

export type SessionData = {
  session: Session;
};

export type OriginalSaveStatus =
  | "pending"
  | "saving"
  | "saved_original"
  | "failed";
export type ProcessingStatus = "not_eligible" | "eligible";
export type GradedProcessingStatus =
  | "not_eligible"
  | "pending"
  | "processing"
  | "processed"
  | "failed";
export type ProcessingQueueStatus = GradedProcessingStatus | "retrying";

export type RoutedPhoto = {
  photo_id: string;
  station_id: string;
  session_id: string;
  source_path: string;
  source_size_bytes: number;
  detected_at: string;
  stable_at: string;
  routed_at: string;
  status: "routed";
  local_original_path?: string;
  original_save_status: OriginalSaveStatus;
  last_error?: string;
  attempt_count: number;
  original_save_started_at?: string;
  original_saved_at?: string;
  processing_status: ProcessingStatus;
  local_graded_path?: string;
  graded_processing_status: GradedProcessingStatus;
  graded_last_error?: string;
  graded_attempt_count: number;
  graded_processing_started_at?: string;
  graded_processed_at?: string;
  lut_snapshot_path?: string;
  duplicate?: boolean;
};

export type QuarantineStatus = "quarantined" | "assigned";

export type QuarantinedPhoto = QuarantineItem;

export type QuarantineItem = {
  quarantine_id: string;
  photo_id?: string;
  station_id: string;
  related_session_id?: string;
  source_path: string;
  source_size_bytes: number;
  detected_at: string;
  stable_at: string;
  quarantined_at: string;
  reason: "no_active_session" | "late_photo";
  status: QuarantineStatus;
  assigned_session_id?: string;
  assigned_photo_id?: string;
  assigned_at?: string;
  duplicate?: boolean;
};

export type EligibleSession = {
  session_id: string;
  station_id: string;
  status: "active" | "locked";
  customer_name: string;
  order_number: string;
  eligible: boolean;
  requires_confirmation: boolean;
  eligibility_reason: string;
};

export type QuarantineListData = { items: QuarantineItem[] };
export type EligibleSessionsData = {
  quarantine_id: string;
  sessions: EligibleSession[];
};
export type AssignQuarantineRequest = { session_id: string };
export type AssignQuarantineData = {
  quarantine: QuarantineItem;
  photo: RoutedPhoto;
};
export type RetryProcessingData = {
  photo: RoutedPhoto;
  retry_started: boolean;
};

export type ProcessingQueueItem = {
  photo_id: string;
  station_id: string;
  session_id: string;
  source_path: string;
  local_original_path?: string;
  local_graded_path?: string;
  original_save_status: OriginalSaveStatus;
  processing_status: ProcessingStatus;
  graded_processing_status: GradedProcessingStatus;
  graded_last_error?: string;
  graded_attempt_count: number;
  graded_processing_started_at?: string;
  graded_processed_at?: string;
  last_updated_at: string;
};

export type ProcessingQueueSummary = {
  total: number;
  not_eligible: number;
  pending: number;
  processing: number;
  processed: number;
  failed: number;
  retrying: number;
  last_updated_at?: string;
  current_job?: ProcessingQueueItem;
};

export type ProcessingQueueData = {
  summary: ProcessingQueueSummary;
  items: ProcessingQueueItem[];
};

export type ProcessingQueueFilters = {
  station_id?: string;
  session_id?: string;
  status?: ProcessingQueueStatus;
  limit?: number;
};

export type RouteResult =
  | RoutedPhoto
  | QuarantinedPhoto
  | {
      station_id: string;
      source_path: string;
      source_size_bytes: number;
      detected_at: string;
      stable_at: string;
      status: "unassigned_pending_quarantine";
      duplicate: false;
    };

export type SessionDetailData = {
  session: Session;
  summary: SessionSummary;
  photos: RoutedPhoto[];
};

export type SessionCloudTargetData = {
  cloud_target: SessionCloudTarget;
};

export type SessionUploadsData = {
  session_id: string;
  upload_status: SessionUploadStatus;
  jobs: FileUploadJob[];
};

export type SessionListData = {
  sessions: Session[];
  recovered: boolean;
};

export type EndSessionRequest = {
  reason: "manual" | "timer";
};

export type StartSessionRequest = {
  customer_name: string;
  order_number: string;
  timer_seconds: number;
};

export type WatchValidationStatus =
  | "ready"
  | "running"
  | "success"
  | "warning"
  | "failed";

export type WatchValidation = {
  station_id: string;
  status: WatchValidationStatus;
  label: string;
  action: string;
  source_path: string | null;
  detected_at: string | null;
  stable_at: string | null;
  validated_at: string;
  validation_only: boolean;
};

export type WatchValidationData = {
  validation: WatchValidation;
};

export type UpdateStationRequest = Omit<Station, "station_id" | "camera_assignment">;

export type ActivityLogEntry = {
  id: string;
  occurred_at: string;
  action_type: string;
  result: "success" | "failure";
  message: string;
  station_id: string | null;
  session_id: string | null;
};

export type ActivityLogData = {
  entries: ActivityLogEntry[];
};

export type DetectedPhoto = {
  station_id: string;
  source_path: string;
  size_bytes: number;
  detected_at: string;
  stable_at: string;
  status: "detected";
};
export type StationScanError = {
  station_id: string;
  code: string;
  message: string;
};
export type IngestionScanData = {
  photos: DetectedPhoto[];
  routed_photos: RouteResult[];
  quarantined_photos: QuarantinedPhoto[];
  errors: StationScanError[];
};

export type ConfigPlaceholderData = {
  recorded: boolean;
};

export type AuthSession = {
  authenticated: boolean;
};

export type HealthStatusValue =
  | "ok"
  | "warning"
  | "error"
  | "placeholder"
  | "unknown";

export type HealthComponentStatus = {
  status: HealthStatusValue;
  label: string;
  action: string;
};

export type HealthData = {
  service: string;
  status: HealthStatusValue;
  database: HealthComponentStatus;
  worker: HealthComponentStatus;
  disk: HealthComponentStatus;
};

export type CloudConnectionStatus =
  | "not_configured"
  | "checking"
  | "authorized"
  | "failed";
export type CloudSettings = {
  provider: "google_drive";
  drive_root_folder_id: string;
  drive_root_folder_name?: string;
  folder_naming_template: string;
  credentials_configured: boolean;
  connection_status: CloudConnectionStatus;
  last_checked_at?: string;
  last_error_code?: string;
  last_error_action?: string;
};
export type UpdateCloudSettingsRequest = {
  provider: "google_drive";
  drive_root_folder_id: string;
  drive_root_folder_name?: string;
  service_account_json?: string;
  credential_file_path?: string;
};
export type DriveFolderPreviewRequest = {
  customer_name: string;
  order_number: string;
  station_id: string;
  session_id: string;
  asset_kind?: "original" | "graded";
  file_name?: string;
};
export type DriveFolderPreview = { folder_path: string; template: string };

export type DataResponse<T> = {
  data: T;
};

export type ErrorResponse = {
  error: {
    code: string;
    message: string;
    action: string;
    details: Record<string, unknown>;
  };
};

export class ApiError extends Error {
  code: string;
  action: string;
  details: Record<string, unknown>;

  constructor(error: ErrorResponse["error"]) {
    super(error.message);
    this.name = "ApiError";
    this.code = error.code;
    this.action = error.action;
    this.details = error.details;
  }
}

const apiBaseUrl = (
  process.env.NEXT_PUBLIC_SELFSTUDIO_API_URL || "http://localhost:8080"
).replace(/\/$/, "");

export function getApiBaseUrl() {
  return apiBaseUrl;
}

export async function getHealth(): Promise<HealthData> {
  return request<HealthData>("/api/health", { method: "GET" });
}

export async function getCloudSettings(): Promise<CloudSettings> {
  return request<CloudSettings>("/api/cloud/settings", { method: "GET" });
}

export async function updateCloudSettings(
  body: UpdateCloudSettingsRequest,
): Promise<CloudSettings> {
  return request<CloudSettings>("/api/cloud/settings", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export async function checkCloudConnection(): Promise<CloudSettings> {
  return request<CloudSettings>("/api/cloud/settings/check", {
    method: "POST",
  });
}

export async function previewCloudFolderPath(
  body: DriveFolderPreviewRequest,
): Promise<DriveFolderPreview> {
  return request<DriveFolderPreview>("/api/cloud/settings/folder-preview", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export async function getStations(): Promise<StationListData> {
  const data = await request<StationListData>("/api/stations", {
    method: "GET",
  });
  if (!Array.isArray(data.stations)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim daftar station yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getEventReadiness(): Promise<EventReadinessData> {
  const data = await request<EventReadinessData>("/api/readiness", {
    method: "GET",
  });
  if (!isEventReadiness(data.readiness)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim event readiness yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function runEventReadinessCheck(): Promise<EventReadinessData> {
  const data = await request<EventReadinessData>("/api/readiness/check", {
    method: "POST",
  });
  if (!isEventReadiness(data.readiness)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim event readiness yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getStationReadiness(
  stationId: string,
): Promise<StationReadinessData> {
  const data = await request<StationReadinessData>(
    `/api/stations/${encodeURIComponent(stationId)}/readiness`,
    { method: "GET" },
  );
  if (!isReadiness(data.readiness) || data.readiness.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim readiness station yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function runIngestionScan(): Promise<IngestionScanData> {
  const data = await request<IngestionScanData>("/api/ingestion/scan", {
    method: "POST",
  });
  if (
    !Array.isArray(data.photos) ||
    !Array.isArray(data.routed_photos) ||
    !Array.isArray(data.quarantined_photos) ||
    !Array.isArray(data.errors) ||
    !data.photos.every(isDetectedPhoto) ||
    !data.routed_photos.every(isRouteResult) ||
    !data.quarantined_photos.every(isQuarantinedPhoto)
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim hasil ingestion scan yang tidak valid.",
      action: "Coba scan ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getSessionCloudTarget(
  sessionId: string,
): Promise<SessionCloudTargetData> {
  const data = await request<SessionCloudTargetData>(
    `/api/sessions/${encodeURIComponent(sessionId)}/cloud-target`,
    { method: "GET" },
  );
  if (
    !isSessionCloudTarget(data.cloud_target) ||
    data.cloud_target.session_id !== sessionId
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim cloud target session yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function resolveSessionCloudTarget(
  sessionId: string,
): Promise<SessionCloudTargetData> {
  const data = await request<SessionCloudTargetData>(
    `/api/sessions/${encodeURIComponent(sessionId)}/cloud-target/resolve`,
    { method: "POST" },
  );
  if (
    !isSessionCloudTarget(data.cloud_target) ||
    data.cloud_target.session_id !== sessionId
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim cloud target session yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getSessionUploads(
  sessionId: string,
): Promise<SessionUploadsData> {
  const data = await request<SessionUploadsData>(
    `/api/sessions/${encodeURIComponent(sessionId)}/uploads`,
    { method: "GET" },
  );
  if (!isSessionUploadsData(data) || data.session_id !== sessionId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim upload status yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function startSessionUploads(
  sessionId: string,
): Promise<SessionUploadsData> {
  const data = await request<SessionUploadsData>(
    `/api/sessions/${encodeURIComponent(sessionId)}/uploads/start`,
    { method: "POST" },
  );
  if (!isSessionUploadsData(data) || data.session_id !== sessionId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim upload status yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function retrySessionUploads(
  sessionId: string,
): Promise<SessionUploadsData> {
  const data = await request<SessionUploadsData>(
    `/api/sessions/${encodeURIComponent(sessionId)}/uploads/retry`,
    { method: "POST" },
  );
  if (!isSessionUploadsData(data) || data.session_id !== sessionId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim retry upload status yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function retryUploadJob(
  jobId: string,
): Promise<SessionUploadsData> {
  const data = await request<SessionUploadsData>(
    `/api/uploads/${encodeURIComponent(jobId)}/retry`,
    { method: "POST" },
  );
  if (!isSessionUploadsData(data)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim retry upload status yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getSessionDetail(
  sessionId: string,
): Promise<SessionDetailData> {
  const data = await request<SessionDetailData>(
    `/api/sessions/${encodeURIComponent(sessionId)}`,
    { method: "GET" },
  );
  if (
    !isSession(data.session) ||
    data.session.session_id !== sessionId ||
    !isSessionSummary(data.summary) ||
    !Array.isArray(data.photos) ||
    !data.photos.every(isRoutedPhoto)
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim session detail yang tidak valid.",
      action: "Coba refresh ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getStationQuarantineSummary(
  stationId: string,
): Promise<StationQuarantineSummaryData> {
  const data = await request<StationQuarantineSummaryData>(
    `/api/stations/${encodeURIComponent(stationId)}/quarantine-summary`,
    { method: "GET" },
  );
  if (
    !isStationQuarantineSummary(data.summary) ||
    data.summary.station_id !== stationId
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim summary quarantine station yang tidak valid.",
      action: "Coba refresh ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getProcessingQueue(
  filters: ProcessingQueueFilters = {},
): Promise<ProcessingQueueData> {
  const params = new URLSearchParams();
  if (filters.station_id) params.set("station_id", filters.station_id);
  if (filters.session_id) params.set("session_id", filters.session_id);
  if (filters.status) params.set("status", filters.status);
  if (typeof filters.limit === "number")
    params.set("limit", String(filters.limit));
  const query = params.toString();
  const data = await request<ProcessingQueueData>(
    `/api/processing/queue${query ? `?${query}` : ""}`,
    { method: "GET" },
  );
  if (!isProcessingQueueData(data)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim processing queue yang tidak valid.",
      action: "Refresh dashboard atau restart Go agent.",
      details: {},
    });
  }
  return data;
}

export async function retryPhotoProcessing(
  photoId: string,
): Promise<RetryProcessingData> {
  const data = await request<RetryProcessingData>(
    `/api/photos/${encodeURIComponent(photoId)}/retry-processing`,
    { method: "POST" },
  );
  if (!isRetryProcessingData(data) || data.photo.photo_id !== photoId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim response retry processing yang tidak valid.",
      action: "Refresh queue lalu coba lagi.",
      details: {},
    });
  }
  return data;
}

export async function getQuarantineItems(
  filters: { status?: string; station_id?: string } = {},
): Promise<QuarantineListData> {
  const params = new URLSearchParams();
  if (filters.status) params.set("status", filters.status);
  if (filters.station_id) params.set("station_id", filters.station_id);
  const query = params.toString();
  const data = await request<QuarantineListData>(
    `/api/quarantine${query ? `?${query}` : ""}`,
    { method: "GET" },
  );
  if (!Array.isArray(data.items) || !data.items.every(isQuarantineItem)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim daftar quarantine yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getEligibleQuarantineSessions(
  quarantineId: string,
): Promise<EligibleSessionsData> {
  const data = await request<EligibleSessionsData>(
    `/api/quarantine/${encodeURIComponent(quarantineId)}/eligible-sessions`,
    { method: "GET" },
  );
  if (
    data.quarantine_id !== quarantineId ||
    !Array.isArray(data.sessions) ||
    !data.sessions.every(isEligibleSession)
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim daftar eligible session yang tidak valid.",
      action: "Refresh daftar quarantine lalu coba lagi.",
      details: {},
    });
  }
  return data;
}

export async function assignQuarantineItem(
  quarantineId: string,
  body: AssignQuarantineRequest,
): Promise<AssignQuarantineData> {
  const data = await request<AssignQuarantineData>(
    `/api/quarantine/${encodeURIComponent(quarantineId)}/assign`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
  if (
    !isQuarantineItem(data.quarantine) ||
    data.quarantine.quarantine_id !== quarantineId ||
    data.quarantine.status !== "assigned" ||
    !isRoutedPhoto(data.photo)
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim hasil assignment yang tidak valid.",
      action: "Refresh daftar quarantine lalu cek session tujuan.",
      details: {},
    });
  }
  return data;
}

export async function endSession(
  sessionId: string,
  body: EndSessionRequest = { reason: "manual" },
): Promise<SessionData> {
  const data = await request<SessionData>(
    `/api/sessions/${encodeURIComponent(sessionId)}/end`,
    { method: "POST", body: JSON.stringify(body) },
  );
  if (!isSession(data.session) || data.session.session_id !== sessionId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim session end response yang tidak valid.",
      action: "Coba refresh ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getSessions(): Promise<SessionListData> {
  const data = await request<SessionListData>("/api/sessions", {
    method: "GET",
  });
  if (
    !Array.isArray(data.sessions) ||
    !data.sessions.every(isSession) ||
    typeof data.recovered !== "boolean"
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim session list yang tidak valid.",
      action: "Coba refresh ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function startSession(
  stationId: string,
  body: StartSessionRequest,
): Promise<SessionData> {
  const data = await request<SessionData>(
    `/api/stations/${encodeURIComponent(stationId)}/sessions`,
    { method: "POST", body: JSON.stringify(body) },
  );
  if (!isSession(data.session) || data.session.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim session yang tidak valid.",
      action: "Coba mulai session ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function refreshStationHealth(
  stationId: string,
): Promise<StationReadinessData> {
  const data = await request<StationReadinessData>(
    `/api/stations/${encodeURIComponent(stationId)}/health/refresh`,
    { method: "POST" },
  );
  if (!isReadiness(data.readiness) || data.readiness.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim hasil refresh health yang tidak valid.",
      action: "Coba refresh ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function runCameraTestCapture(
  stationId: string,
): Promise<CameraTestCaptureData> {
  const data = await request<CameraTestCaptureData>(
    `/api/stations/${encodeURIComponent(stationId)}/camera-test-capture`,
    { method: "POST" },
  );
  if (!isCameraTestCaptureResult(data.test_capture) || data.test_capture.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim hasil camera test capture yang tidak valid.",
      action: "Jalankan ulang test capture atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function runStationReadinessCheck(
  stationId: string,
): Promise<StationReadinessData> {
  const data = await request<StationReadinessData>(
    `/api/stations/${encodeURIComponent(stationId)}/readiness/check`,
    { method: "POST" },
  );
  if (!isReadiness(data.readiness) || data.readiness.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim readiness station yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function backupStations(): Promise<StationBackupData> {
  const data = await request<StationBackupData>("/api/stations/backup", {
    method: "POST",
  });
  if (!isStationBackupData(data)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim metadata backup yang tidak valid.",
      action: "Coba backup ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function restoreStations(
  filename: string,
): Promise<StationRestoreData> {
  const data = await request<StationRestoreData>("/api/stations/restore", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ filename }),
  });
  if (!isStationRestoreData(data)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim hasil restore yang tidak valid.",
      action: "Coba restore ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function runWatchValidation(
  stationId: string,
): Promise<WatchValidationData> {
  const data = await request<WatchValidationData>(
    `/api/stations/${encodeURIComponent(stationId)}/validation/watch-test`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ timeout_ms: 5000, stability_ms: 500 }),
    },
  );
  if (
    !isWatchValidation(data.validation) ||
    data.validation.station_id !== stationId
  ) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim hasil watch validation yang tidak valid.",
      action: "Coba ulangi validation atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function discoverGPhoto2Cameras(): Promise<GPhoto2DiscoveryData> {
  const data = await request<GPhoto2DiscoveryData>("/api/cameras/gphoto2/discover", {
    method: "POST",
  });
  if (!Array.isArray(data.cameras) || !Array.isArray(data.diagnostics)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim hasil discovery kamera yang tidak valid.",
      action: "Coba discovery ulang atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function assignGPhoto2Camera(
  stationId: string,
  requestBody: CameraAssignmentRequest,
): Promise<StationData> {
  const data = await request<StationData>(
    `/api/stations/${encodeURIComponent(stationId)}/camera-assignment`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestBody),
    },
  );
  if (!isStation(data.station) || data.station.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim camera assignment yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getTetherListener(stationId: string): Promise<TetherListenerData> {
  const data = await request<TetherListenerData>(
    `/api/stations/${encodeURIComponent(stationId)}/tether-listener`,
    { method: "GET" },
  );
  if (!isTetherListener(data.listener) || data.listener.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim status tether listener yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function startTetherListener(stationId: string): Promise<TetherListenerData> {
  return mutateTetherListener(stationId, "start");
}

export async function stopTetherListener(stationId: string): Promise<TetherListenerData> {
  return mutateTetherListener(stationId, "stop");
}

export async function retryTetherListener(stationId: string): Promise<TetherListenerData> {
  return mutateTetherListener(stationId, "retry");
}

export async function updateTetherListenerSettings(stationId: string, autoRestartEnabled: boolean): Promise<TetherListenerData> {
  const data = await request<TetherListenerData>(
    `/api/stations/${encodeURIComponent(stationId)}/tether-listener/settings`,
    { method: "PUT", body: JSON.stringify({ auto_restart_enabled: autoRestartEnabled }) },
  );
  validateTetherListenerData(data, stationId);
  return data;
}

async function mutateTetherListener(stationId: string, action: "start" | "stop" | "retry"): Promise<TetherListenerData> {
  const data = await request<TetherListenerData>(
    `/api/stations/${encodeURIComponent(stationId)}/tether-listener/${action}`,
    { method: "POST" },
  );
  validateTetherListenerData(data, stationId);
  return data;
}

function validateTetherListenerData(data: TetherListenerData, stationId: string) {
  if (!isTetherListener(data.listener) || data.listener.station_id !== stationId || !data.settings || data.settings.station_id !== stationId || !data.recovery || data.recovery.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim hasil tether listener yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
}

export async function updateStation(
  stationId: string,
  requestBody: UpdateStationRequest,
): Promise<StationData> {
  const data = await request<StationData>(
    `/api/stations/${encodeURIComponent(stationId)}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestBody),
    },
  );
  if (!isStation(data.station) || data.station.station_id !== stationId) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message:
        "Selfstudio Agent mengirim konfigurasi station yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function getAuthSession(): Promise<AuthSession> {
  return request<AuthSession>("/api/auth/session", { method: "GET" });
}

export async function loginWithPin(pin: string): Promise<AuthSession> {
  return request<AuthSession>("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pin }),
  });
}

export async function logout(): Promise<AuthSession> {
  return request<AuthSession>("/api/auth/logout", { method: "POST" });
}

export async function getActivityLog(
  actionType?: string,
): Promise<ActivityLogData> {
  const params = new URLSearchParams({ limit: "50" });
  if (actionType) params.set("action_type", actionType);
  const data = await request<ActivityLogData>(
    `/api/activity?${params.toString()}`,
    { method: "GET" },
  );
  if (!Array.isArray(data.entries)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim activity log yang tidak valid.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: {},
    });
  }
  return data;
}

export async function recordConfigPlaceholderAction(): Promise<ConfigPlaceholderData> {
  return request<ConfigPlaceholderData>("/api/config/placeholder-action", {
    method: "POST",
  });
}

async function request<T>(path: string, init: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${apiBaseUrl}${path}`, {
      ...init,
      credentials: "include",
    });
  } catch {
    throw new ApiError({
      code: "AGENT_UNREACHABLE",
      message: "Tidak bisa terhubung ke Selfstudio Agent.",
      action: "Pastikan Go agent sedang berjalan dan URL API benar.",
      details: {},
    });
  }

  const payload = await parsePayload<T>(response);
  if (!response.ok || isErrorResponse(payload)) {
    if (isErrorResponse(payload)) {
      throw new ApiError(payload.error);
    }
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim response yang tidak dikenali.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: { status: response.status },
    });
  }

  if (!isDataResponse<T>(payload)) {
    throw new ApiError({
      code: "UNEXPECTED_RESPONSE",
      message: "Selfstudio Agent mengirim response tanpa data.",
      action: "Refresh dashboard atau restart aplikasi.",
      details: { status: response.status },
    });
  }

  return payload.data;
}

async function parsePayload<T>(
  response: Response,
): Promise<DataResponse<T> | ErrorResponse> {
  const contentType = response.headers.get("Content-Type") ?? "";
  if (!contentType.includes("application/json")) {
    return {
      error: {
        code: "NON_JSON_RESPONSE",
        message: "Selfstudio Agent mengirim response non-JSON.",
        action:
          "Pastikan URL API mengarah ke Go agent, bukan dashboard atau halaman lain.",
        details: { status: response.status },
      },
    };
  }

  try {
    return (await response.json()) as DataResponse<T> | ErrorResponse;
  } catch {
    return {
      error: {
        code: "INVALID_JSON_RESPONSE",
        message: "Selfstudio Agent mengirim JSON yang tidak valid.",
        action: "Coba ulangi aksi atau restart aplikasi.",
        details: { status: response.status },
      },
    };
  }
}

function isDataResponse<T>(
  payload: DataResponse<T> | ErrorResponse,
): payload is DataResponse<T> {
  return "data" in payload && !("error" in payload);
}

function isTetherListener(value: unknown): value is TetherListener {
  if (!value || typeof value !== "object") return false;
  const listener = value as TetherListener;
  return (
    typeof listener.station_id === "string" &&
    typeof listener.status === "string" &&
    typeof listener.message === "string"
  );
}

function isEventReadiness(value: unknown): value is EventReadiness {
  if (!value || typeof value !== "object") return false;
  const readiness = value as EventReadiness;
  if (
    !isEventReadinessStatus(readiness.status) ||
    typeof readiness.label !== "string" ||
    typeof readiness.action !== "string" ||
    typeof readiness.checked_at !== "string" ||
    Number.isNaN(Date.parse(readiness.checked_at)) ||
    typeof readiness.session_start_available !== "boolean" ||
    typeof readiness.session_start_action !== "string" ||
    !Array.isArray(readiness.items) ||
    !readiness.items.every(isEventReadinessItem)
  ) {
    return false;
  }
  const categories = new Set(readiness.items.map((item) => item.category));
  const itemKeys = new Set(readiness.items.map((item) => item.item_key));
  return (
    categories.has("stations") &&
    categories.has("storage") &&
    categories.has("cloud") &&
    categories.has("operator") &&
    ["station_1", "station_2", "station_3"].every((stationId) =>
      [
        stationId,
        `${stationId}.input_folder`,
        `${stationId}.output_folder`,
        `${stationId}.default_lut`,
        `${stationId}.device`,
      ].every((key) => itemKeys.has(key)),
    ) &&
    itemKeys.has("local_output_root") &&
    itemKeys.has("supabase") &&
    itemKeys.has("google_drive") &&
    itemKeys.has("session_start")
  );
}

function isEventReadinessItem(value: unknown): value is EventReadinessItem {
  if (!value || typeof value !== "object") return false;
  const item = value as EventReadinessItem;
  return (
    isEventReadinessCategory(item.category) &&
    typeof item.item_key === "string" &&
    isEventReadinessStatus(item.status) &&
    typeof item.label === "string" &&
    typeof item.action === "string"
  );
}

function isEventReadinessCategory(
  value: unknown,
): value is EventReadinessItem["category"] {
  return (
    value === "stations" ||
    value === "storage" ||
    value === "cloud" ||
    value === "operator"
  );
}

function isEventReadinessStatus(value: unknown): value is EventReadinessStatus {
  return isReadinessStatus(value) || value === "placeholder";
}

function isReadiness(value: unknown): value is StationReadiness {
  if (!value || typeof value !== "object") return false;
  const readiness = value as StationReadiness;
  const requiredKeys = new Set([
    "input_folder",
    "output_folder",
    "default_lut",
    "device",
  ]);
  if (
    typeof readiness.station_id !== "string" ||
    !isReadinessStatus(readiness.status) ||
    typeof readiness.label !== "string" ||
    typeof readiness.action !== "string" ||
    typeof readiness.checked_at !== "string" ||
    Number.isNaN(Date.parse(readiness.checked_at)) ||
    !Array.isArray(readiness.checks) ||
    !readiness.checks.every(isReadinessCheck)
  ) {
    return false;
  }
  for (const check of readiness.checks) requiredKeys.delete(check.check_key);
  return requiredKeys.size === 0;
}

function isCameraTestCaptureResult(value: unknown): value is CameraTestCaptureResult {
  if (!value || typeof value !== "object") return false;
  const result = value as CameraTestCaptureResult;
  return (
    typeof result.station_id === "string" &&
    (result.status === "not_run" || result.status === "running" || result.status === "success" || result.status === "warning" || result.status === "failed") &&
    typeof result.label === "string" &&
    typeof result.action === "string" &&
    (result.file_name === undefined || typeof result.file_name === "string") &&
    (result.captured_at === undefined || typeof result.captured_at === "string") &&
    (result.detected_at === undefined || typeof result.detected_at === "string") &&
    (result.stable_at === undefined || typeof result.stable_at === "string") &&
    typeof result.validation_only === "boolean"
  );
}

function isReadinessCheck(value: unknown): value is ReadinessCheck {
  if (!value || typeof value !== "object") return false;
  const check = value as ReadinessCheck;
  return (
    typeof check.check_key === "string" &&
    isReadinessStatus(check.status) &&
    typeof check.label === "string" &&
    typeof check.action === "string"
  );
}

function isReadinessStatus(value: unknown): value is ReadinessStatus {
  return (
    value === "ready" ||
    value === "warning" ||
    value === "failed" ||
    value === "unknown"
  );
}

function isDetectedPhoto(value: unknown): value is DetectedPhoto {
  if (!value || typeof value !== "object") return false;
  const photo = value as DetectedPhoto;
  return (
    typeof photo.station_id === "string" &&
    typeof photo.source_path === "string" &&
    typeof photo.size_bytes === "number" &&
    typeof photo.detected_at === "string" &&
    !Number.isNaN(Date.parse(photo.detected_at)) &&
    typeof photo.stable_at === "string" &&
    !Number.isNaN(Date.parse(photo.stable_at)) &&
    photo.status === "detected"
  );
}

function isRouteResult(value: unknown): value is RouteResult {
  if (!value || typeof value !== "object") return false;
  const photo = value as RouteResult;
  if (photo.status === "routed") return isRoutedPhoto(photo);
  if (photo.status === "quarantined") return isQuarantinedPhoto(photo);
  return (
    typeof photo.station_id === "string" &&
    typeof photo.source_path === "string" &&
    typeof photo.source_size_bytes === "number" &&
    typeof photo.detected_at === "string" &&
    !Number.isNaN(Date.parse(photo.detected_at)) &&
    typeof photo.stable_at === "string" &&
    !Number.isNaN(Date.parse(photo.stable_at)) &&
    photo.status === "unassigned_pending_quarantine"
  );
}

function isRoutedPhoto(value: unknown): value is RoutedPhoto {
  if (!value || typeof value !== "object") return false;
  const photo = value as RoutedPhoto;
  return (
    typeof photo.photo_id === "string" &&
    photo.photo_id.startsWith("photo_") &&
    typeof photo.station_id === "string" &&
    typeof photo.session_id === "string" &&
    typeof photo.source_path === "string" &&
    typeof photo.source_size_bytes === "number" &&
    typeof photo.detected_at === "string" &&
    !Number.isNaN(Date.parse(photo.detected_at)) &&
    typeof photo.stable_at === "string" &&
    !Number.isNaN(Date.parse(photo.stable_at)) &&
    typeof photo.routed_at === "string" &&
    !Number.isNaN(Date.parse(photo.routed_at)) &&
    photo.status === "routed" &&
    isGradedProcessingStatus(photo.graded_processing_status)
  );
}

function isGradedProcessingStatus(
  value: unknown,
): value is GradedProcessingStatus {
  return (
    value === "not_eligible" ||
    value === "pending" ||
    value === "processing" ||
    value === "processed" ||
    value === "failed"
  );
}

function isRetryProcessingData(value: unknown): value is RetryProcessingData {
  if (!value || typeof value !== "object") return false;
  const data = value as RetryProcessingData;
  return isRoutedPhoto(data.photo) && typeof data.retry_started === "boolean";
}

function isProcessingQueueData(value: unknown): value is ProcessingQueueData {
  if (!value || typeof value !== "object") return false;
  const data = value as ProcessingQueueData;
  return (
    isProcessingQueueSummary(data.summary) &&
    Array.isArray(data.items) &&
    data.items.every(isProcessingQueueItem)
  );
}

function isProcessingQueueSummary(
  value: unknown,
): value is ProcessingQueueSummary {
  if (!value || typeof value !== "object") return false;
  const summary = value as ProcessingQueueSummary;
  return (
    typeof summary.total === "number" &&
    typeof summary.not_eligible === "number" &&
    typeof summary.pending === "number" &&
    typeof summary.processing === "number" &&
    typeof summary.processed === "number" &&
    typeof summary.failed === "number" &&
    typeof summary.retrying === "number" &&
    (typeof summary.last_updated_at === "undefined" ||
      (typeof summary.last_updated_at === "string" &&
        !Number.isNaN(Date.parse(summary.last_updated_at)))) &&
    (typeof summary.current_job === "undefined" ||
      isProcessingQueueItem(summary.current_job))
  );
}

function isProcessingQueueItem(value: unknown): value is ProcessingQueueItem {
  if (!value || typeof value !== "object") return false;
  const item = value as ProcessingQueueItem;
  return (
    typeof item.photo_id === "string" &&
    typeof item.station_id === "string" &&
    typeof item.session_id === "string" &&
    typeof item.source_path === "string" &&
    (typeof item.local_original_path === "string" ||
      typeof item.local_original_path === "undefined") &&
    (typeof item.local_graded_path === "string" ||
      typeof item.local_graded_path === "undefined") &&
    isOriginalSaveStatus(item.original_save_status) &&
    isProcessingStatus(item.processing_status) &&
    isGradedProcessingStatus(item.graded_processing_status) &&
    (typeof item.graded_last_error === "string" ||
      typeof item.graded_last_error === "undefined") &&
    typeof item.graded_attempt_count === "number" &&
    (typeof item.graded_processing_started_at === "undefined" ||
      (typeof item.graded_processing_started_at === "string" &&
        !Number.isNaN(Date.parse(item.graded_processing_started_at)))) &&
    (typeof item.graded_processed_at === "undefined" ||
      (typeof item.graded_processed_at === "string" &&
        !Number.isNaN(Date.parse(item.graded_processed_at)))) &&
    typeof item.last_updated_at === "string" &&
    !Number.isNaN(Date.parse(item.last_updated_at))
  );
}

function isOriginalSaveStatus(value: unknown): value is OriginalSaveStatus {
  return (
    value === "pending" ||
    value === "saving" ||
    value === "saved_original" ||
    value === "failed"
  );
}

function isProcessingStatus(value: unknown): value is ProcessingStatus {
  return value === "not_eligible" || value === "eligible";
}

function isQuarantinedPhoto(value: unknown): value is QuarantinedPhoto {
  return isQuarantineItem(value) && value.status === "quarantined";
}

function isQuarantineItem(value: unknown): value is QuarantineItem {
  if (!value || typeof value !== "object") return false;
  const photo = value as QuarantineItem;
  const assignmentFieldsValid =
    photo.status === "quarantined" ||
    (typeof photo.assigned_session_id === "string" &&
      typeof photo.assigned_photo_id === "string" &&
      typeof photo.assigned_at === "string" &&
      !Number.isNaN(Date.parse(photo.assigned_at)));
  return (
    typeof photo.quarantine_id === "string" &&
    photo.quarantine_id.startsWith("quar_") &&
    typeof photo.station_id === "string" &&
    (typeof photo.related_session_id === "string" ||
      typeof photo.related_session_id === "undefined") &&
    typeof photo.source_path === "string" &&
    typeof photo.source_size_bytes === "number" &&
    typeof photo.detected_at === "string" &&
    !Number.isNaN(Date.parse(photo.detected_at)) &&
    typeof photo.stable_at === "string" &&
    !Number.isNaN(Date.parse(photo.stable_at)) &&
    typeof photo.quarantined_at === "string" &&
    !Number.isNaN(Date.parse(photo.quarantined_at)) &&
    (photo.reason === "no_active_session" || photo.reason === "late_photo") &&
    (photo.status === "quarantined" || photo.status === "assigned") &&
    assignmentFieldsValid
  );
}

function isEligibleSession(value: unknown): value is EligibleSession {
  if (!value || typeof value !== "object") return false;
  const session = value as EligibleSession;
  return (
    typeof session.session_id === "string" &&
    typeof session.station_id === "string" &&
    (session.status === "active" || session.status === "locked") &&
    typeof session.customer_name === "string" &&
    typeof session.order_number === "string" &&
    session.eligible === true &&
    session.requires_confirmation === true &&
    typeof session.eligibility_reason === "string"
  );
}

function isQuarantineReason(value: unknown): value is QuarantineReason {
  return value === "no_active_session" || value === "late_photo";
}

function isSessionSummary(value: unknown): value is SessionSummary {
  if (!value || typeof value !== "object") return false;
  const summary = value as SessionSummary;
  return (
    typeof summary.local_output_folder === "string" &&
    typeof summary.photo_count === "number" &&
    typeof summary.failures === "number" &&
    typeof summary.quarantine_count === "number" &&
    (typeof summary.station_quarantine_count === "number" ||
      typeof summary.station_quarantine_count === "undefined") &&
    (isQuarantineReason(summary.latest_quarantine_reason) ||
      summary.latest_quarantine_reason === "" ||
      typeof summary.latest_quarantine_reason === "undefined") &&
    isSessionUploadStatus(summary.upload_status) &&
    isCloudTargetStatus(summary.drive_target_status) &&
    optionalString(summary.drive_target_identity) &&
    optionalString(summary.drive_session_folder_id) &&
    optionalString(summary.drive_folder_path) &&
    optionalString(summary.drive_root_folder_id) &&
    optionalString(summary.drive_root_folder_name) &&
    optionalString(summary.drive_last_error_code) &&
    optionalString(summary.drive_last_error_action)
  );
}

function optionalString(value: unknown): value is string | undefined {
  return typeof value === "string" || typeof value === "undefined";
}

function isCloudTargetStatus(value: unknown): value is CloudTargetStatus {
  return (
    value === "not_configured" ||
    value === "pending" ||
    value === "resolving" ||
    value === "ready" ||
    value === "failed" ||
    value === "placeholder"
  );
}

function isSessionUploadStatus(value: unknown): value is SessionUploadStatus {
  return (
    value === "not_configured" ||
    value === "target_pending" ||
    value === "pending" ||
    value === "uploading" ||
    value === "uploaded" ||
    value === "partial_failed" ||
    value === "failed" ||
    value === "pending_local_completion"
  );
}

function isFileUploadJob(value: unknown): value is FileUploadJob {
  if (!value || typeof value !== "object") return false;
  const job = value as FileUploadJob;
  const hasValidStatus =
    job.status === "pending" ||
    job.status === "uploading" ||
    job.status === "retrying" ||
    job.status === "retry_scheduled" ||
    job.status === "uploaded" ||
    job.status === "failed" ||
    job.status === "not_eligible";
  const hasDriveIdentity =
    typeof job.drive_folder_id === "string" ||
    typeof job.drive_file_id === "string";
  const hasLegacyIdentity =
    typeof job.bucket_name === "string" && typeof job.object_key === "string";
  const hasValidUploadIdentity =
    hasDriveIdentity ||
    hasLegacyIdentity ||
    job.status === "not_eligible" ||
    job.status === "failed";
  const hasValidUploadedDriveFile =
    job.status !== "uploaded" || typeof job.drive_file_id === "string";

  return (
    typeof job.job_id === "string" &&
    typeof job.session_id === "string" &&
    typeof job.station_id === "string" &&
    typeof job.photo_id === "string" &&
    (job.asset_kind === "original" || job.asset_kind === "graded") &&
    optionalString(job.local_file_name) &&
    optionalString(job.bucket_name) &&
    optionalString(job.object_key) &&
    optionalString(job.drive_folder_id) &&
    optionalString(job.drive_file_id) &&
    optionalString(job.remote_identity) &&
    hasValidStatus &&
    hasValidUploadIdentity &&
    hasValidUploadedDriveFile &&
    typeof job.attempt_count === "number" &&
    typeof job.created_at === "string" &&
    typeof job.updated_at === "string"
  );
}

function isSessionUploadsData(value: unknown): value is SessionUploadsData {
  if (!value || typeof value !== "object") return false;
  const data = value as SessionUploadsData;
  return (
    typeof data.session_id === "string" &&
    isSessionUploadStatus(data.upload_status) &&
    Array.isArray(data.jobs) &&
    data.jobs.every(isFileUploadJob)
  );
}

function isSessionCloudTarget(value: unknown): value is SessionCloudTarget {
  if (!value || typeof value !== "object") return false;
  const target = value as SessionCloudTarget;
  return (
    typeof target.session_id === "string" &&
    typeof target.station_id === "string" &&
    isCloudTargetStatus(target.status) &&
    (typeof target.object_prefix === "string" ||
      typeof target.object_prefix === "undefined") &&
    (typeof target.bucket_name === "string" ||
      typeof target.bucket_name === "undefined") &&
    (typeof target.last_error_code === "string" ||
      typeof target.last_error_code === "undefined") &&
    (typeof target.last_error_action === "string" ||
      typeof target.last_error_action === "undefined")
  );
}

function isStationQuarantineSummary(
  value: unknown,
): value is StationQuarantineSummary {
  if (!value || typeof value !== "object") return false;
  const summary = value as StationQuarantineSummary;
  return (
    typeof summary.station_id === "string" &&
    typeof summary.station_quarantine_count === "number" &&
    (isQuarantineReason(summary.latest_quarantine_reason) ||
      summary.latest_quarantine_reason === "" ||
      typeof summary.latest_quarantine_reason === "undefined")
  );
}

function isSession(value: unknown): value is Session {
  if (!value || typeof value !== "object") return false;
  const session = value as Session;
  return (
    typeof session.session_id === "string" &&
    typeof session.station_id === "string" &&
    (session.status === "active" || session.status === "locked") &&
    typeof session.customer_name === "string" &&
    typeof session.order_number === "string" &&
    typeof session.timer_seconds === "number" &&
    typeof session.started_at === "string" &&
    !Number.isNaN(Date.parse(session.started_at)) &&
    typeof session.ends_at === "string" &&
    !Number.isNaN(Date.parse(session.ends_at)) &&
    isStationSnapshot(session.station_snapshot) &&
    ((typeof session.ended_at === "string" &&
      !Number.isNaN(Date.parse(session.ended_at))) ||
      session.ended_at === null ||
      typeof session.ended_at === "undefined") &&
    (session.end_reason === "manual" ||
      session.end_reason === "timer" ||
      session.end_reason === null ||
      typeof session.end_reason === "undefined")
  );
}

function isStationSnapshot(value: unknown): value is StationSnapshot {
  if (!value || typeof value !== "object") return false;
  const snapshot = value as StationSnapshot;
  return (
    typeof snapshot.station_name === "string" &&
    typeof snapshot.background_name === "string" &&
    typeof snapshot.default_lut_path === "string" &&
    typeof snapshot.input_folder === "string" &&
    typeof snapshot.output_rule === "string" &&
    typeof snapshot.output_folder === "string" &&
    typeof snapshot.device_identifier === "string"
  );
}

function isWatchValidation(value: unknown): value is WatchValidation {
  if (!value || typeof value !== "object") return false;
  const validation = value as WatchValidation;
  return (
    typeof validation.station_id === "string" &&
    isWatchValidationStatus(validation.status) &&
    typeof validation.label === "string" &&
    typeof validation.action === "string" &&
    (typeof validation.source_path === "string" ||
      validation.source_path === null) &&
    (typeof validation.detected_at === "string" ||
      validation.detected_at === null) &&
    (typeof validation.stable_at === "string" ||
      validation.stable_at === null) &&
    typeof validation.validated_at === "string" &&
    !Number.isNaN(Date.parse(validation.validated_at)) &&
    validation.validation_only === true &&
    (validation.status !== "success" ||
      (typeof validation.source_path === "string" &&
        isValidOptionalDate(validation.detected_at) &&
        isValidOptionalDate(validation.stable_at)))
  );
}

function isValidOptionalDate(value: string | null): boolean {
  return typeof value === "string" && !Number.isNaN(Date.parse(value));
}

function isWatchValidationStatus(
  value: unknown,
): value is WatchValidationStatus {
  return (
    value === "ready" ||
    value === "running" ||
    value === "success" ||
    value === "warning" ||
    value === "failed"
  );
}

function isStationBackupData(value: unknown): value is StationBackupData {
  if (!value || typeof value !== "object") return false;
  const data = value as StationBackupData;
  return (
    !!data.backup &&
    typeof data.backup.filename === "string" &&
    data.backup.filename.endsWith(".json") &&
    typeof data.backup.created_at === "string" &&
    !Number.isNaN(Date.parse(data.backup.created_at)) &&
    data.backup.station_count === 3
  );
}

function isStationRestoreData(value: unknown): value is StationRestoreData {
  if (!value || typeof value !== "object") return false;
  const data = value as StationRestoreData;
  return data.restored === true && data.station_count === 3;
}

function isStation(value: unknown): value is Station {
  if (!value || typeof value !== "object") return false;
  const station = value as Station;
  return (
    typeof station.station_id === "string" &&
    typeof station.name === "string" &&
    typeof station.device_identifier === "string" &&
    typeof station.input_folder === "string" &&
    typeof station.background_name === "string" &&
    typeof station.default_lut_path === "string" &&
    typeof station.output_rule === "string" &&
    (station.camera_assignment === undefined || isCameraAssignment(station.camera_assignment))
  );
}

function isCameraAssignment(value: unknown): value is CameraAssignment {
  if (!value || typeof value !== "object") return false;
  const assignment = value as CameraAssignment;
  return (
    typeof assignment.identity_key === "string" &&
    typeof assignment.camera_name === "string" &&
    typeof assignment.port === "string" &&
    typeof assignment.runtime === "string" &&
    typeof assignment.assigned_at === "string" &&
    typeof assignment.connected === "boolean"
  );
}

function isErrorResponse(
  payload: DataResponse<unknown> | ErrorResponse,
): payload is ErrorResponse {
  return "error" in payload && typeof payload.error?.code === "string";
}
