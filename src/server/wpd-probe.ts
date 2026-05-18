import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { detectUsbCameras } from './usb-camera-detector.js';

export type WpdProbeDevice = {
  deviceId: string;
  friendlyName: string | null;
  manufacturer: string | null;
  description: string | null;
  matchedSony: boolean;
  matchedIlce6000: boolean;
  rawIdHint: string | null;
};

export type WpdShellDevice = {
  name: string;
  path: string | null;
  type: string | null;
};

export type WpdProbeResponse = {
  platform: NodeJS.Platform;
  supported: boolean;
  status: 'ok' | 'unsupported' | 'error';
  scannedAt: string;
  deviceCount: number;
  sonyDeviceCount: number;
  devices: WpdProbeDevice[];
  shellDevices: WpdShellDevice[];
  findings: string[];
  nextActions: string[];
  error: string | null;
};

type RawWpdProbe = {
  managerAvailable?: boolean | null;
  devices?: WpdProbeDevice[] | WpdProbeDevice | null;
  shellDevices?: WpdShellDevice[] | WpdShellDevice | null;
  errors?: string[] | string | null;
};

const WPD_PROBE_SCRIPT = String.raw`
$errors = @()
$devices = @()
$shellDevices = @()
$managerAvailable = $false

function Normalize-String($value) {
  if ($null -eq $value) { return $null }
  $text = ([string]$value).Trim([char]0).Trim()
  if ($text.Length -eq 0) { return $null }
  return $text
}

# NOTE: PortableDeviceApi.PortableDeviceManager can hang on some Windows/WPD stacks.
# This shallow probe intentionally avoids opening the WPD manager. It uses PnP + Shell
# enumeration first so diagnostics stay responsive. A later deep-helper spike can
# move WPD manager access into a short-lived native process with its own watchdog.
try {
  $pattern = 'Sony|ILCE-6000|ILCE 6000|VID_054C&PID_094E|WPD|Portable Device'
  $pnpDevices = Get-PnpDevice -PresentOnly -ErrorAction Stop | Where-Object {
    $_.Class -eq 'WPD' -or (($_.FriendlyName, $_.Class, $_.Manufacturer, $_.InstanceId) -join ' ') -match $pattern
  }

  foreach ($device in $pnpDevices) {
    $id = [string]$device.InstanceId
    $friendlyName = Normalize-String $device.FriendlyName
    $manufacturer = Normalize-String $device.Manufacturer
    $description = Normalize-String (($device.Class, $device.Status) -join ' / ')
    $haystack = (($id, $friendlyName, $manufacturer, $description) -join ' ')
    $devices += [pscustomobject]@{
      deviceId = $id
      friendlyName = $friendlyName
      manufacturer = $manufacturer
      description = $description
      matchedSony = ($haystack -match 'Sony|VID_054C')
      matchedIlce6000 = ($haystack -match 'ILCE[- ]?6000|PID_094E')
      rawIdHint = if ($id.Length -gt 160) { $id.Substring(0, 160) } else { $id }
    }
  }
} catch {
  $errors += "PnP WPD shallow probe failed: $($_.Exception.Message)"
}

# Shell.Application enumeration is intentionally skipped here: on some machines it
# keeps PowerShell alive even after producing JSON. PnP details are more reliable
# for this spike.

[pscustomobject]@{
  managerAvailable = $managerAvailable
  devices = $devices
  shellDevices = $shellDevices
  errors = $errors
} | ConvertTo-Json -Depth 6 -Compress
`;

export async function probeWindowsPortableDevices(): Promise<WpdProbeResponse> {
  if (process.platform !== 'win32') {
    return {
      platform: process.platform,
      supported: false,
      status: 'unsupported',
      scannedAt: new Date().toISOString(),
      deviceCount: 0,
      sonyDeviceCount: 0,
      devices: [],
      shellDevices: [],
      findings: ['WPD probe currently supports Windows only.'],
      nextActions: [],
      error: 'Windows only.'
    };
  }

  try {
    const [raw, usb] = await Promise.all([runWpdProbeScript(), detectUsbCameras({ force: true })]);
    const devices = normalizeArray(raw.devices);
    const shellDevices = normalizeArray(raw.shellDevices);
    const errors = normalizeStrings(raw.errors);
    const usbWpdDevices = usb.devices
      .filter((device) => device.className?.toLowerCase() === 'wpd')
      .map((device) => ({
        deviceId: device.instanceId ?? device.name,
        friendlyName: device.friendlyName ?? device.name,
        manufacturer: device.manufacturer,
        description: `${device.className ?? 'unknown'} / ${device.status ?? 'unknown'} / ${device.source}`,
        matchedSony: /sony|vid_054c/i.test(`${device.name} ${device.manufacturer ?? ''} ${device.instanceId ?? ''}`),
        matchedIlce6000: /ilce[- ]?6000|pid_094e/i.test(`${device.name} ${device.instanceId ?? ''}`),
        rawIdHint: device.instanceId
      }));
    const combinedDevices = dedupeWpdDevices([...devices, ...usbWpdDevices]);
    const sonyDevices = combinedDevices.filter((device) => device.matchedSony || device.matchedIlce6000 || /sony|ilce[- ]?6000/i.test(`${device.friendlyName ?? ''} ${device.manufacturer ?? ''} ${device.description ?? ''}`));

    return {
      platform: process.platform,
      supported: true,
      status: errors.length > 0 && devices.length === 0 ? 'error' : 'ok',
      scannedAt: new Date().toISOString(),
      deviceCount: combinedDevices.length,
      sonyDeviceCount: sonyDevices.length,
      devices: combinedDevices,
      shellDevices,
      findings: buildFindings(Boolean(raw.managerAvailable), combinedDevices.length, sonyDevices.length, errors),
      nextActions: buildNextActions(sonyDevices.length),
      error: errors.length > 0 ? errors.join(' | ') : null
    };
  } catch (error) {
    return {
      platform: process.platform,
      supported: true,
      status: 'error',
      scannedAt: new Date().toISOString(),
      deviceCount: 0,
      sonyDeviceCount: 0,
      devices: [],
      shellDevices: [],
      findings: ['WPD probe crashed before returning usable diagnostic data.'],
      nextActions: ['Check data/logs/debug-events.jsonl for the full error.'],
      error: error instanceof Error ? sanitizeError(error.message) : sanitizeError(String(error))
    };
  }
}

async function runWpdProbeScript(): Promise<RawWpdProbe> {
  const scriptPath = path.join(os.tmpdir(), `selfstudio-wpd-probe-${process.pid}-${Date.now()}.ps1`);
  await fs.writeFile(scriptPath, WPD_PROBE_SCRIPT, 'utf8');
  try {
    return await new Promise((resolve, reject) => {
      execFile(
        'powershell.exe',
        ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', scriptPath],
        { timeout: 12_000, windowsHide: true, maxBuffer: 2 * 1024 * 1024 },
        (error, stdout, stderr) => {
          const output = stdout.trim();
          if (error && !output) {
            reject(new Error(sanitizeError(stderr.trim() || error.message)));
            return;
          }
          const jsonText = extractJsonObject(output);
          try {
            resolve(JSON.parse(jsonText) as RawWpdProbe);
          } catch (parseError) {
            reject(new Error(`WPD probe returned non-JSON output. stderr=${sanitizeError(stderr.trim())}; stdout=${sanitizeError(output.slice(0, 1000))}; parse=${parseError instanceof Error ? parseError.message : String(parseError)}`));
          }
        }
      );
    });
  } finally {
    await fs.rm(scriptPath, { force: true }).catch(() => undefined);
  }
}

function dedupeWpdDevices(devices: WpdProbeDevice[]): WpdProbeDevice[] {
  const byId = new Map<string, WpdProbeDevice>();
  for (const device of devices) {
    const key = (device.deviceId || device.friendlyName || '').toLowerCase();
    if (!key) {
      continue;
    }
    const current = byId.get(key);
    byId.set(key, current ? {
      ...current,
      friendlyName: current.friendlyName ?? device.friendlyName,
      manufacturer: current.manufacturer ?? device.manufacturer,
      description: current.description ?? device.description,
      matchedSony: current.matchedSony || device.matchedSony,
      matchedIlce6000: current.matchedIlce6000 || device.matchedIlce6000,
      rawIdHint: current.rawIdHint ?? device.rawIdHint
    } : device);
  }
  return Array.from(byId.values());
}

function extractJsonObject(output: string): string {
  const first = output.indexOf('{');
  const last = output.lastIndexOf('}');
  if (first === -1 || last === -1 || last < first) {
    return output;
  }
  return output.slice(first, last + 1);
}

function normalizeArray<T>(value: T[] | T | null | undefined): T[] {
  if (!value) {
    return [];
  }
  return Array.isArray(value) ? value : [value];
}

function normalizeStrings(value: string[] | string | null | undefined): string[] {
  return normalizeArray(value).map((item) => sanitizeError(String(item))).filter(Boolean);
}

function buildFindings(
  managerAvailable: boolean,
  deviceCount: number,
  sonyDeviceCount: number,
  errors: string[]
): string[] {
  const findings: string[] = [];
  findings.push(managerAvailable ? 'Windows PortableDeviceManager COM object was opened.' : 'PortableDeviceManager deep COM open was intentionally skipped to avoid WPD hangs; shallow PnP/Shell probe was used.');
  findings.push(`WPD shallow probe returned ${deviceCount} device(s).`);
  findings.push(`Sony/ILCE match count: ${sonyDeviceCount}.`);
  if (sonyDeviceCount > 0) {
    findings.push('This confirms Windows exposes the Sony camera through WPD/PnP without changing drivers.');
    findings.push('Next technical uncertainty: whether a dedicated helper can safely open PortableDeviceManager/IPortableDevice and access command transport without hanging.');
  }
  if (errors.length > 0) {
    findings.push(`Probe errors: ${errors.join(' | ')}`);
  }
  return findings;
}

function buildNextActions(sonyDeviceCount: number): string[] {
  if (sonyDeviceCount === 0) {
    return [
      'Keep camera in PC Remote mode and rerun /api/sony-ptp/wpd-probe.',
      'Confirm ILCE-6000 still appears in /api/sony-ptp/native-feasibility.'
    ];
  }

  return [
    'Use the returned WPD deviceId as the target for a dedicated WPD command helper.',
    'Next spike should attempt opening IPortableDevice and listing supported commands/content objects.',
    'Only after supported commands are visible should we attempt shutter/capture/download.'
  ];
}

function sanitizeError(error: string): string {
  return error.replace(/[A-Z]:\\[^\s"']+/gi, '[local-path]').slice(0, 1000);
}
