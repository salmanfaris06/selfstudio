# Hardware gPhoto2 Smoke Test

Dokumen ini menjelaskan guided real-hardware smoke test untuk memastikan alur kamera fisik → gPhoto2 tether listener → station input folder → watcher/ingestion → local original → graded processing → optional Google Drive siap sebelum event.

## Prinsip keselamatan

- Local-first: Google Drive optional; local-only tetap valid.
- Tidak menghapus file kamera, station input, output original/graded, quarantine, report, atau Drive.
- Tidak menjalankan privileged USB/setup/install/driver/global kill action seperti `usbipd bind/attach/detach`, installer package, driver change, `taskkill`, atau `pkill`.
- Tidak bypass folder watcher/ingestion. Smoke hanya memverifikasi state backend setelah file benar-benar masuk ke input folder.
- Report operator hanya memuat field aman: basename file, safe camera summary, status/action; tanpa raw port, identity key, absolute path, command, stdout/stderr, token, atau credential.

## API

`POST /api/stations/{station_id}/hardware-smoke-tests`

Body contoh local-only:

```json
{
  "mode": "local_only",
  "stop_on_failure": true,
  "timeout_ms": 30000,
  "allow_active_session": false,
  "restore_previous_listener_state": false
}
```

Mode:

- `local_only`: Drive tidak diverifikasi; Drive dicatat `not_configured`/`skipped`.
- `drive_optional`: sama-sama tidak memblokir local pass ketika Drive absent.
- `drive_verify`: Drive status diverifikasi setelah local original dan graded processing berhasil; kegagalan Drive dicatat sebagai failure/warning sesuai report.

Report disimpan di `local-data/reports/hardware-smoke/` dengan nama unik `hardware-smoke-<timestamp>-<report_id>.json`.

## Checklist manual real hardware

1. Pastikan Windows/WSL/gPhoto2 tersedia dan kamera fisik (misalnya Sony A6000) tersambung.
2. Assign kamera ke station melalui dashboard/settings.
3. Pastikan station input folder, output folder, dan LUT siap.
4. Jalankan agent dan web dashboard.
5. Jalankan hardware smoke mode `local_only` untuk station yang dipilih.
6. Saat report/status meminta, tekan shutter fisik kamera.
7. Pastikan report menunjukkan:
   - `gphoto2_discovery` passed,
   - `listener_started_or_running` passed,
   - `downloaded_file_detected_in_input_folder` passed,
   - `ingestion_verified` passed,
   - `local_original_verified` passed,
   - `graded_processing_verified` passed,
   - Drive `skipped` atau `not_configured` bila Drive tidak disiapkan.
8. Jika Drive configured dan perlu diuji, jalankan ulang dengan `mode=drive_verify` dan catat hasil upload status.

## Catatan validasi

Automated tests memakai fake verifier dan tidak membuktikan kamera nyata. Jangan klaim hardware-pass kecuali checklist di atas benar-benar dijalankan dengan kamera fisik dan ringkasan report dicatat di Dev Agent Record story.
