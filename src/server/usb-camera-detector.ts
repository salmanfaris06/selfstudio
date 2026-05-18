import { execFile } from 'node:child_process';

export type UsbCameraDevice = {
  name: string;
  friendlyName: string | null;
  className: string | null;
  manufacturer: string | null;
  status: string | null;
  present: boolean | null;
  instanceId: string | null;
  source: string;
  matchedTerms: string[];
};

export type UsbCameraDetectionResponse = {
  platform: NodeJS.Platform;
  supported: boolean;
  status: 'ok' | 'unsupported' | 'error';
  scannedAt: string;
  detectedCount: number;
  devices: UsbCameraDevice[];
  error?: string;
};

type RawDevice = {
  name?: string | null;
  friendlyName?: string | null;
  className?: string | null;
  manufacturer?: string | null;
  status?: string | null;
  present?: boolean | null;
  instanceId?: string | null;
  source?: string | null;
};

type PowerShellScanResult = {
  devices?: RawDevice[] | RawDevice | null;
  errors?: string[] | string | null;
};

type MatchRule = {
  label: string;
  test: (text: string, device: NormalizedRawDevice) => boolean;
};

type NormalizedRawDevice = {
  name: string | null;
  friendlyName: string | null;
  className: string | null;
  manufacturer: string | null;
  status: string | null;
  present: boolean | null;
  instanceId: string | null;
  source: string | null;
};

const CACHE_TTL_MS = 10_000;
let cachedResponse: UsbCameraDetectionResponse | null = null;
let cachedAt = 0;
let inFlightScan: Promise<UsbCameraDetectionResponse> | null = null;

const MATCH_RULES: MatchRule[] = [
  { label: 'sony-usb', test: (text, device) => isUsbDevice(device) && /\bsony\b/i.test(text) },
  { label: 'a6000-usb', test: (text, device) => isUsbDevice(device) && /\ba6000\b/i.test(text) },
  { label: 'ilce-6000-usb', test: (text, device) => isUsbDevice(device) && /\bilce[- ]?6000\b/i.test(text) },
  { label: 'usb-camera', test: (text, device) => isUsbDevice(device) && /\bcamera\b/i.test(text) },
  { label: 'usb-imaging', test: (text, device) => isUsbDevice(device) && /\bimaging\b|\bimage\b/i.test(text) },
  { label: 'usb-mtp', test: (text, device) => isUsbDevice(device) && hasCameraIdentity(text) && /\bmtp\b/i.test(text) },
  { label: 'usb-ptp', test: (text, device) => isUsbDevice(device) && hasCameraIdentity(text) && /\bptp\b/i.test(text) },
  { label: 'usb-still-image', test: (text, device) => isUsbDevice(device) && /usb still image|still image|digital still/i.test(text) },
  { label: 'camera-class', test: (_text, device) => isUsbDevice(device) && equalsIgnoreCase(device.className, 'Camera') },
  { label: 'image-class', test: (_text, device) => isUsbDevice(device) && equalsIgnoreCase(device.className, 'Image') }
];

const POWERSHELL_SCAN_SCRIPT = String.raw`
$pattern = 'Sony|A6000|ILCE-6000|ILCE 6000|Camera|Imaging|Image|MTP|PTP|USB Still Image|Still Image|Digital Still|Portable Device|WPD'
$items = @()
$errors = @()

try {
  $pnpDevices = Get-PnpDevice -PresentOnly -ErrorAction Stop | Where-Object {
    (($_.FriendlyName, $_.Class, $_.Manufacturer, $_.InstanceId) -join ' ') -match $pattern
  }
  foreach ($device in $pnpDevices) {
    $items += [pscustomobject]@{
      name = $device.FriendlyName
      friendlyName = $device.FriendlyName
      className = $device.Class
      manufacturer = $device.Manufacturer
      status = $device.Status
      present = $true
      instanceId = $device.InstanceId
      source = 'Get-PnpDevice'
    }
  }
} catch {
  $errors += "Get-PnpDevice failed: $($_.Exception.Message)"
}

try {
  $cimDevices = Get-CimInstance Win32_PnPEntity -ErrorAction Stop | Where-Object {
    $_.ConfigManagerErrorCode -eq 0 -and (($_.Name, $_.PNPClass, $_.Manufacturer, $_.DeviceID) -join ' ') -match $pattern
  }
  foreach ($device in $cimDevices) {
    $items += [pscustomobject]@{
      name = $device.Name
      friendlyName = $device.Name
      className = $device.PNPClass
      manufacturer = $device.Manufacturer
      status = $device.Status
      present = $true
      instanceId = $device.DeviceID
      source = 'Win32_PnPEntity'
    }
  }
} catch {
  $errors += "Win32_PnPEntity failed: $($_.Exception.Message)"
}

[pscustomobject]@{
  devices = $items
  errors = $errors
} | ConvertTo-Json -Depth 5 -Compress
`;

export async function detectUsbCameras(options: { force?: boolean } = {}): Promise<UsbCameraDetectionResponse> {
  const now = Date.now();
  if (!options.force && cachedResponse && now - cachedAt < CACHE_TTL_MS) {
    return cachedResponse;
  }

  if (inFlightScan) {
    return inFlightScan;
  }

  inFlightScan = runDetection();
  try {
    const result = await inFlightScan;
    if (result.status === 'ok' && !result.error) {
      cachedResponse = result;
      cachedAt = Date.now();
    }
    return result;
  } finally {
    inFlightScan = null;
  }
}

async function runDetection(): Promise<UsbCameraDetectionResponse> {
  if (process.platform !== 'win32') {
    return {
      platform: process.platform,
      supported: false,
      status: 'unsupported',
      scannedAt: new Date().toISOString(),
      detectedCount: 0,
      devices: [],
      error: 'USB camera detection spike currently supports Windows only.'
    };
  }

  try {
    const stdout = await runPowerShellScan();
    const { rawDevices, errors } = parsePowerShellJson(stdout);
    const devices = normalizeAndDedupeDevices(rawDevices);
    const error = errors.length > 0 ? errors.join(' | ') : undefined;

    return {
      platform: process.platform,
      supported: true,
      status: error ? 'error' : 'ok',
      scannedAt: new Date().toISOString(),
      detectedCount: devices.length,
      devices,
      ...(error ? { error } : {})
    };
  } catch (error) {
    return {
      platform: process.platform,
      supported: true,
      status: 'error',
      scannedAt: new Date().toISOString(),
      detectedCount: 0,
      devices: [],
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

function runPowerShellScan(): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      'powershell.exe',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', POWERSHELL_SCAN_SCRIPT],
      { timeout: 8_000, windowsHide: true, maxBuffer: 1024 * 1024 },
      (error, stdout, stderr) => {
        if (error) {
          reject(new Error(sanitizeError(stderr.trim() || error.message)));
          return;
        }
        resolve(stdout.trim());
      }
    );
  });
}

function parsePowerShellJson(stdout: string): { rawDevices: RawDevice[]; errors: string[] } {
  if (!stdout) {
    return { rawDevices: [], errors: ['PowerShell scan returned no output'] };
  }

  const parsed = JSON.parse(stdout) as PowerShellScanResult | null;
  if (!parsed || typeof parsed !== 'object') {
    return { rawDevices: [], errors: ['PowerShell scan returned an invalid response'] };
  }

  return {
    rawDevices: normalizeRawDeviceArray(parsed.devices),
    errors: normalizeErrorArray(parsed.errors)
  };
}

function normalizeRawDeviceArray(devices: PowerShellScanResult['devices']): RawDevice[] {
  if (!devices) {
    return [];
  }
  const values = Array.isArray(devices) ? devices : [devices];
  return values.filter((device): device is RawDevice => Boolean(device) && typeof device === 'object');
}

function normalizeErrorArray(errors: PowerShellScanResult['errors']): string[] {
  if (!errors) {
    return [];
  }
  const values = Array.isArray(errors) ? errors : [errors];
  return values.map((error) => sanitizeError(String(error))).filter(Boolean);
}

function normalizeAndDedupeDevices(rawDevices: RawDevice[]): UsbCameraDevice[] {
  const devicesByKey = new Map<string, UsbCameraDevice>();

  for (const raw of rawDevices) {
    const device = normalizeRawDevice(raw);
    if (device.present === false || equalsIgnoreCase(device.status, 'Error') || isKnownNonCamera(device)) {
      continue;
    }

    const searchable = [
      device.name,
      device.friendlyName,
      device.className,
      device.manufacturer,
      device.instanceId
    ]
      .filter(Boolean)
      .join(' ');

    const matchedTerms = MATCH_RULES.filter((rule) => rule.test(searchable, device)).map(
      (rule) => rule.label
    );
    if (matchedTerms.length === 0) {
      continue;
    }

    const name = device.name ?? device.friendlyName ?? 'Unknown USB camera-like device';
    const key = normalizeDeviceKey(device.instanceId ?? `${name}|${device.className ?? ''}`);
    const current = devicesByKey.get(key);
    const next = toUsbCameraDevice(name, device, matchedTerms);

    devicesByKey.set(key, current ? mergeDeviceRecords(current, next) : next);
  }

  return Array.from(devicesByKey.values()).sort((a, b) => a.name.localeCompare(b.name));
}

function normalizeRawDevice(raw: RawDevice): NormalizedRawDevice {
  return {
    name: toStringOrNull(raw.name),
    friendlyName: toStringOrNull(raw.friendlyName),
    className: toStringOrNull(raw.className),
    manufacturer: toStringOrNull(raw.manufacturer),
    status: toStringOrNull(raw.status),
    present: typeof raw.present === 'boolean' ? raw.present : null,
    instanceId: toStringOrNull(raw.instanceId),
    source: toStringOrNull(raw.source)
  };
}

function toUsbCameraDevice(
  name: string,
  device: NormalizedRawDevice,
  matchedTerms: string[]
): UsbCameraDevice {
  return {
    name,
    friendlyName: device.friendlyName,
    className: device.className,
    manufacturer: device.manufacturer,
    status: device.status,
    present: device.present,
    instanceId: device.instanceId,
    source: device.source ?? 'unknown',
    matchedTerms
  };
}

function mergeDeviceRecords(current: UsbCameraDevice, next: UsbCameraDevice): UsbCameraDevice {
  return {
    name: current.name !== 'Unknown USB camera-like device' ? current.name : next.name,
    friendlyName: current.friendlyName ?? next.friendlyName,
    className: current.className ?? next.className,
    manufacturer: current.manufacturer ?? next.manufacturer,
    status: current.status ?? next.status,
    present: current.present ?? next.present,
    instanceId: current.instanceId ?? next.instanceId,
    source: Array.from(new Set([...current.source.split(', '), next.source])).join(', '),
    matchedTerms: Array.from(new Set([...current.matchedTerms, ...next.matchedTerms]))
  };
}

function isUsbDevice(device: NormalizedRawDevice): boolean {
  return /^(USB|USBSTOR|WPD\\)/i.test(device.instanceId ?? '');
}

function hasCameraIdentity(text: string): boolean {
  return /\bsony\b|\ba6000\b|\bilce[- ]?6000\b|\bcamera\b|\bimaging\b|\bimage\b|still image|digital still/i.test(text);
}

function isKnownNonCamera(device: NormalizedRawDevice): boolean {
  const text = [device.name, device.friendlyName, device.className, device.manufacturer, device.instanceId]
    .filter(Boolean)
    .join(' ');

  return /WAN Miniport|PPTP|virtual camera|SWD\\VCAMDEVAPI/i.test(text);
}

function equalsIgnoreCase(value: string | null, expected: string): boolean {
  return value?.toLowerCase() === expected.toLowerCase();
}

function normalizeDeviceKey(key: string): string {
  return key.trim().toLowerCase();
}

function sanitizeError(error: string): string {
  return error.replace(/[A-Z]:\\[^\s"']+/gi, '[local-path]').slice(0, 500);
}

function toStringOrNull(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}
