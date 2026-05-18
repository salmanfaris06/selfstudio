import { constants as fsConstants } from 'node:fs';
import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import type { StationConfig } from './stations.js';

export type CameraImportResult = {
  success: true;
  stationId: StationConfig['id'];
  sourcePath: string;
  destinationPath: string;
  filename: string;
  fileSize: number;
  modifiedTime: string;
};

export type CameraImportError = {
  success: false;
  error: string;
};

type CameraVolume = {
  driveLetter: string;
  label: string | null;
  model: string | null;
  pnpDeviceId: string | null;
};

type RawCameraVolume = {
  driveLetter?: string | null;
  label?: string | null;
  model?: string | null;
  pnpDeviceId?: string | null;
};

type JpgCandidate = {
  filePath: string;
  fileSize: number;
  modifiedTimeMs: number;
  modifiedTime: string;
};

type ScanResult = {
  candidates: JpgCandidate[];
  dcimFound: boolean;
};

const MIN_SOURCE_AGE_MS = 2_000;
const MAX_SCAN_DEPTH = 5;
const MAX_SCAN_FILES = 20_000;

const POWERSHELL_VOLUME_SCRIPT = String.raw`
$ErrorActionPreference = 'Stop'
$items = @()
$diskDrives = Get-CimInstance Win32_DiskDrive | Where-Object {
  (($_.Model, $_.PNPDeviceID, $_.Caption) -join ' ') -match 'Sony|DSC|Camera'
}
foreach ($disk in $diskDrives) {
  $partitions = @(Get-CimAssociatedInstance -InputObject $disk -ResultClassName Win32_DiskPartition -ErrorAction SilentlyContinue)
  foreach ($partition in $partitions) {
    $logicalDisks = @(Get-CimAssociatedInstance -InputObject $partition -ResultClassName Win32_LogicalDisk -ErrorAction SilentlyContinue)
    foreach ($logicalDisk in $logicalDisks) {
      $items += [pscustomobject]@{
        driveLetter = $logicalDisk.DeviceID
        label = $logicalDisk.VolumeName
        model = $disk.Model
        pnpDeviceId = $disk.PNPDeviceID
      }
    }
  }
}
$items | ConvertTo-Json -Depth 4 -Compress
`;

export async function importLatestPhotoFromCamera(
  stationId: string,
  stations: StationConfig[]
): Promise<CameraImportResult | CameraImportError> {
  try {
    const station = stations.find((candidate) => candidate.id === stationId.trim());
    if (!station) {
      return { success: false, error: `Invalid station id: ${stationId}` };
    }

    if (process.platform !== 'win32') {
      return { success: false, error: 'Camera storage import currently supports Windows only.' };
    }

    await assertStationInputFolderReady(station.inputPath);

    const volumes = await findCameraVolumes();
    if (volumes.length === 0) {
      return {
        success: false,
        error: 'Camera storage not found. Set camera USB Connection to Mass Storage, reconnect USB, and confirm it appears as a drive in Windows.'
      };
    }

    const latestPhoto = await findLatestJpg(volumes);
    if (!latestPhoto.dcimFound) {
      return {
        success: false,
        error: `DCIM folder not found on detected camera storage (${volumes.map((volume) => volume.driveLetter).join(', ')}).`
      };
    }
    if (!latestPhoto.candidate) {
      return {
        success: false,
        error: `No stable JPG/JPEG found under DCIM on detected camera storage (${volumes.map((volume) => volume.driveLetter).join(', ')}).`
      };
    }

    const result = await copyWithUniqueDestination(station, latestPhoto.candidate);
    return result;
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

async function assertStationInputFolderReady(inputPath: string): Promise<void> {
  const stat = await fs.stat(inputPath);
  if (!stat.isDirectory()) {
    throw new Error(`Station input path is not a directory: ${inputPath}`);
  }
}

async function findCameraVolumes(): Promise<CameraVolume[]> {
  const stdout = await runPowerShell(POWERSHELL_VOLUME_SCRIPT);
  const rawVolumes = parsePowerShellJson(stdout);
  return rawVolumes
    .map(normalizeVolume)
    .filter((volume): volume is CameraVolume => Boolean(volume));
}

function runPowerShell(script: string): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      'powershell.exe',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', script],
      { timeout: 10_000, windowsHide: true, maxBuffer: 1024 * 1024 },
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

function parsePowerShellJson(stdout: string): RawCameraVolume[] {
  if (!stdout) {
    return [];
  }
  const parsed = JSON.parse(stdout) as RawCameraVolume | RawCameraVolume[] | null;
  if (!parsed) {
    return [];
  }
  return Array.isArray(parsed) ? parsed : [parsed];
}

function normalizeVolume(raw: RawCameraVolume): CameraVolume | null {
  const driveLetter = toStringOrNull(raw.driveLetter);
  if (!driveLetter || !/^[A-Z]:$/i.test(driveLetter)) {
    return null;
  }
  return {
    driveLetter: driveLetter.toUpperCase(),
    label: toStringOrNull(raw.label),
    model: toStringOrNull(raw.model),
    pnpDeviceId: toStringOrNull(raw.pnpDeviceId)
  };
}

async function findLatestJpg(volumes: CameraVolume[]): Promise<{ candidate: JpgCandidate | null; dcimFound: boolean }> {
  const candidates: JpgCandidate[] = [];
  let dcimFound = false;

  for (const volume of volumes) {
    const rootPath = `${volume.driveLetter}${path.sep}`;
    const dcimPath = await findDcimFolder(rootPath);
    if (!dcimPath) {
      continue;
    }
    dcimFound = true;
    const scan = await scanJpgs(dcimPath, 0, { filesSeen: 0 });
    candidates.push(...scan.candidates);
  }

  candidates.sort((a, b) => b.modifiedTimeMs - a.modifiedTimeMs || b.filePath.localeCompare(a.filePath));
  return { candidate: candidates[0] ?? null, dcimFound };
}

async function findDcimFolder(rootPath: string): Promise<string | null> {
  const entries = await fs.readdir(rootPath, { withFileTypes: true });
  const dcim = entries.find((entry) => entry.isDirectory() && entry.name.toLowerCase() === 'dcim');
  return dcim ? path.join(rootPath, dcim.name) : null;
}

async function scanJpgs(
  directoryPath: string,
  depth: number,
  state: { filesSeen: number }
): Promise<ScanResult> {
  const candidates: JpgCandidate[] = [];
  if (depth > MAX_SCAN_DEPTH || state.filesSeen > MAX_SCAN_FILES) {
    return { candidates, dcimFound: true };
  }

  const entries = await fs.readdir(directoryPath, { withFileTypes: true });
  for (const entry of entries) {
    const entryPath = path.join(directoryPath, entry.name);
    if (entry.isDirectory()) {
      candidates.push(...(await scanJpgs(entryPath, depth + 1, state)).candidates);
    } else if (entry.isFile() && /\.(jpe?g)$/i.test(entry.name)) {
      state.filesSeen += 1;
      try {
        const stat = await fs.stat(entryPath);
        if (Date.now() - stat.mtimeMs < MIN_SOURCE_AGE_MS) {
          continue;
        }
        candidates.push({
          filePath: entryPath,
          fileSize: stat.size,
          modifiedTimeMs: stat.mtimeMs,
          modifiedTime: stat.mtime.toISOString()
        });
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== 'ENOENT') {
          throw error;
        }
      }
    }
  }

  return { candidates, dcimFound: true };
}

async function copyWithUniqueDestination(
  station: StationConfig,
  latestPhoto: JpgCandidate
): Promise<CameraImportResult> {
  const parsed = path.parse(latestPhoto.filePath);
  const safeBase = parsed.name.replace(/[^a-z0-9_-]+/gi, '-').replace(/^-+|-+$/g, '') || 'photo';
  const ext = parsed.ext.toLowerCase() === '.jpeg' ? '.jpeg' : '.jpg';
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  const baseName = `imported-${timestamp}-${safeBase}`;

  for (let index = 0; index < 1000; index += 1) {
    const suffix = index === 0 ? '' : `-${index}`;
    const destinationPath = path.join(station.inputPath, `${baseName}${suffix}${ext}`);
    assertDestinationInsideStation(station.inputPath, destinationPath);
    try {
      await fs.copyFile(latestPhoto.filePath, destinationPath, fsConstants.COPYFILE_EXCL);
      const destinationStat = await fs.stat(destinationPath);
      return {
        success: true,
        stationId: station.id,
        sourcePath: latestPhoto.filePath,
        destinationPath,
        filename: path.basename(destinationPath),
        fileSize: destinationStat.size,
        modifiedTime: destinationStat.mtime.toISOString()
      };
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'EEXIST') {
        throw error;
      }
    }
  }

  throw new Error('Unable to create a unique destination filename for imported photo.');
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
  if (typeof value !== 'string') {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}
