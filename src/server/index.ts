import express from 'express';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { stations } from './stations.js';
import { readIngestEvents } from './ingest-log.js';
import { getStationStates, hydrateStationStateFromEvents, initializeStationState } from './state.js';
import { startStationWatchers } from './watcher.js';
import { detectUsbCameras } from './usb-camera-detector.js';
import { importLatestPhotoFromCamera } from './camera-storage.js';
import { capturePhotoToStation, getDirectCaptureCapabilities } from './direct-capture.js';
import { autoSetupGPhoto } from './gphoto-autosetup.js';
import { captureWithGPhoto, downloadCameraTriggeredPhotosWithGPhoto, getGPhotoDiagnostics } from './gphoto-helper.js';
import { getSonyPtpCapabilities, requestSonyPtpCapture } from './sony-ptp-diagnostic.js';
import { getSonyNativeFeasibility } from './sony-native-feasibility.js';
import { probeWpdManagerDeep } from './wpd-deep-probe.js';
import { probeWindowsPortableDevices } from './wpd-probe.js';
import { logDebugEvent, readDebugEvents, serializeError } from './debug-log.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const clientDir = path.resolve(__dirname, '..', 'client');
const port = parsePort(process.env.PORT ?? '3000');

initializeStationState(stations);
hydrateStationStateFromEvents(await readIngestEvents(500));
await startStationWatchers(stations);

const app = express();

app.use(express.json({ limit: '16kb' }));

app.get('/api/stations', (_request, response) => {
  response.json({ stations: getStationStates() });
});

app.get('/api/events', async (request, response, next) => {
  try {
    const rawLimit = Array.isArray(request.query.limit) ? request.query.limit[0] : request.query.limit;
    const limit = Number(rawLimit ?? 100);
    response.json({ events: await readIngestEvents(Number.isFinite(limit) ? limit : 100) });
  } catch (error) {
    next(error);
  }
});

app.get('/api/usb-cameras', async (request, response, next) => {
  try {
    const result = await detectUsbCameras({ force: request.query.force === '1' });
    await logDebugEvent({
      level: result.status === 'error' ? 'warn' : 'info',
      area: 'usb-detection',
      message: 'USB camera scan completed',
      data: { force: request.query.force === '1', detectedCount: result.detectedCount, status: result.status, devices: result.devices }
    });
    response.json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'usb-detection', message: 'USB camera scan failed', data: serializeError(error) });
    next(error);
  }
});

app.post('/api/camera-import/latest', async (request, response, next) => {
  try {
    const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId : '';
    const result = await importLatestPhotoFromCamera(stationId, stations);
    await logDebugEvent({
      level: result.success ? 'info' : 'warn',
      area: 'camera-import',
      message: result.success ? 'Latest photo import completed' : 'Latest photo import failed',
      data: { stationId, result }
    });
    response.status(result.success ? 200 : 400).json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'camera-import', message: 'Latest photo import crashed', data: serializeError(error) });
    next(error);
  }
});

app.get('/api/direct-capture/capabilities', async (request, response, next) => {
  try {
    const result = await getDirectCaptureCapabilities({ force: request.query.force === '1' });
    await logDebugEvent({
      level: result.status === 'error' ? 'warn' : 'info',
      area: 'direct-capture',
      message: 'WIA capture capability scan completed',
      data: { force: request.query.force === '1', status: result.status, captureCapableCount: result.captureCapableCount, devices: result.devices, error: result.error }
    });
    response.json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'direct-capture', message: 'WIA capture capability scan crashed', data: serializeError(error) });
    next(error);
  }
});

let captureInFlight = false;
app.post('/api/direct-capture/capture', async (request, response, next) => {
  const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId.trim() : '';
  if (!stations.some((station) => station.id === stationId)) {
    await logDebugEvent({ level: 'warn', area: 'direct-capture', message: 'Rejected direct capture for invalid station', data: { stationId } });
    response.status(400).json({ success: false, error: `Invalid station id: ${stationId}` });
    return;
  }

  if (captureInFlight) {
    await logDebugEvent({ level: 'warn', area: 'direct-capture', message: 'Rejected direct capture because another capture is in progress', data: { stationId } });
    response.status(409).json({ success: false, error: 'Direct capture already in progress' });
    return;
  }

  captureInFlight = true;
  try {
    const result = await capturePhotoToStation(stationId, stations);
    await logDebugEvent({
      level: result.success ? 'info' : 'warn',
      area: 'direct-capture',
      message: result.success ? 'Direct capture completed' : 'Direct capture failed',
      data: { stationId, result }
    });
    response.status(result.success ? 200 : 400).json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'direct-capture', message: 'Direct capture crashed', data: serializeError(error) });
    next(error);
  } finally {
    captureInFlight = false;
  }
});

app.get('/api/sony-ptp/capabilities', async (_request, response, next) => {
  try {
    const result = await getSonyPtpCapabilities();
    await logDebugEvent({
      level: result.status === 'error' ? 'warn' : 'info',
      area: 'sony-ptp',
      message: 'Sony/PTP diagnostic completed',
      data: result
    });
    response.json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'sony-ptp', message: 'Sony/PTP diagnostic crashed', data: serializeError(error) });
    next(error);
  }
});

app.post('/api/sony-ptp/capture', async (request, response) => {
  const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId : '';
  const result = requestSonyPtpCapture(stationId, stations);
  await logDebugEvent({
    level: 'warn',
    area: 'sony-ptp',
    message: 'Sony/PTP capture requested without native backend',
    data: result
  });
  response.status(501).json(result);
});

app.get('/api/sony-ptp/native-feasibility', async (_request, response, next) => {
  try {
    const result = await getSonyNativeFeasibility();
    await logDebugEvent({
      level: result.status === 'error' ? 'warn' : 'info',
      area: 'sony-native-feasibility',
      message: 'Sony native backend feasibility scan completed',
      data: result
    });
    response.json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'sony-native-feasibility', message: 'Sony native backend feasibility scan crashed', data: serializeError(error) });
    next(error);
  }
});

app.get('/api/sony-ptp/wpd-probe', async (_request, response, next) => {
  try {
    const result = await probeWindowsPortableDevices();
    await logDebugEvent({
      level: result.status === 'error' ? 'warn' : 'info',
      area: 'wpd-probe',
      message: 'Windows WPD probe completed',
      data: result
    });
    response.json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'wpd-probe', message: 'Windows WPD probe crashed', data: serializeError(error) });
    next(error);
  }
});

app.get('/api/sony-ptp/wpd-deep-probe', async (request, response, next) => {
  try {
    const deviceId = typeof request.query.deviceId === 'string' ? request.query.deviceId : null;
    const result = await probeWpdManagerDeep(deviceId);
    await logDebugEvent({
      level: result.status === 'ok' ? 'info' : 'warn',
      area: 'wpd-deep-probe',
      message: 'Windows WPD deep manager probe completed',
      data: result
    });
    response.json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'wpd-deep-probe', message: 'Windows WPD deep manager probe crashed', data: serializeError(error) });
    next(error);
  }
});

app.get('/api/gphoto/diagnostics', async (_request, response, next) => {
  try {
    const result = await getGPhotoDiagnostics();
    await logDebugEvent({
      level: result.status === 'ok' ? 'info' : 'warn',
      area: 'gphoto',
      message: 'gphoto2 diagnostics completed',
      data: result
    });
    response.json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'gphoto', message: 'gphoto2 diagnostics crashed', data: serializeError(error) });
    next(error);
  }
});

type TriggerListenerState = {
  stationId: string;
  active: boolean;
  waitSeconds: number;
  startedAt: string;
  downloadedCount: number;
  lastResult: unknown;
  lastError: string | null;
};

const triggerListeners = new Map<string, TriggerListenerState>();

let gphotoSetupInFlight = false;
app.post('/api/gphoto/setup', async (_request, response, next) => {
  if (gphotoSetupInFlight) {
    response.status(409).json({ status: 'error', summary: 'gPhoto setup already in progress' });
    return;
  }

  gphotoSetupInFlight = true;
  try {
    const result = await autoSetupGPhoto();
    await logDebugEvent({
      level: result.status === 'ready' ? 'info' : 'warn',
      area: 'gphoto-setup',
      message: 'gphoto2 one-click setup completed',
      data: result
    });
    response.status(result.status === 'ready' ? 200 : 400).json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'gphoto-setup', message: 'gphoto2 one-click setup crashed', data: serializeError(error) });
    next(error);
  } finally {
    gphotoSetupInFlight = false;
  }
});

type ContinuousCaptureState = {
  stationId: string;
  active: boolean;
  intervalMs: number;
  startedAt: string;
  captureCount: number;
  lastResult: unknown;
  lastError: string | null;
};

const continuousCaptures = new Map<string, ContinuousCaptureState>();
let gphotoCaptureInFlight = false;

app.get('/api/gphoto/trigger-listener/status', (_request, response) => {
  response.json({ listeners: Array.from(triggerListeners.values()) });
});

app.post('/api/gphoto/trigger-listener/start', async (request, response, next) => {
  const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId.trim() : '';
  const waitSecondsRaw = Number(request.body?.waitSeconds ?? 10);
  const waitSeconds = Number.isFinite(waitSecondsRaw) ? Math.max(2, Math.min(60, Math.floor(waitSecondsRaw))) : 10;

  if (!stations.some((station) => station.id === stationId)) {
    response.status(400).json({ success: false, error: `Invalid station id: ${stationId}` });
    return;
  }

  const existing = triggerListeners.get(stationId);
  if (existing?.active) {
    response.status(409).json({ success: false, error: `Camera-trigger listener already running for ${stationId}`, state: existing });
    return;
  }

  try {
    const setupResult = await autoSetupGPhoto();
    if (setupResult.status !== 'ready') {
      response.status(400).json({ success: false, stationId, error: setupResult.summary, setupStatus: setupResult.status, setup: setupResult, nextActions: setupResult.nextActions });
      return;
    }

    const state: TriggerListenerState = {
      stationId,
      active: true,
      waitSeconds,
      startedAt: new Date().toISOString(),
      downloadedCount: 0,
      lastResult: null,
      lastError: null
    };
    triggerListeners.set(stationId, state);
    void runTriggerListenerLoop(state);
    response.json({ success: true, state });
  } catch (error) {
    next(error);
  }
});

app.post('/api/gphoto/trigger-listener/stop', async (request, response) => {
  const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId.trim() : '';
  const state = triggerListeners.get(stationId);
  if (!state) {
    response.status(404).json({ success: false, error: `Camera-trigger listener is not running for ${stationId}` });
    return;
  }

  state.active = false;
  triggerListeners.delete(stationId);
  await logDebugEvent({ level: 'info', area: 'gphoto-trigger-listener', message: 'Camera-trigger listener stopped', data: state });
  response.json({ success: true, state });
});

async function runTriggerListenerLoop(state: TriggerListenerState): Promise<void> {
  await logDebugEvent({ level: 'info', area: 'gphoto-trigger-listener', message: 'Camera-trigger listener started', data: state });

  while (state.active) {
    if (gphotoCaptureInFlight) {
      await sleep(500);
      continue;
    }

    gphotoCaptureInFlight = true;
    try {
      const result = await downloadCameraTriggeredPhotosWithGPhoto(state.stationId, stations, state.waitSeconds);
      state.lastResult = result;
      if (result.success) {
        state.downloadedCount += result.downloadedCount;
        state.lastError = null;
      } else if (!result.noEvent) {
        state.lastError = result.error;
      }
      await logDebugEvent({ level: result.success ? 'info' : result.noEvent ? 'debug' : 'warn', area: 'gphoto-trigger-listener', message: result.success ? 'Camera-triggered download completed' : 'Camera-triggered wait finished without photo or failed', data: { state, result } });
    } catch (error) {
      state.lastError = error instanceof Error ? error.message : String(error);
      await logDebugEvent({ level: 'error', area: 'gphoto-trigger-listener', message: 'Camera-trigger listener crashed', data: serializeError(error) });
    } finally {
      gphotoCaptureInFlight = false;
    }
  }
}

app.get('/api/gphoto/continuous/status', (_request, response) => {
  response.json({ captures: Array.from(continuousCaptures.values()) });
});

app.post('/api/gphoto/continuous/start', async (request, response, next) => {
  const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId.trim() : '';
  const intervalMsRaw = Number(request.body?.intervalMs ?? 3000);
  const intervalMs = Number.isFinite(intervalMsRaw) ? Math.max(1500, Math.min(30000, intervalMsRaw)) : 3000;

  if (!stations.some((station) => station.id === stationId)) {
    response.status(400).json({ success: false, error: `Invalid station id: ${stationId}` });
    return;
  }

  const existing = continuousCaptures.get(stationId);
  if (existing?.active) {
    response.status(409).json({ success: false, error: `Continuous capture already running for ${stationId}`, state: existing });
    return;
  }

  try {
    const setupResult = await autoSetupGPhoto();
    await logDebugEvent({
      level: setupResult.status === 'ready' ? 'info' : 'warn',
      area: 'gphoto-setup',
      message: 'gphoto2 setup before continuous capture completed',
      data: setupResult
    });

    if (setupResult.status !== 'ready') {
      response.status(400).json({
        success: false,
        stationId,
        error: setupResult.summary,
        setupStatus: setupResult.status,
        setup: setupResult,
        nextActions: setupResult.nextActions
      });
      return;
    }

    const state: ContinuousCaptureState = {
      stationId,
      active: true,
      intervalMs,
      startedAt: new Date().toISOString(),
      captureCount: 0,
      lastResult: null,
      lastError: null
    };
    continuousCaptures.set(stationId, state);
    void runContinuousCaptureLoop(state);
    response.json({ success: true, state });
  } catch (error) {
    next(error);
  }
});

app.post('/api/gphoto/continuous/stop', async (request, response) => {
  const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId.trim() : '';
  const state = continuousCaptures.get(stationId);
  if (!state) {
    response.status(404).json({ success: false, error: `Continuous capture is not running for ${stationId}` });
    return;
  }

  state.active = false;
  continuousCaptures.delete(stationId);
  await logDebugEvent({
    level: 'info',
    area: 'gphoto-continuous',
    message: 'Continuous capture stopped',
    data: state
  });
  response.json({ success: true, state });
});

async function runContinuousCaptureLoop(state: ContinuousCaptureState): Promise<void> {
  await logDebugEvent({
    level: 'info',
    area: 'gphoto-continuous',
    message: 'Continuous capture started',
    data: state
  });

  while (state.active) {
    if (gphotoCaptureInFlight) {
      await sleep(500);
      continue;
    }

    gphotoCaptureInFlight = true;
    try {
      const result = await captureWithGPhoto(state.stationId, stations);
      state.lastResult = result;
      if (result.success) {
        state.captureCount += 1;
        state.lastError = null;
      } else {
        state.lastError = result.error;
      }
      await logDebugEvent({
        level: result.success ? 'info' : 'warn',
        area: 'gphoto-continuous',
        message: result.success ? 'Continuous gphoto2 capture completed' : 'Continuous gphoto2 capture failed',
        data: { state, result }
      });
    } catch (error) {
      state.lastError = error instanceof Error ? error.message : String(error);
      await logDebugEvent({
        level: 'error',
        area: 'gphoto-continuous',
        message: 'Continuous gphoto2 capture crashed',
        data: serializeError(error)
      });
    } finally {
      gphotoCaptureInFlight = false;
    }

    await sleep(state.intervalMs);
  }
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

app.post('/api/gphoto/capture', async (request, response, next) => {
  const stationId = typeof request.body?.stationId === 'string' ? request.body.stationId.trim() : '';

  if (gphotoCaptureInFlight) {
    response.status(409).json({ success: false, error: 'gphoto2 capture already in progress' });
    return;
  }

  gphotoCaptureInFlight = true;
  try {
    const setupResult = await autoSetupGPhoto();
    await logDebugEvent({
      level: setupResult.status === 'ready' ? 'info' : 'warn',
      area: 'gphoto-setup',
      message: 'gphoto2 setup before capture completed',
      data: setupResult
    });

    if (setupResult.status !== 'ready') {
      response.status(400).json({
        success: false,
        stationId,
        error: setupResult.summary,
        setupStatus: setupResult.status,
        setup: setupResult,
        nextActions: setupResult.nextActions
      });
      return;
    }

    const result = await captureWithGPhoto(stationId, stations);
    await logDebugEvent({
      level: result.success ? 'info' : 'warn',
      area: 'gphoto',
      message: result.success ? 'gphoto2 capture completed' : 'gphoto2 capture failed',
      data: result
    });
    response.status(result.success ? 200 : 400).json(result);
  } catch (error) {
    await logDebugEvent({ level: 'error', area: 'gphoto', message: 'gphoto2 capture crashed', data: serializeError(error) });
    next(error);
  } finally {
    gphotoCaptureInFlight = false;
  }
});

app.get('/api/debug/events', async (request, response, next) => {
  try {
    const rawLimit = Array.isArray(request.query.limit) ? request.query.limit[0] : request.query.limit;
    const limit = Number(rawLimit ?? 200);
    response.json({ events: await readDebugEvents(Number.isFinite(limit) ? limit : 200) });
  } catch (error) {
    next(error);
  }
});

app.use(express.static(clientDir));

app.get('*', (_request, response) => {
  response.sendFile(path.join(clientDir, 'index.html'));
});

app.listen(port, () => {
  console.log(`Selfstudio camera input spike running at http://localhost:${port}`);
  console.log('Watching:');
  for (const station of stations) {
    console.log(`- ${station.id}: ${station.inputPath}`);
  }
});

function parsePort(value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1 || parsed > 65_535) {
    throw new Error(`Invalid PORT value: ${value}`);
  }
  return parsed;
}
