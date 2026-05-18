---
title: 'gPhoto2 Helper Diagnostic Spike'
type: 'feature'
created: '2026-05-16'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-wpd-probe-spike.md'
---

## Intent

Add a safe gphoto2 helper path for Sony A6000 direct capture research. The app does not install dependencies or change drivers; it detects whether `gphoto2` is available natively or through WSL and can optionally run capture if available.

## Implementation

- Added `src/server/gphoto-helper.ts`.
- Added `GET /api/gphoto/diagnostics`.
- Added `POST /api/gphoto/capture`.
- Added dashboard `gPhoto2 Helper` panel and per-station capture buttons.
- Added debug logging under area `gphoto`.

## Safety

- No install commands.
- No driver changes.
- Capture validates station id and writes only inside station input folder.
- Capture is globally serialized.
- Captured file must exist and start with JPEG SOI bytes.

## Suggested Review Order

- gPhoto helper implementation.
  [`gphoto-helper.ts`](../../src/server/gphoto-helper.ts)

- gPhoto endpoints and logging.
  [`index.ts`](../../src/server/index.ts)

- Dashboard helper panel.
  [`index.html`](../../src/client/index.html)

## One-click Setup Addendum

Added one-click helper setup endpoint and dashboard button:

- `POST /api/gphoto/setup`
- Implementation: `src/server/gphoto-autosetup.ts`
- Dashboard button: `One-click Setup` in gPhoto2 Helper panel.

Behavior:

- Detects default WSL2 distro.
- Verifies `gphoto2` inside WSL.
- Finds Sony `054c:094e` bus id from `usbipd list`.
- Runs `usbipd bind` and tolerates already-shared state.
- Runs `usbipd attach --busid <busid> --wsl <distro>`.
- Re-runs gPhoto diagnostics.

Limitations:

- If `usbipd bind` requires admin, setup returns `needs-admin` with exact command.
- If WSL/gphoto2 is missing, setup returns manual install instructions.
- It does not install dependencies or change drivers automatically.

## Setup Before Capture Addendum

`POST /api/gphoto/capture` now automatically runs `autoSetupGPhoto()` before calling gphoto2 capture. This makes the capture button itself a setup+capture action for daily operation.

- If setup returns `ready`, capture proceeds.
- If setup returns `needs-admin`, `needs-gphoto`, `needs-wsl`, `needs-camera`, or `attach-failed`, capture returns setup details and next actions.
- Dashboard button label changed to `Setup + Capture`.

## Continuous Capture Addendum

Added auto/continuous capture mode:

- `GET /api/gphoto/continuous/status`
- `POST /api/gphoto/continuous/start`
- `POST /api/gphoto/continuous/stop`

Dashboard gPhoto card now has:

- `Single Capture Camera X` for one photo.
- `Start Auto Capture Camera X` to keep taking photos every ~3 seconds.
- `Stop Auto Capture Camera X` to stop the loop.

Continuous capture runs setup once before starting, then loops `captureWithGPhoto()` until stopped. Captures are serialized globally to avoid camera contention.

## Camera-trigger Listener Addendum

Added mode for physical camera shutter trigger:

- `GET /api/gphoto/trigger-listener/status`
- `POST /api/gphoto/trigger-listener/start`
- `POST /api/gphoto/trigger-listener/stop`

Dashboard now exposes:

- `Capture from Dashboard Camera X` for software-triggered capture.
- `Start Camera Trigger Camera X` to listen for physical shutter events from the camera and download JPGs.
- `Stop Camera Trigger Camera X` to stop listening.

Implementation uses `gphoto2 --wait-event-and-download=<seconds>` in a loop. Operator starts listener once, then presses the physical camera shutter as many times as needed.
