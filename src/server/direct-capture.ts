import { constants as fsConstants } from 'node:fs';
import { execFile } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import type { StationConfig } from './stations.js';

export type DirectCaptureDevice = {
  id: string;
  name: string;
  type: string | null;
  canCapture: boolean;
};

export type DirectCaptureCapabilities = {
  platform: NodeJS.Platform;
  supported: boolean;
  status: 'ok' | 'unsupported' | 'error';
  scannedAt: string;
  devices: DirectCaptureDevice[];
  captureCapableCount: number;
  error: string | null;
};

export type DirectCaptureResult =
  | {
      success: true;
      stationId: StationConfig['id'];
      deviceName: string;
      destinationPath: string;
      filename: string;
      fileSize: number;
      modifiedTime: string;
    }
  | {
      success: false;
      error: string;
    };

type RawWiaDevice = {
  id?: string | null;
  name?: string | null;
  type?: string | number | null;
  canCapture?: boolean | null;
};

type RawWiaCapabilityResult = {
  devices?: RawWiaDevice[] | RawWiaDevice | null;
  errors?: string[] | string | null;
};

type RawCaptureResult = {
  success?: boolean;
  error?: string | null;
  tempPath?: string | null;
  deviceName?: string | null;
};

const CAPABILITY_CACHE_TTL_MS = 10_000;
let cachedCapabilities: DirectCaptureCapabilities | null = null;
let cachedAt = 0;
let capabilityInFlight: Promise<DirectCaptureCapabilities> | null = null;

const WIA_CAPABILITY_SCRIPT = String.raw`
$items = @()
$errors = @()
try {
  $manager = New-Object -ComObject WIA.DeviceManager
  foreach ($info in $manager.DeviceInfos) {
    try { $name = $info.Properties.Item('Name').Value } catch { $name = $info.DeviceID }
    try { $type = [string]$info.Type } catch { $type = 'unknown' }
    $canCapture = $false
    try {
      $device = $info.Connect()
      foreach ($command in $device.Commands) {
        if ([string]$command.CommandID -eq '{AF933CAC-ACAD-11D2-A093-00C04F72DC3C}' -or [string]$command.Name -match 'Take|Capture|Picture') {
          $canCapture = $true
        }
      }
    } catch {
      $errors += ('Connect failed for ' + $name + ': ' + $_.Exception.Message)
    }
    $items += [pscustomobject]@{
      id = $info.DeviceID
      name = $name
      type = $type
      canCapture = $canCapture
    }
  }
} catch {
  $errors += ('WIA DeviceManager failed: ' + $_.Exception.Message)
}
[pscustomobject]@{
  devices = $items
  errors = $errors
} | ConvertTo-Json -Depth 5 -Compress
`;

const WIA_CAPTURE_SCRIPT = String.raw`
param($OutputPath)
$takePictureCommand = '{AF933CAC-ACAD-11D2-A093-00C04F72DC3C}'
try {
  $manager = New-Object -ComObject WIA.DeviceManager
  foreach ($info in $manager.DeviceInfos) {
    try { $name = $info.Properties.Item('Name').Value } catch { $name = $info.DeviceID }
    try {
      $device = $info.Connect()
      $commandToRun = $null
      foreach ($command in $device.Commands) {
        if ([string]$command.CommandID -eq $takePictureCommand -or [string]$command.Name -match 'Take|Capture|Picture') {
          $commandToRun = $command
          break
        }
      }
      if ($null -eq $commandToRun) {
        continue
      }
      $item = $device.ExecuteCommand($commandToRun.CommandID)
      if ($null -eq $item) {
        throw 'Capture command returned no image item.'
      }
      $imageFile = $item.Transfer()
      if ($null -eq $imageFile) {
        throw 'Image transfer returned no file.'
      }
      $imageFile.SaveFile($OutputPath)
      [pscustomobject]@{
        success = $true
        tempPath = $OutputPath
        deviceName = $name
      } | ConvertTo-Json -Compress
      exit 0
    } catch {
      $lastError = ('Capture failed for ' + $name + ': ' + $_.Exception.Message)
    }
  }
  if ($lastError) {
    throw $lastError
  }
  throw 'No WIA capture-capable camera was found.'
} catch {
  [pscustomobject]@{
    success = $false
    error = $_.Exception.Message
  } | ConvertTo-Json -Compress
  exit 0
}
`;

export async function getDirectCaptureCapabilities(
  options: { force?: boolean } = {}
): Promise<DirectCaptureCapabilities> {
  const now = Date.now();
  if (!options.force && cachedCapabilities && now - cachedAt < CAPABILITY_CACHE_TTL_MS) {
    return cachedCapabilities;
  }

  if (capabilityInFlight) {
    return capabilityInFlight;
  }

  capabilityInFlight = readCapabilities();
  try {
    const result = await capabilityInFlight;
    if (result.status === 'ok') {
      cachedCapabilities = result;
      cachedAt = Date.now();
    }
    return result;
  } finally {
    capabilityInFlight = null;
  }
}

export async function capturePhotoToStation(
  stationId: string,
  stations: StationConfig[]
): Promise<DirectCaptureResult> {
  try {
    const station = stations.find((candidate) => candidate.id === stationId.trim());
    if (!station) {
      return { success: false, error: `Invalid station id: ${stationId}` };
    }

    if (process.platform !== 'win32') {
      return { success: false, error: 'Direct capture spike currently supports Windows only.' };
    }

    await assertStationInputFolderReady(station.inputPath);

    const tempDir = path.join(process.cwd(), 'data', 'tmp', 'direct-capture');
    await fs.mkdir(tempDir, { recursive: true });
    const tempPath = path.join(tempDir, `capture-${crypto.randomUUID()}.jpg`);
    const stdout = await runPowerShell(WIA_CAPTURE_SCRIPT, [tempPath], 20_000);
    const raw = parseCaptureJson(stdout);

    if (!raw.success || !raw.tempPath) {
      return { success: false, error: raw.error || 'Direct capture failed without a detailed error.' };
    }

    try {
      await assertCapturedJpgReady(raw.tempPath);
      const destinationPath = await copyCaptureToStation(station.inputPath, raw.tempPath);
      const stat = await fs.stat(destinationPath);

      return {
        success: true,
        stationId: station.id,
        deviceName: raw.deviceName || 'WIA camera',
        destinationPath,
        filename: path.basename(destinationPath),
        fileSize: stat.size,
        modifiedTime: stat.mtime.toISOString()
      };
    } finally {
      await fs.rm(raw.tempPath, { force: true });
    }
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

async function readCapabilities(): Promise<DirectCaptureCapabilities> {
  if (process.platform !== 'win32') {
    return {
      platform: process.platform,
      supported: false,
      status: 'unsupported',
      scannedAt: new Date().toISOString(),
      devices: [],
      captureCapableCount: 0,
      error: 'Direct capture spike currently supports Windows only.'
    };
  }

  try {
    const stdout = await runPowerShell(WIA_CAPABILITY_SCRIPT, [], 10_000);
    const parsed = parseCapabilityJson(stdout);
    const devices = normalizeWiaDevices(parsed.devices);
    const errors = normalizeErrorArray(parsed.errors);

    return {
      platform: process.platform,
      supported: true,
      status: errors.length > 0 && devices.length === 0 ? 'error' : 'ok',
      scannedAt: new Date().toISOString(),
      devices,
      captureCapableCount: devices.filter((device) => device.canCapture).length,
      error: errors.length > 0 ? errors.join(' | ') : null
    };
  } catch (error) {
    return {
      platform: process.platform,
      supported: true,
      status: 'error',
      scannedAt: new Date().toISOString(),
      devices: [],
      captureCapableCount: 0,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

function runPowerShell(script: string, args: string[] = [], timeout = 10_000): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      'powershell.exe',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', script, ...args],
      { timeout, windowsHide: true, maxBuffer: 1024 * 1024 },
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

function parseCapabilityJson(stdout: string): RawWiaCapabilityResult {
  if (!stdout) {
    return { devices: [], errors: ['WIA capability probe returned no output'] };
  }
  return JSON.parse(extractJsonObject(stdout)) as RawWiaCapabilityResult;
}

function parseCaptureJson(stdout: string): RawCaptureResult {
  if (!stdout) {
    return { success: false, error: 'Direct capture command returned no output' };
  }
  return JSON.parse(extractJsonObject(stdout)) as RawCaptureResult;
}

function extractJsonObject(stdout: string): string {
  const start = stdout.indexOf('{');
  const end = stdout.lastIndexOf('}');
  if (start === -1 || end === -1 || end <= start) {
    throw new Error('PowerShell did not return a JSON object');
  }
  return stdout.slice(start, end + 1);
}

function normalizeWiaDevices(devices: RawWiaCapabilityResult['devices']): DirectCaptureDevice[] {
  if (!devices) {
    return [];
  }
  const values = Array.isArray(devices) ? devices : [devices];
  return values
    .filter((device): device is RawWiaDevice => Boolean(device) && typeof device === 'object')
    .map((device) => ({
      id: toStringOrNull(device.id) ?? 'unknown',
      name: toStringOrNull(device.name) ?? 'Unknown WIA device',
      type: toStringOrNull(device.type),
      canCapture: device.canCapture === true
    }));
}

function normalizeErrorArray(errors: RawWiaCapabilityResult['errors']): string[] {
  if (!errors) {
    return [];
  }
  const values = Array.isArray(errors) ? errors : [errors];
  return values.map((error) => sanitizeError(String(error))).filter(Boolean);
}

async function assertStationInputFolderReady(inputPath: string): Promise<void> {
  const stat = await fs.stat(inputPath);
  if (!stat.isDirectory()) {
    throw new Error(`Station input path is not a directory: ${inputPath}`);
  }
}

async function assertCapturedJpgReady(filePath: string): Promise<void> {
  const stat = await fs.stat(filePath);
  if (stat.size === 0) {
    throw new Error('Captured file is empty.');
  }

  const handle = await fs.open(filePath, 'r');
  try {
    const buffer = Buffer.alloc(3);
    await handle.read(buffer, 0, 3, 0);
    if (buffer[0] !== 0xff || buffer[1] !== 0xd8 || buffer[2] !== 0xff) {
      throw new Error('Captured file is not a JPEG.');
    }
  } finally {
    await handle.close();
  }
}

async function copyCaptureToStation(inputPath: string, sourcePath: string): Promise<string> {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  const baseName = `direct-capture-${timestamp}`;

  for (let index = 0; index < 1000; index += 1) {
    const suffix = index === 0 ? '' : `-${index}`;
    const destinationPath = path.join(inputPath, `${baseName}${suffix}.jpg`);
    assertDestinationInsideStation(inputPath, destinationPath);
    try {
      await fs.copyFile(sourcePath, destinationPath, fsConstants.COPYFILE_EXCL);
      return destinationPath;
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'EEXIST') {
        throw error;
      }
    }
  }

  throw new Error('Unable to create a unique destination filename for captured photo.');
}

function assertDestinationInsideStation(stationInputPath: string, destinationPath: string): void {
  const relativePath = path.relative(path.resolve(stationInputPath), path.resolve(destinationPath));
  if (relativePath.startsWith('..') || path.isAbsolute(relativePath)) {
    throw new Error('Refusing to copy outside the station input folder.');
  }
}

function sanitizeError(error: string): string {
  return error.replace(/[A-Z]:\\[^\s"']+/gi, '[local-path]').slice(0, 500);
}

function toStringOrNull(value: unknown): string | null {
  if (typeof value !== 'string' && typeof value !== 'number') {
    return null;
  }
  const trimmed = String(value).trim();
  return trimmed.length > 0 ? trimmed : null;
}
