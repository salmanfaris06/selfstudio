---
title: 'Windows WPD Probe Spike'
type: 'feature'
created: '2026-05-16'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-sony-ptp-diagnostic-spike.md'
---

## Intent

Probe Windows Portable Device (WPD) enumeration for Sony ILCE-6000 in PC Remote mode so future native Sony/PTP backend work has concrete device identifiers and feasibility logs.

## Implementation

- Added `src/server/wpd-probe.ts`.
- Added `GET /api/sony-ptp/wpd-probe`.
- Logs probe output to `data/logs/debug-events.jsonl` with area `wpd-probe`.
- Dashboard Sony/PTP diagnostic now includes summarized WPD probe data.

## Acceptance

- WPD COM `PortableDeviceApi.PortableDeviceManager` availability is reported.
- Returned devices include WPD `deviceId`, friendly name, manufacturer, description, and Sony/ILCE match flags.
- Safe diagnostic only; no shutter command is sent.

## Suggested Review Order

- WPD probe implementation.
  [`wpd-probe.ts`](../../src/server/wpd-probe.ts)

- WPD endpoint and logging.
  [`index.ts`](../../src/server/index.ts)

- Dashboard diagnostic summary.
  [`index.html`](../../src/client/index.html)

## Deep Probe Addendum

Added guarded deep probe endpoint:

- `GET /api/sony-ptp/wpd-deep-probe?deviceId=...`
- Implementation: `src/server/wpd-deep-probe.ts`
- Uses short-lived PowerShell STA process with 5s timeout.
- Purpose: test whether `PortableDeviceApi.PortableDeviceManager` can enumerate and match Sony device id without hanging.
- Still diagnostic only: no shutter command, no driver changes, no native dependency install.
