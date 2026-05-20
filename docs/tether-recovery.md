# gPhoto2 Tether Recovery

Story 7.5 adds safe recovery for gPhoto2 tether listeners during an event.

## Operator Contract

- Listener start and manual retry set `desired_state=running`.
- Listener stop sets `desired_state=stopped` and cancels pending recovery.
- `auto_restart_enabled` defaults to `false` and must be explicitly enabled per station.
- Startup recovery schedules a safe restart only when `desired_state=running` and `auto_restart_enabled=true`.
- Unexpected process exit leaves desired state as `running` so eligible stations can recover.

## Safety Boundaries

Recovery only uses persisted station assignment and `station.input_folder`. It does not accept runtime path/port overrides from the dashboard.

Recovery never runs privileged or setup actions automatically, including:

- `usbipd bind`, `usbipd attach`, or `usbipd detach`
- driver/Zadig/registry changes
- `winget`, `choco`, or `apt install`
- global process kill commands such as taskkill or pkill
- camera file deletion or local input/output/quarantine/result deletion
- direct Google Drive upload or credential checks

If the listener cannot restart, API/SSE/activity messages expose safe action codes such as `CHECK_USBIPD`, `INSTALL_GPHOTO2`, `CHECK_WSL`, `CONNECT_CAMERA`, `CHECK_CAMERA_USB_MODE`, or `CHECK_STATION_INPUT_FOLDER`.

## Ingestion Boundary

Reconnect only restarts the gPhoto2 wait-event/download listener. JPG files continue to land in the same station input folder. The existing folder watcher/stable-file/idempotency pipeline remains the only ingestion source of truth. Recovery does not create photo, session, processing, quarantine, or upload records directly.

## Backoff

Recovery uses per-station bounded backoff and pauses after repeated failures. Manual retry is authenticated/trusted-origin and duplicate-start safe.

## Events

The agent publishes sanitized `camera.tether_recovery_updated` SSE events with station recovery status. Dashboard cards invalidate tether/readiness/activity data and display safe Indonesian operator copy.
