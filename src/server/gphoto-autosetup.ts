import { execFile } from 'node:child_process';
import { detectUsbCameras } from './usb-camera-detector.js';
import { getGPhotoDiagnostics } from './gphoto-helper.js';

export type GPhotoSetupResult = {
  platform: NodeJS.Platform;
  status: 'ready' | 'needs-admin' | 'needs-wsl' | 'needs-gphoto' | 'needs-camera' | 'attach-failed' | 'error';
  scannedAt: string;
  busId: string | null;
  distro: string | null;
  diagnostics: Awaited<ReturnType<typeof getGPhotoDiagnostics>> | null;
  steps: Array<{
    name: string;
    ok: boolean;
    command?: string;
    stdout?: string;
    stderr?: string;
    error?: string | null;
  }>;
  summary: string;
  nextActions: string[];
};

type CommandResult = {
  exitCode: number;
  stdout: string;
  stderr: string;
  error: string | null;
};

const SETUP_TIMEOUT_MS = 20_000;

export async function autoSetupGPhoto(): Promise<GPhotoSetupResult> {
  const steps: GPhotoSetupResult['steps'] = [];

  if (process.platform !== 'win32') {
    return buildResult('error', null, null, null, steps, 'One-click setup currently supports Windows host only.', []);
  }

  const distro = await getDefaultWslDistro(steps);
  if (!distro) {
    return buildResult('needs-wsl', null, null, null, steps, 'No WSL2 distro is available.', [
      'Install WSL2 + Ubuntu first: wsl --install -d Ubuntu',
      'Then run One-click gPhoto Setup again.'
    ]);
  }

  const gphotoVersion = await exec('wsl.exe', ['-d', distro, 'bash', '-lc', 'command -v gphoto2 && gphoto2 --version'], SETUP_TIMEOUT_MS);
  steps.push({ name: 'Check gphoto2 in WSL', ok: gphotoVersion.exitCode === 0, command: `wsl.exe -d ${distro} bash -lc "command -v gphoto2 && gphoto2 --version"`, ...gphotoVersion });
  if (gphotoVersion.exitCode !== 0) {
    return buildResult('needs-gphoto', null, distro, null, steps, 'WSL is available, but gphoto2 is not installed in the selected distro.', [
      `Run: wsl.exe -d ${distro} bash -lc "sudo apt update && sudo apt install -y gphoto2"`,
      'Then run One-click gPhoto Setup again.'
    ]);
  }

  const preAttachDetect = await exec('wsl.exe', ['-d', distro, 'bash', '-lc', 'gphoto2 --auto-detect'], SETUP_TIMEOUT_MS);
  const alreadyReady = preAttachDetect.exitCode === 0 && /Sony|ILCE|usb:/i.test(preAttachDetect.stdout);
  steps.push({ name: 'Check if Sony is already attached to WSL', ok: alreadyReady, command: `wsl.exe -d ${distro} bash -lc "gphoto2 --auto-detect"`, ...preAttachDetect });
  if (alreadyReady) {
    const diagnostics = await getGPhotoDiagnostics();
    return buildResult('ready', null, distro, diagnostics, steps, 'Sony camera is already attached to WSL and gphoto2 is ready.', [
      'Use Setup + Capture Camera 1/2/3 in the dashboard.'
    ]);
  }

  const busId = await findSonyBusId(steps);
  if (!busId) {
    return buildResult('needs-camera', null, distro, null, steps, 'Sony ILCE-6000 USB device was not found in usbipd list / Windows USB detection.', [
      'Set camera USB Connection = PC Remote.',
      'Reconnect USB cable and run One-click gPhoto Setup again.'
    ]);
  }

  const bind = await exec('usbipd.exe', ['bind', '--busid', busId], SETUP_TIMEOUT_MS);
  const bindOk = bind.exitCode === 0 || /already shared/i.test(`${bind.stdout} ${bind.stderr} ${bind.error ?? ''}`);
  steps.push({ name: 'Share Sony USB with usbipd', ok: bindOk, command: `usbipd bind --busid ${busId}`, ...bind });
  if (!bindOk) {
    return buildResult('needs-admin', busId, distro, null, steps, 'usbipd bind failed. This usually requires PowerShell/Admin rights.', [
      `Open PowerShell as Administrator and run: usbipd bind --busid ${busId}`,
      'Then run One-click gPhoto Setup again.'
    ]);
  }

  const persisted = await exec('usbipd.exe', ['bind', '--busid', busId, '--auto-attach', '--wsl', distro], SETUP_TIMEOUT_MS);
  const persistedOk = persisted.exitCode === 0 || /already shared|already bound|already exists|persisted/i.test(`${persisted.stdout} ${persisted.stderr} ${persisted.error ?? ''}`);
  steps.push({ name: 'Persist Sony auto-attach for future reconnects/reboots', ok: persistedOk, command: `usbipd bind --busid ${busId} --auto-attach --wsl ${distro}`, ...persisted });

  const attach = await exec('usbipd.exe', ['attach', '--busid', busId, '--wsl', distro], SETUP_TIMEOUT_MS);
  const attachOk = attach.exitCode === 0 || /already attached|is attached/i.test(`${attach.stdout} ${attach.stderr} ${attach.error ?? ''}`);
  steps.push({ name: 'Attach Sony USB to WSL', ok: attachOk, command: `usbipd attach --busid ${busId} --wsl ${distro}`, ...attach });
  if (!attachOk) {
    return buildResult('attach-failed', busId, distro, null, steps, 'usbipd attach failed.', [
      `Keep a WSL terminal open: wsl -d ${distro}`,
      `Then run as Admin: usbipd attach --busid ${busId} --wsl ${distro}`,
      'After that, run One-click gPhoto Setup again.'
    ]);
  }

  const lsusb = await exec('wsl.exe', ['-d', distro, 'bash', '-lc', 'lsusb | grep -i "054c:094e\|sony" || true'], SETUP_TIMEOUT_MS);
  steps.push({ name: 'Verify Sony visible in WSL USB list', ok: /054c:094e|sony/i.test(lsusb.stdout), command: `wsl.exe -d ${distro} bash -lc "lsusb | grep -i sony"`, ...lsusb });

  const diagnostics = await getGPhotoDiagnostics();
  const ready = diagnostics.selectedBackend === 'wsl' && /Sony|ILCE|usb:/i.test(diagnostics.autoDetect?.stdout ?? '');
  return buildResult(
    ready ? 'ready' : 'attach-failed',
    busId,
    distro,
    diagnostics,
    steps,
    ready ? 'gphoto2 WSL backend is ready for one-click dashboard capture.' : 'Setup ran, but gphoto2 still does not auto-detect the Sony camera.',
    ready
      ? ['Use gPhoto Capture Camera 1/2/3 in the dashboard.']
      : ['Check WSL: gphoto2 --auto-detect', 'Reconnect camera in PC Remote mode, then rerun setup.']
  );
}

async function getDefaultWslDistro(steps: GPhotoSetupResult['steps']): Promise<string | null> {
  const result = await exec('wsl.exe', ['-l', '-v'], SETUP_TIMEOUT_MS);
  steps.push({ name: 'List WSL distros', ok: result.exitCode === 0, command: 'wsl -l -v', ...result });
  if (result.exitCode !== 0) {
    return null;
  }

  const lines = result.stdout.replace(/\u0000/g, '').split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  for (const line of lines) {
    if (/^NAME\s+STATE\s+VERSION/i.test(line)) {
      continue;
    }
    const cleaned = line.replace(/^\*\s*/, '').trim();
    const match = cleaned.match(/^(.+?)\s+(Running|Stopped)\s+2$/i);
    if (match) {
      return match[1].trim();
    }
  }
  return null;
}

async function findSonyBusId(steps: GPhotoSetupResult['steps']): Promise<string | null> {
  const list = await exec('usbipd.exe', ['list'], SETUP_TIMEOUT_MS);
  steps.push({ name: 'List usbipd devices', ok: list.exitCode === 0, command: 'usbipd list', ...list });
  const match = list.stdout.match(/^(\S+)\s+054c:094e\s+/im);
  if (match) {
    return match[1];
  }

  const usb = await detectUsbCameras({ force: true });
  steps.push({
    name: 'Fallback Windows USB detection',
    ok: usb.detectedCount > 0,
    stdout: JSON.stringify(usb.devices),
    stderr: '',
    error: usb.error ?? null
  });
  return null;
}

function buildResult(
  status: GPhotoSetupResult['status'],
  busId: string | null,
  distro: string | null,
  diagnostics: GPhotoSetupResult['diagnostics'],
  steps: GPhotoSetupResult['steps'],
  summary: string,
  nextActions: string[]
): GPhotoSetupResult {
  return {
    platform: process.platform,
    status,
    scannedAt: new Date().toISOString(),
    busId,
    distro,
    diagnostics,
    steps,
    summary,
    nextActions
  };
}

function exec(command: string, args: string[], timeout: number): Promise<CommandResult> {
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
