import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import type { StationConfig } from './stations.js';

export type GPhotoBackend = 'native' | 'wsl';

export type CommandProbe = {
  backend: GPhotoBackend;
  available: boolean;
  command: string;
  stdout: string;
  stderr: string;
  error: string | null;
};

export type GPhotoDiagnostics = {
  platform: NodeJS.Platform;
  status: 'ok' | 'unavailable' | 'error';
  scannedAt: string;
  selectedBackend: GPhotoBackend | null;
  probes: CommandProbe[];
  autoDetect: CommandProbe | null;
  summary: string;
  nextActions: string[];
};

export type GPhotoTriggerDownloadResult =
  | {
      success: true;
      stationId: string;
      backend: GPhotoBackend;
      downloadedCount: number;
      files: Array<{ outputPath: string; filename: string }>;
      stdout: string;
      stderr: string;
    }
  | {
      success: false;
      stationId?: string;
      backend?: GPhotoBackend;
      noEvent?: boolean;
      error: string;
      stdout?: string;
      stderr?: string;
      nextActions: string[];
    };

export type GPhotoCaptureResult =
  | {
      success: true;
      stationId: string;
      backend: GPhotoBackend;
      outputPath: string;
      filename: string;
      stdout: string;
      stderr: string;
    }
  | {
      success: false;
      stationId?: string;
      backend?: GPhotoBackend;
      error: string;
      stdout?: string;
      stderr?: string;
      nextActions: string[];
    };

const COMMAND_TIMEOUT_MS = 15_000;
const CAPTURE_TIMEOUT_MS = 60_000;
const CAMERA_RESET_TIMEOUT_MS = 15_000;

export async function getGPhotoDiagnostics(): Promise<GPhotoDiagnostics> {
  const probes = await Promise.all([probeNativeGPhoto(), probeWslGPhoto()]);
  const selected = selectBackend(probes);
  const autoDetect = selected ? await runAutoDetect(selected.backend) : null;

  return {
    platform: process.platform,
    status: selected ? 'ok' : 'unavailable',
    scannedAt: new Date().toISOString(),
    selectedBackend: selected?.backend ?? null,
    probes,
    autoDetect,
    summary: selected
      ? `gphoto2 helper is available through ${selected.backend}.`
      : 'gphoto2 helper is not available on native Windows PATH or WSL PATH.',
    nextActions: buildDiagnosticNextActions(selected?.backend ?? null, autoDetect)
  };
}

export async function downloadCameraTriggeredPhotosWithGPhoto(
  stationId: string,
  stations: StationConfig[],
  waitSeconds = 10
): Promise<GPhotoTriggerDownloadResult> {
  const station = stations.find((candidate) => candidate.id === stationId.trim());
  if (!station) {
    return {
      success: false,
      stationId,
      error: `Invalid station id: ${stationId}`,
      nextActions: ['Choose one configured station id: camera-1, camera-2, camera-3.']
    };
  }

  const diagnostics = await getGPhotoDiagnostics();
  if (diagnostics.selectedBackend !== 'wsl') {
    return {
      success: false,
      stationId: station.id,
      backend: diagnostics.selectedBackend ?? undefined,
      error: 'Camera-trigger listener currently requires the WSL gphoto2 backend.',
      nextActions: diagnostics.nextActions
    };
  }

  const stationInput = path.resolve(station.inputPath);
  await fs.mkdir(stationInput, { recursive: true });

  const batchId = `camera-trigger-${new Date().toISOString().replace(/[:.]/g, '-')}-${randomUUID()}`;
  const wslFolder = windowsPathToWslPath(stationInput);
  if (!wslFolder) {
    return {
      success: false,
      stationId: station.id,
      backend: 'wsl',
      error: `Failed to convert station input path to WSL path: ${stationInput}`,
      nextActions: ['Move the project to a normal mounted drive path like D:\\_Project\\selfstudio.']
    };
  }

  const autoDetect = await runAutoDetect('wsl');
  const detectedPort = parseGPhotoPort(autoDetect.stdout);
  if (!detectedPort) {
    return {
      success: false,
      stationId: station.id,
      backend: 'wsl',
      error: 'gphoto2 is installed in WSL, but no camera is detected for trigger listener.',
      stdout: autoDetect.stdout,
      stderr: autoDetect.stderr,
      nextActions: ['Run One-click Setup, keep USB Connection = PC Remote, then start listener again.']
    };
  }

  const pattern = `${wslFolder}/${batchId}-%n.%C`;
  const safeWaitSeconds = Math.max(2, Math.min(60, Math.floor(waitSeconds)));
  const result = await exec(
    'wsl.exe',
    wslArgs(`mkdir -p ${shellQuote(wslFolder)} && gphoto2 --port ${shellQuote(detectedPort)} --wait-event-and-download=${safeWaitSeconds}s --force-overwrite --filename ${shellQuote(pattern)}`),
    (safeWaitSeconds + 20) * 1000
  );

  if (result.exitCode !== 0) {
    return {
      success: false,
      stationId: station.id,
      backend: 'wsl',
      error: result.error ?? 'gphoto2 wait-event-and-download failed.',
      stdout: result.stdout,
      stderr: result.stderr,
      nextActions: buildCaptureFailureActions('wsl')
    };
  }

  const files = await findDownloadedJpgs(stationInput, batchId);
  if (files.length === 0) {
    return {
      success: false,
      stationId: station.id,
      backend: 'wsl',
      noEvent: true,
      error: 'No new JPG event downloaded during wait window.',
      stdout: result.stdout,
      stderr: result.stderr,
      nextActions: ['Press the physical shutter while trigger listener is running.']
    };
  }

  return { success: true, stationId: station.id, backend: 'wsl', downloadedCount: files.length, files, stdout: result.stdout, stderr: result.stderr };
}

export async function captureWithGPhoto(
  stationId: string,
  stations: StationConfig[]
): Promise<GPhotoCaptureResult> {
  const station = stations.find((candidate) => candidate.id === stationId.trim());
  if (!station) {
    return {
      success: false,
      stationId,
      error: `Invalid station id: ${stationId}`,
      nextActions: ['Choose one configured station id: camera-1, camera-2, camera-3.']
    };
  }

  const diagnostics = await getGPhotoDiagnostics();
  if (!diagnostics.selectedBackend) {
    return {
      success: false,
      stationId: station.id,
      error: diagnostics.summary,
      nextActions: diagnostics.nextActions
    };
  }

  const stationInput = path.resolve(station.inputPath);
  const batchId = `gphoto-capture-${new Date().toISOString().replace(/[:.]/g, '-')}-${randomUUID()}`;
  const filename = `${batchId}.jpg`;
  const outputPath = path.resolve(stationInput, filename);
  if (!outputPath.startsWith(`${stationInput}${path.sep}`)) {
    return {
      success: false,
      stationId: station.id,
      error: 'Resolved capture output escaped station input folder.',
      nextActions: ['Check station input path configuration.']
    };
  }

  await fs.mkdir(stationInput, { recursive: true });

  if (diagnostics.selectedBackend === 'native') {
    return captureNative(station.id, outputPath, filename);
  }

  return captureWsl(station.id, stationInput, outputPath, batchId);
}

async function probeNativeGPhoto(): Promise<CommandProbe> {
  const command = process.env.GPHOTO2_BIN || 'gphoto2';
  const result = await exec(command, ['--version'], COMMAND_TIMEOUT_MS);
  return {
    backend: 'native',
    available: result.exitCode === 0,
    command: `${command} --version`,
    stdout: result.stdout,
    stderr: result.stderr,
    error: result.exitCode === 0 ? null : result.error
  };
}

async function probeWslGPhoto(): Promise<CommandProbe> {
  const result = await exec('wsl.exe', wslArgs('command -v gphoto2 && gphoto2 --version'), COMMAND_TIMEOUT_MS);
  return {
    backend: 'wsl',
    available: result.exitCode === 0,
    command: 'wsl.exe bash -lc "command -v gphoto2 && gphoto2 --version"',
    stdout: result.stdout,
    stderr: result.stderr,
    error: result.exitCode === 0 ? null : result.error
  };
}

async function runAutoDetect(backend: GPhotoBackend): Promise<CommandProbe> {
  if (backend === 'native') {
    const command = process.env.GPHOTO2_BIN || 'gphoto2';
    const result = await exec(command, ['--auto-detect'], COMMAND_TIMEOUT_MS);
    return {
      backend,
      available: result.exitCode === 0,
      command: `${command} --auto-detect`,
      stdout: result.stdout,
      stderr: result.stderr,
      error: result.exitCode === 0 ? null : result.error
    };
  }

  const result = await exec('wsl.exe', wslArgs('gphoto2 --auto-detect'), COMMAND_TIMEOUT_MS);
  return {
    backend,
    available: result.exitCode === 0,
    command: 'wsl.exe bash -lc "gphoto2 --auto-detect"',
    stdout: result.stdout,
    stderr: result.stderr,
    error: result.exitCode === 0 ? null : result.error
  };
}

async function captureNative(
  stationId: string,
  outputPath: string,
  filename: string
): Promise<GPhotoCaptureResult> {
  const command = process.env.GPHOTO2_BIN || 'gphoto2';
  const result = await exec(
    command,
    ['--capture-image-and-download', '--force-overwrite', '--filename', outputPath],
    CAPTURE_TIMEOUT_MS
  );

  if (result.exitCode !== 0) {
    return {
      success: false,
      stationId,
      backend: 'native',
      error: result.error ?? 'gphoto2 native capture failed.',
      stdout: result.stdout,
      stderr: result.stderr,
      nextActions: buildCaptureFailureActions('native')
    };
  }

  const ready = await assertNonEmptyJpg(outputPath);
  if (!ready.success) {
    return {
      success: false,
      stationId,
      backend: 'native',
      error: ready.error,
      stdout: result.stdout,
      stderr: result.stderr,
      nextActions: ['Check whether gphoto2 downloaded the image to a different path.', 'Check camera USB mode = PC Remote.']
    };
  }

  return { success: true, stationId, backend: 'native', outputPath, filename, stdout: result.stdout, stderr: result.stderr };
}

async function captureWsl(
  stationId: string,
  stationInput: string,
  outputPath: string,
  batchId: string
): Promise<GPhotoCaptureResult> {
  const wslOutputPath = windowsPathToWslPath(outputPath);
  if (!wslOutputPath) {
    return {
      success: false,
      stationId,
      backend: 'wsl',
      error: `Failed to convert Windows output path to WSL path: ${outputPath}`,
      nextActions: ['Move the project to a normal drive path like D:\\_Project\\selfstudio or configure a custom WSL output path.']
    };
  }

  await resetWslCamera().catch(() => undefined);
  await sleep(1000);

  const autoDetect = await runAutoDetect('wsl');
  const detectedPort = parseGPhotoPort(autoDetect.stdout);
  if (!detectedPort) {
    return {
      success: false,
      stationId,
      backend: 'wsl',
      error: 'gphoto2 is installed in WSL, but no camera is currently detected immediately before capture.',
      stdout: autoDetect.stdout,
      stderr: autoDetect.stderr,
      nextActions: [
        'Run inside the same/default WSL distro: gphoto2 --auto-detect',
        'If PowerShell uses a different distro, set WSL_DISTRO=<distro name> before npm run dev.',
        'Re-attach Sony USB to WSL with usbipd attach --wsl --busid <BUSID>.',
        'Keep camera USB Connection = PC Remote.'
      ]
    };
  }

  const wslOutputDir = wslOutputPath.slice(0, wslOutputPath.lastIndexOf('/'));
  const quotedOutput = shellQuote(wslOutputPath);
  const quotedPort = shellQuote(detectedPort);
  const result = await exec(
    'wsl.exe',
    wslArgs(`mkdir -p ${shellQuote(wslOutputDir)} && gphoto2 --port ${quotedPort} --capture-image-and-download --force-overwrite --filename ${quotedOutput} && test -f ${quotedOutput}`),
    CAPTURE_TIMEOUT_MS
  );

  if (result.exitCode !== 0 && isRecoverableGPhotoError(`${result.stdout}\n${result.stderr}\n${result.error ?? ''}`)) {
    await resetWslCamera().catch(() => undefined);
    await sleep(2000);
    const retry = await exec(
      'wsl.exe',
      wslArgs(`gphoto2 --port ${quotedPort} --capture-image-and-download --force-overwrite --filename ${quotedOutput} && test -f ${quotedOutput}`),
      CAPTURE_TIMEOUT_MS
    );
    result.stdout = `${result.stdout}\n--- retry after reset ---\n${retry.stdout}`;
    result.stderr = `${result.stderr}\n${retry.stderr}`;
    result.error = retry.exitCode === 0 ? null : retry.error ?? result.error;
    result.exitCode = retry.exitCode;
  }

  if (result.exitCode !== 0) {
    return {
      success: false,
      stationId,
      backend: 'wsl',
      error: result.error ?? 'gphoto2 WSL capture failed.',
      stdout: result.stdout,
      stderr: result.stderr,
      nextActions: [...buildCaptureFailureActions('wsl'), `Detected port before capture: ${detectedPort}`]
    };
  }

  const ready = await assertNonEmptyJpg(outputPath);
  if (!ready.success) {
    return {
      success: false,
      stationId,
      backend: 'wsl',
      error: ready.error,
      stdout: result.stdout,
      stderr: result.stderr,
      nextActions: [
        'gphoto2 reported success but the downloaded file did not pass validation.',
        `Downloaded file: ${outputPath}`,
        `Expected WSL output path: ${wslOutputPath}`,
        'Check stdout/stderr to see what gphoto2 downloaded.'
      ]
    };
  }

  return { success: true, stationId, backend: 'wsl', outputPath, filename: path.basename(outputPath), stdout: result.stdout, stderr: result.stderr };
}

async function resetWslCamera(): Promise<void> {
  await exec('wsl.exe', wslArgs('gphoto2 --reset || true'), CAMERA_RESET_TIMEOUT_MS);
}

function isRecoverableGPhotoError(output: string): boolean {
  return /busy|timeout|timed out|No camera found|Could not claim|PTP|I\/O|USB/i.test(output);
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

function selectBackend(probes: CommandProbe[]): CommandProbe | null {
  const preferred = process.env.GPHOTO2_BACKEND as GPhotoBackend | undefined;
  if (preferred) {
    return probes.find((probe) => probe.backend === preferred && probe.available) ?? null;
  }
  return probes.find((probe) => probe.backend === 'native' && probe.available) ??
    probes.find((probe) => probe.backend === 'wsl' && probe.available) ??
    null;
}

async function findDownloadedJpgs(
  folderPath: string,
  batchId: string
): Promise<Array<{ outputPath: string; filename: string; mtimeMs: number }>> {
  const entries = await fs.readdir(folderPath, { withFileTypes: true });
  const candidates = await Promise.all(
    entries
      .filter((entry) => entry.isFile() && entry.name.startsWith(batchId) && /\.jpe?g$/i.test(entry.name))
      .map(async (entry) => {
        const outputPath = path.join(folderPath, entry.name);
        const stat = await fs.stat(outputPath);
        return { outputPath, filename: entry.name, mtimeMs: stat.mtimeMs };
      })
  );

  candidates.sort((a, b) => b.mtimeMs - a.mtimeMs);
  return candidates;
}

async function findLatestDownloadedJpg(
  folderPath: string,
  batchId: string
): Promise<{ path: string; filename: string } | null> {
  const [latest] = await findDownloadedJpgs(folderPath, batchId);
  return latest ? { path: latest.outputPath, filename: latest.filename } : null;
}

function parseGPhotoPort(stdout: string): string | null {
  const lines = stdout.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  for (const line of lines) {
    if (/^Model\s+Port/i.test(line) || /^-+$/.test(line)) {
      continue;
    }
    const match = line.match(/(usb:\d+,\d+|ptpip:[^\s]+)$/i);
    if (match) {
      return match[1];
    }
  }
  return null;
}

function wslArgs(command: string): string[] {
  const distro = process.env.WSL_DISTRO?.trim();
  return distro ? ['-d', distro, 'bash', '-lc', command] : ['bash', '-lc', command];
}

function buildDiagnosticNextActions(
  backend: GPhotoBackend | null,
  autoDetect: CommandProbe | null
): string[] {
  if (!backend) {
    return [
      'Install/prepare a gphoto2 helper only if direct dashboard shutter remains required.',
      'Recommended Windows path: WSL2 + usbipd-win + gphoto2, then attach Sony USB device to WSL.',
      'Current no-extra-helper workflow remains Mass Storage + Import Latest.'
    ];
  }

  const actions = [`gphoto2 backend selected: ${backend}.`];
  if (autoDetect && !/Sony|ILCE|usb:/i.test(autoDetect.stdout)) {
    actions.push('gphoto2 is installed but did not auto-detect Sony camera. If using WSL, attach USB device to WSL with usbipd-win.');
  } else {
    actions.push('If Sony camera appears in auto-detect, try /api/gphoto/capture with stationId camera-1.');
  }
  return actions;
}

function buildCaptureFailureActions(backend: GPhotoBackend): string[] {
  if (backend === 'wsl') {
    return [
      'Confirm Sony camera is attached to WSL, not only Windows WPD.',
      'Run inside WSL: gphoto2 --auto-detect',
      'If timeout occurs, reconnect camera in PC Remote mode and retry.'
    ];
  }
  return [
    'Run: gphoto2 --auto-detect',
    'Confirm camera USB mode = PC Remote.',
    'If native Windows gphoto2 cannot access the camera, try WSL2 + usbipd-win.'
  ];
}

async function assertNonEmptyJpg(filePath: string): Promise<{ success: true } | { success: false; error: string }> {
  try {
    const file = await fs.open(filePath, 'r');
    try {
      const stat = await file.stat();
      if (stat.size < 4) {
        return { success: false, error: 'Captured file is empty or too small.' };
      }
      const header = Buffer.alloc(16);
      await file.read(header, 0, 16, 0);
      if (header[0] !== 0xff || header[1] !== 0xd8) {
        const extension = path.extname(filePath).toLowerCase();
        if (extension === '.jpg' || extension === '.jpeg') {
          return { success: true };
        }
        return {
          success: false,
          error: `Captured file is not a JPEG. Size=${stat.size} bytes, first16=${header.toString('hex')}`
        };
      }
      return { success: true };
    } finally {
      await file.close();
    }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : String(error) };
  }
}

function windowsPathToWslPath(value: string): string | null {
  const normalized = path.resolve(value);
  const match = normalized.match(/^([A-Za-z]):\\(.*)$/);
  if (!match) {
    return null;
  }
  const drive = match[1].toLowerCase();
  const rest = match[2].replace(/\\/g, '/');
  return `/mnt/${drive}/${rest}`;
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`;
}

function exec(
  command: string,
  args: string[],
  timeout: number
): Promise<{ exitCode: number; stdout: string; stderr: string; error: string | null }> {
  return new Promise((resolve) => {
    execFile(command, args, { timeout, windowsHide: true, maxBuffer: 2 * 1024 * 1024 }, (error, stdout, stderr) => {
      resolve({
        exitCode: error && typeof error === 'object' && 'code' in error && typeof error.code === 'number' ? error.code : error ? 1 : 0,
        stdout: sanitizeOutput(stdout),
        stderr: sanitizeOutput(stderr),
        error: error ? sanitizeOutput(error.message) : null
      });
    });
  });
}

function sanitizeOutput(value: string): string {
  return value.replace(/[A-Z]:\\[^\s"']+/gi, '[local-path]').slice(0, 4000);
}
