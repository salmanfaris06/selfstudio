import { execFile } from 'node:child_process';
import { detectUsbCameras } from './usb-camera-detector.js';
import { getDirectCaptureCapabilities } from './direct-capture.js';

export type SonyNativeFeasibilityResponse = {
  platform: NodeJS.Platform;
  supported: boolean;
  status: 'ok' | 'unsupported' | 'error';
  scannedAt: string;
  verdict: 'ready-for-native-research' | 'needs-camera' | 'windows-only' | 'error';
  cameraIdentity: {
    vid: string | null;
    pid: string | null;
    instanceId: string | null;
    name: string | null;
    className: string | null;
    manufacturer: string | null;
  } | null;
  constraints: string[];
  recommendedBackendOptions: string[];
  diagnostics: {
    usbDevices: unknown[];
    wiaDevices: unknown[];
    pnpDetails: PnpDetail[];
  };
  error: string | null;
};

type PnpDetail = {
  name: string | null;
  className: string | null;
  manufacturer: string | null;
  status: string | null;
  instanceId: string | null;
  service: string | null;
  classGuid: string | null;
  hardwareIds: string[];
  compatibleIds: string[];
  locationPaths: string[];
  driverInfPath: string | null;
  driverProvider: string | null;
  rawProperties: Record<string, unknown>;
};

type PowerShellNativeResult = {
  devices?: PnpDetail[] | PnpDetail | null;
  errors?: string[] | string | null;
};

const NATIVE_FEASIBILITY_SCRIPT = String.raw`
$pattern = 'Sony|ILCE-6000|ILCE 6000|VID_054C&PID_094E'
$items = @()
$errors = @()

function Get-PropValue($InstanceId, $KeyName) {
  try {
    $prop = Get-PnpDeviceProperty -InstanceId $InstanceId -KeyName $KeyName -ErrorAction Stop
    return $prop.Data
  } catch {
    return $null
  }
}

try {
  $devices = Get-PnpDevice -PresentOnly -ErrorAction Stop | Where-Object {
    (($_.FriendlyName, $_.Class, $_.Manufacturer, $_.InstanceId) -join ' ') -match $pattern
  }

  foreach ($device in $devices) {
    $instanceId = $device.InstanceId
    $hardwareIds = @(Get-PropValue $instanceId 'DEVPKEY_Device_HardwareIds')
    $compatibleIds = @(Get-PropValue $instanceId 'DEVPKEY_Device_CompatibleIds')
    $locationPaths = @(Get-PropValue $instanceId 'DEVPKEY_Device_LocationPaths')
    $service = Get-PropValue $instanceId 'DEVPKEY_Device_Service'
    $classGuid = Get-PropValue $instanceId 'DEVPKEY_Device_ClassGuid'
    $driverInfPath = Get-PropValue $instanceId 'DEVPKEY_Device_DriverInfPath'
    $driverProvider = Get-PropValue $instanceId 'DEVPKEY_Device_DriverProvider'

    $items += [pscustomobject]@{
      name = $device.FriendlyName
      className = $device.Class
      manufacturer = $device.Manufacturer
      status = $device.Status
      instanceId = $instanceId
      service = if ($null -eq $service) { $null } else { [string]$service }
      classGuid = if ($null -eq $classGuid) { $null } else { [string]$classGuid }
      hardwareIds = @($hardwareIds | Where-Object { $null -ne $_ } | ForEach-Object { [string]$_ })
      compatibleIds = @($compatibleIds | Where-Object { $null -ne $_ } | ForEach-Object { [string]$_ })
      locationPaths = @($locationPaths | Where-Object { $null -ne $_ } | ForEach-Object { [string]$_ })
      driverInfPath = if ($null -eq $driverInfPath) { $null } else { [string]$driverInfPath }
      driverProvider = if ($null -eq $driverProvider) { $null } else { [string]$driverProvider }
      rawProperties = [pscustomobject]@{
        problemCode = Get-PropValue $instanceId 'DEVPKEY_Device_ProblemCode'
        installState = Get-PropValue $instanceId 'DEVPKEY_Device_InstallState'
        parent = Get-PropValue $instanceId 'DEVPKEY_Device_Parent'
        children = @(Get-PropValue $instanceId 'DEVPKEY_Device_Children')
      }
    }
  }
} catch {
  $errors += "Native feasibility PnP scan failed: $($_.Exception.Message)"
}

[pscustomobject]@{
  devices = $items
  errors = $errors
} | ConvertTo-Json -Depth 8 -Compress
`;

export async function getSonyNativeFeasibility(): Promise<SonyNativeFeasibilityResponse> {
  if (process.platform !== 'win32') {
    return {
      platform: process.platform,
      supported: false,
      status: 'unsupported',
      scannedAt: new Date().toISOString(),
      verdict: 'windows-only',
      cameraIdentity: null,
      constraints: ['Native Sony/PTP feasibility probe currently targets Windows because the camera is connected on Windows.'],
      recommendedBackendOptions: [],
      diagnostics: { usbDevices: [], wiaDevices: [], pnpDetails: [] },
      error: 'Windows only.'
    };
  }

  try {
    const [usb, wia, pnpDetails] = await Promise.all([
      detectUsbCameras({ force: true }),
      getDirectCaptureCapabilities({ force: true }),
      runNativePnPScan()
    ]);

    const sonyDevice = usb.devices.find((device) => {
      const text = [device.name, device.manufacturer, device.instanceId, device.matchedTerms.join(' ')]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return text.includes('sony') || text.includes('ilce') || text.includes('vid_054c');
    });

    const cameraIdentity = sonyDevice
      ? {
          vid: extractUsbPart(sonyDevice.instanceId, 'VID'),
          pid: extractUsbPart(sonyDevice.instanceId, 'PID'),
          instanceId: sonyDevice.instanceId,
          name: sonyDevice.name,
          className: sonyDevice.className,
          manufacturer: sonyDevice.manufacturer
        }
      : null;

    const wiaCaptureAvailable = wia.captureCapableCount > 0;
    const sonyWpd = sonyDevice?.className?.toLowerCase() === 'wpd';

    return {
      platform: process.platform,
      supported: true,
      status: 'ok',
      scannedAt: new Date().toISOString(),
      verdict: cameraIdentity ? 'ready-for-native-research' : 'needs-camera',
      cameraIdentity,
      constraints: buildConstraints(Boolean(cameraIdentity), Boolean(sonyWpd), wiaCaptureAvailable),
      recommendedBackendOptions: buildBackendOptions(Boolean(cameraIdentity), Boolean(sonyWpd), wiaCaptureAvailable),
      diagnostics: {
        usbDevices: usb.devices,
        wiaDevices: wia.devices,
        pnpDetails
      },
      error: [usb.error, wia.error].filter(Boolean).join(' | ') || null
    };
  } catch (error) {
    return {
      platform: process.platform,
      supported: true,
      status: 'error',
      scannedAt: new Date().toISOString(),
      verdict: 'error',
      cameraIdentity: null,
      constraints: [],
      recommendedBackendOptions: [],
      diagnostics: { usbDevices: [], wiaDevices: [], pnpDetails: [] },
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

function runNativePnPScan(): Promise<PnpDetail[]> {
  return new Promise((resolve, reject) => {
    execFile(
      'powershell.exe',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', NATIVE_FEASIBILITY_SCRIPT],
      { timeout: 10_000, windowsHide: true, maxBuffer: 1024 * 1024 },
      (error, stdout, stderr) => {
        if (error && !stdout.trim()) {
          reject(new Error(sanitizeError(stderr.trim() || error.message)));
          return;
        }
        try {
          const parsed = JSON.parse(stdout.trim()) as PowerShellNativeResult | null;
          if (!parsed || typeof parsed !== 'object') {
            resolve([]);
            return;
          }
          resolve(normalizePnpDetails(parsed.devices));
        } catch {
          resolve([]);
        }
      }
    );
  });
}

function normalizePnpDetails(value: PowerShellNativeResult['devices']): PnpDetail[] {
  if (!value) {
    return [];
  }
  return Array.isArray(value) ? value : [value];
}

function buildConstraints(cameraDetected: boolean, sonyWpd: boolean, wiaCaptureAvailable: boolean): string[] {
  if (!cameraDetected) {
    return ['Sony ILCE-6000 is not currently visible to the native feasibility probe.'];
  }

  const constraints = [
    'Camera identity is visible through Windows PnP, so native research can target the concrete VID/PID and instance id.',
    'WIA reports no capture command for ILCE-6000, so WIA is not the dashboard-shutter backend.'
  ];

  if (sonyWpd) {
    constraints.push('The camera is bound as WPD/portable device in PC Remote mode; low-level USB access may require a native helper and may conflict with the Windows WPD stack.');
  }

  if (!wiaCaptureAvailable) {
    constraints.push('A future backend must initialize the Sony/PTP session itself instead of relying on Windows WIA.');
  }

  return constraints;
}

function buildBackendOptions(cameraDetected: boolean, sonyWpd: boolean, wiaCaptureAvailable: boolean): string[] {
  if (!cameraDetected) {
    return ['Reconnect Sony A6000 in PC Remote mode and rerun /api/sony-ptp/native-feasibility.'];
  }

  if (wiaCaptureAvailable) {
    return ['Use existing WIA Direct Capture path first.'];
  }

  const options = [
    'Research Sony Camera Remote Command / Sony SDK compatibility for ILCE-6000.',
    'Research PTP/MTP command access for VID_054C PID_094E and whether object download after capture is exposed.',
    'Prototype a small native helper process that the Node server can call, keeping the dashboard as the only operator UI.'
  ];

  if (sonyWpd) {
    options.push('Check whether Windows WPD binding blocks raw USB access or whether MTP/PTP commands can be sent through a supported Windows API/helper.');
  }

  return options;
}

function extractUsbPart(instanceId: string | null, part: 'VID' | 'PID'): string | null {
  const match = instanceId?.match(new RegExp(`${part}_([0-9A-F]{4})`, 'i'));
  return match ? match[1].toUpperCase() : null;
}

function sanitizeError(error: string): string {
  return error.replace(/[A-Z]:\\[^\s"']+/gi, '[local-path]').slice(0, 800);
}
