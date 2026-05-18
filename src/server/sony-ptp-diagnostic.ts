import type { StationConfig } from './stations.js';
import { getDirectCaptureCapabilities } from './direct-capture.js';
import { detectUsbCameras } from './usb-camera-detector.js';

export type SonyPtpCapability = {
  platform: NodeJS.Platform;
  status: 'ok' | 'unsupported' | 'error';
  scannedAt: string;
  sonyUsbDetected: boolean;
  sonyWpdDetected: boolean;
  wiaCaptureAvailable: boolean;
  nativePtpBackendAvailable: boolean;
  summary: string;
  nextActions: string[];
  devices: Array<{
    name: string;
    className: string | null;
    manufacturer: string | null;
    status: string | null;
    source: string;
    matchedTerms: string[];
  }>;
  wiaDevices: Array<{
    name: string;
    type: string | null;
    canCapture: boolean;
  }>;
  error: string | null;
};

export type SonyPtpCaptureResult = {
  success: false;
  stationId?: string;
  error: string;
  nextActions: string[];
};

export async function getSonyPtpCapabilities(): Promise<SonyPtpCapability> {
  const [usb, wia] = await Promise.all([
    detectUsbCameras({ force: true }),
    getDirectCaptureCapabilities({ force: true })
  ]);

  const sonyDevices = usb.devices.filter((device) => {
    const haystack = [device.name, device.friendlyName, device.manufacturer, device.className, device.matchedTerms.join(' ')]
      .filter(Boolean)
      .join(' ')
      .toLowerCase();
    return haystack.includes('sony') || haystack.includes('ilce') || haystack.includes('a6000');
  });

  const sonyUsbDetected = sonyDevices.length > 0;
  const sonyWpdDetected = sonyDevices.some((device) => device.className?.toLowerCase() === 'wpd');
  const wiaCaptureAvailable = wia.captureCapableCount > 0;
  const nativePtpBackendAvailable = false;

  return {
    platform: process.platform,
    status: usb.status === 'error' && wia.status === 'error' ? 'error' : process.platform === 'win32' ? 'ok' : 'unsupported',
    scannedAt: new Date().toISOString(),
    sonyUsbDetected,
    sonyWpdDetected,
    wiaCaptureAvailable,
    nativePtpBackendAvailable,
    summary: buildSummary(sonyUsbDetected, sonyWpdDetected, wiaCaptureAvailable),
    nextActions: buildNextActions(sonyUsbDetected, sonyWpdDetected, wiaCaptureAvailable),
    devices: sonyDevices.map((device) => ({
      name: device.name,
      className: device.className,
      manufacturer: device.manufacturer,
      status: device.status,
      source: device.source,
      matchedTerms: device.matchedTerms
    })),
    wiaDevices: wia.devices.map((device) => ({
      name: device.name,
      type: device.type,
      canCapture: device.canCapture
    })),
    error: [usb.error, wia.error].filter(Boolean).join(' | ') || null
  };
}

export function requestSonyPtpCapture(
  stationId: string,
  stations: StationConfig[]
): SonyPtpCaptureResult {
  const station = stations.find((candidate) => candidate.id === stationId.trim());
  if (!station) {
    return {
      success: false,
      stationId,
      error: `Invalid station id: ${stationId}`,
      nextActions: ['Choose one of the configured station ids: camera-1, camera-2, camera-3.']
    };
  }

  return {
    success: false,
    stationId: station.id,
    error: 'Sony/PTP direct capture backend is not implemented yet. Windows detects the camera, but PC Remote requires a Sony/PTP handshake that is not exposed through WIA on this device.',
    nextActions: [
      'Use Mass Storage + Import Latest as the current no-extra-software workflow.',
      'If dashboard-triggered shutter is required, approve a native PTP/Sony backend spike using Sony SDK, Camera Remote Command, libusb/WinUSB, or a dedicated helper.',
      'Keep USB Connection = PC Remote only for the future PTP backend; use Mass Storage for current import workflow.'
    ]
  };
}

function buildSummary(
  sonyUsbDetected: boolean,
  sonyWpdDetected: boolean,
  wiaCaptureAvailable: boolean
): string {
  if (!sonyUsbDetected) {
    return 'Sony camera is not detected by the current USB scan.';
  }
  if (wiaCaptureAvailable) {
    return 'A Windows WIA capture-capable device is available; use the Direct Capture panel first.';
  }
  if (sonyWpdDetected) {
    return 'Sony camera is detected as WPD/PC Remote, but Windows WIA does not expose a capture command. PC Remote is waiting for a Sony/PTP handshake.';
  }
  return 'Sony camera is detected, but no built-in Windows direct-capture path is available.';
}

function buildNextActions(
  sonyUsbDetected: boolean,
  sonyWpdDetected: boolean,
  wiaCaptureAvailable: boolean
): string[] {
  if (!sonyUsbDetected) {
    return [
      'Check USB cable, camera power, and Windows Device Manager.',
      'Try USB Connection modes: PC Remote, MTP, Mass Storage, Auto.',
      'Use Refresh USB Scan after reconnecting the camera.'
    ];
  }

  if (wiaCaptureAvailable) {
    return ['Try Capture Camera 1/2/3 in the Direct Capture panel.'];
  }

  if (sonyWpdDetected) {
    return [
      'This confirms the camera is visible to Windows as WPD/PC Remote.',
      'The stuck “Connecting USB” camera screen means it is waiting for Sony/PTP session initialization.',
      'Use Mass Storage + Import Latest for now, or approve a native Sony/PTP backend implementation spike.'
    ];
  }

  return [
    'Use Mass Storage + Import Latest if the camera appears as a drive.',
    'Approve native Sony/PTP research if dashboard-triggered shutter remains required.'
  ];
}
