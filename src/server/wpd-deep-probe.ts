import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

export type WpdDeepProbeResponse = {
  platform: NodeJS.Platform;
  supported: boolean;
  status: 'ok' | 'timeout' | 'unsupported' | 'error';
  scannedAt: string;
  requestedDeviceId: string | null;
  elapsedMs: number;
  managerOpened: boolean;
  deviceCount: number;
  matchedDevice: WpdDeepProbeDevice | null;
  devices: WpdDeepProbeDevice[];
  findings: string[];
  nextActions: string[];
  error: string | null;
};

export type WpdDeepProbeDevice = {
  deviceId: string;
  friendlyName: string | null;
  manufacturer: string | null;
  description: string | null;
  matchedRequested: boolean;
  matchedSony: boolean;
  matchedIlce6000: boolean;
};

type RawDeepProbe = {
  managerOpened?: boolean | null;
  devices?: WpdDeepProbeDevice[] | WpdDeepProbeDevice | null;
  errors?: string[] | string | null;
};

const DEEP_PROBE_TIMEOUT_MS = 5_000;

const WPD_DEEP_PROBE_SCRIPT = String.raw`
param(
  [string]$RequestedDeviceId = ''
)

$errors = @()
$devices = @()
$managerOpened = $false

function Normalize-String($value) {
  if ($null -eq $value) { return $null }
  $text = ([string]$value).Trim([char]0).Trim()
  if ($text.Length -eq 0) { return $null }
  return $text
}

function Get-WpdString($Manager, $DeviceId, $Kind) {
  try {
    $length = 0
    if ($Kind -eq 'friendlyName') {
      $null = $Manager.GetDeviceFriendlyName($DeviceId, $null, [ref]$length)
    } elseif ($Kind -eq 'manufacturer') {
      $null = $Manager.GetDeviceManufacturer($DeviceId, $null, [ref]$length)
    } elseif ($Kind -eq 'description') {
      $null = $Manager.GetDeviceDescription($DeviceId, $null, [ref]$length)
    } else {
      return $null
    }

    if ($length -le 0) { return $null }
    $buffer = New-Object char[] $length

    if ($Kind -eq 'friendlyName') {
      $null = $Manager.GetDeviceFriendlyName($DeviceId, $buffer, [ref]$length)
    } elseif ($Kind -eq 'manufacturer') {
      $null = $Manager.GetDeviceManufacturer($DeviceId, $buffer, [ref]$length)
    } elseif ($Kind -eq 'description') {
      $null = $Manager.GetDeviceDescription($DeviceId, $buffer, [ref]$length)
    }

    return Normalize-String (-join $buffer)
  } catch {
    return $null
  }
}

try {
  $manager = New-Object -ComObject PortableDeviceApi.PortableDeviceManager
  $managerOpened = $true

  $count = 0
  $null = $manager.GetDevices($null, [ref]$count)

  if ($count -gt 0) {
    $ids = New-Object string[] $count
    $null = $manager.GetDevices($ids, [ref]$count)

    foreach ($id in $ids) {
      if ($null -eq $id) { continue }
      $friendlyName = Get-WpdString $manager $id 'friendlyName'
      $manufacturer = Get-WpdString $manager $id 'manufacturer'
      $description = Get-WpdString $manager $id 'description'
      $haystack = (($id, $friendlyName, $manufacturer, $description) -join ' ')
      $devices += [pscustomobject]@{
        deviceId = [string]$id
        friendlyName = $friendlyName
        manufacturer = $manufacturer
        description = $description
        matchedRequested = ($RequestedDeviceId.Length -gt 0 -and ([string]$id).ToLowerInvariant() -eq $RequestedDeviceId.ToLowerInvariant())
        matchedSony = ($haystack -match 'Sony|VID_054C')
        matchedIlce6000 = ($haystack -match 'ILCE[- ]?6000|PID_094E')
      }
    }
  }
} catch {
  $errors += "PortableDeviceManager deep probe failed: $($_.Exception.Message)"
}

[pscustomobject]@{
  managerOpened = $managerOpened
  devices = $devices
  errors = $errors
} | ConvertTo-Json -Depth 6 -Compress
`;

export async function probeWpdManagerDeep(deviceId: string | null): Promise<WpdDeepProbeResponse> {
  const startedAt = Date.now();
  const requestedDeviceId = normalizeDeviceId(deviceId);

  if (process.platform !== 'win32') {
    return {
      platform: process.platform,
      supported: false,
      status: 'unsupported',
      scannedAt: new Date().toISOString(),
      requestedDeviceId,
      elapsedMs: Date.now() - startedAt,
      managerOpened: false,
      deviceCount: 0,
      matchedDevice: null,
      devices: [],
      findings: ['WPD deep probe currently supports Windows only.'],
      nextActions: [],
      error: 'Windows only.'
    };
  }

  try {
    const raw = await runDeepProbeScript(requestedDeviceId);
    const devices = normalizeArray(raw.devices);
    const errors = normalizeStrings(raw.errors);
    const matchedDevice = findMatchedDevice(devices, requestedDeviceId);
    const managerOpened = raw.managerOpened === true;
    const status: WpdDeepProbeResponse['status'] = errors.length > 0 && devices.length === 0 ? 'error' : 'ok';

    return {
      platform: process.platform,
      supported: true,
      status,
      scannedAt: new Date().toISOString(),
      requestedDeviceId,
      elapsedMs: Date.now() - startedAt,
      managerOpened,
      deviceCount: devices.length,
      matchedDevice,
      devices,
      findings: buildFindings(managerOpened, devices, matchedDevice, errors),
      nextActions: buildNextActions(managerOpened, matchedDevice, errors),
      error: errors.length > 0 ? errors.join(' | ') : null
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    const timedOut = /timed out|timeout|ETIMEDOUT|SIGTERM/i.test(message);
    return {
      platform: process.platform,
      supported: true,
      status: timedOut ? 'timeout' : 'error',
      scannedAt: new Date().toISOString(),
      requestedDeviceId,
      elapsedMs: Date.now() - startedAt,
      managerOpened: false,
      deviceCount: 0,
      matchedDevice: null,
      devices: [],
      findings: [
        timedOut
          ? 'PortableDeviceManager deep probe timed out. This confirms WPD deep COM access can hang and must be isolated in a watchdog helper.'
          : 'PortableDeviceManager deep probe failed before returning usable JSON.'
      ],
      nextActions: [
        'Do not run deep WPD access inline in the web server.',
        'Use a short-lived helper process with timeout and process kill semantics for any future IPortableDevice command probing.',
        'If deep WPD keeps timing out, switch to libgphoto2/libusb/WinUSB or Sony tooling research instead of WPD COM.'
      ],
      error: sanitizeError(message)
    };
  }
}

async function runDeepProbeScript(requestedDeviceId: string | null): Promise<RawDeepProbe> {
  const scriptPath = path.join(os.tmpdir(), `selfstudio-wpd-deep-probe-${process.pid}-${Date.now()}.ps1`);
  await fs.writeFile(scriptPath, WPD_DEEP_PROBE_SCRIPT, 'utf8');
  try {
    return await new Promise((resolve, reject) => {
      const args = ['-Sta', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', scriptPath];
      if (requestedDeviceId) {
        args.push('-RequestedDeviceId', requestedDeviceId);
      }

      execFile(
        'powershell.exe',
        args,
        { timeout: DEEP_PROBE_TIMEOUT_MS, killSignal: 'SIGKILL', windowsHide: true, maxBuffer: 2 * 1024 * 1024 },
        (error, stdout, stderr) => {
          const output = stdout.trim();
          if (error && !output) {
            reject(new Error(sanitizeError(stderr.trim() || error.message)));
            return;
          }
          const jsonText = extractJsonObject(output);
          try {
            resolve(JSON.parse(jsonText) as RawDeepProbe);
          } catch (parseError) {
            reject(new Error(`WPD deep probe returned non-JSON output. stderr=${sanitizeError(stderr.trim())}; stdout=${sanitizeError(output.slice(0, 1000))}; parse=${parseError instanceof Error ? parseError.message : String(parseError)}`));
          }
        }
      );
    });
  } finally {
    await fs.rm(scriptPath, { force: true }).catch(() => undefined);
  }
}

function findMatchedDevice(
  devices: WpdDeepProbeDevice[],
  requestedDeviceId: string | null
): WpdDeepProbeDevice | null {
  return devices.find((device) => device.matchedRequested) ??
    devices.find((device) => device.matchedSony || device.matchedIlce6000) ??
    (requestedDeviceId ? devices.find((device) => device.deviceId.toLowerCase() === requestedDeviceId.toLowerCase()) : undefined) ??
    null;
}

function buildFindings(
  managerOpened: boolean,
  devices: WpdDeepProbeDevice[],
  matchedDevice: WpdDeepProbeDevice | null,
  errors: string[]
): string[] {
  const findings = [
    managerOpened ? 'PortableDeviceManager COM object opened successfully.' : 'PortableDeviceManager COM object did not open.',
    `PortableDeviceManager returned ${devices.length} device(s).`,
    matchedDevice ? 'Sony/ILCE target matched in PortableDeviceManager output.' : 'Sony/ILCE target was not matched in PortableDeviceManager output.'
  ];

  if (matchedDevice) {
    findings.push('Next native step is IPortableDevice open + supported command/content enumeration in a compiled helper.');
  }

  if (errors.length > 0) {
    findings.push(`Errors: ${errors.join(' | ')}`);
  }

  return findings;
}

function buildNextActions(
  managerOpened: boolean,
  matchedDevice: WpdDeepProbeDevice | null,
  errors: string[]
): string[] {
  if (!managerOpened || errors.length > 0) {
    return [
      'Keep using the shallow WPD/PnP identity as the reliable diagnostic source.',
      'Use a compiled helper for deeper WPD access instead of PowerShell COM.'
    ];
  }

  if (!matchedDevice) {
    return [
      'Compare PortableDeviceManager device ids with PnP instance id.',
      'If Sony is missing here, WPD COM may not expose the camera to this process context.'
    ];
  }

  return [
    'Implement a compiled WPD helper that opens this device id through IPortableDevice.',
    'List supported commands and content objects before attempting capture.',
    'Only attempt shutter/download if command discovery shows an available PTP/Sony operation path.'
  ];
}

function normalizeDeviceId(value: string | null): string | null {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
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

function sanitizeError(error: string): string {
  return error.replace(/[A-Z]:\\[^\s"']+/gi, '[local-path]').slice(0, 1200);
}
