---
title: 'USB Camera Detection Spike'
type: 'feature'
created: '2026-05-16'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - 'D:/_Project/selfstudio/_bmad-output/planning-artifacts/prd.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-3-camera-input-spike.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Operator sudah menghubungkan Sony A6000 ke laptop, tetapi app belum memberi indikator apakah kamera USB terdeteksi oleh Windows. Tanpa indikator ini, operator sulit membedakan masalah kamera/kabel/USB mode dari masalah folder watcher.

**Approach:** Tambahkan USB camera detection spike di backend lokal yang membaca daftar perangkat Windows, mengekstrak kandidat kamera/imaging/Sony, menyediakan endpoint API, dan menampilkan ringkasan device terdeteksi di dashboard. Fitur ini hanya mendeteksi kehadiran device OS-level; mapping ke Camera 1/2/3 tetap memakai folder input dan belum melakukan kontrol kamera.

## Boundaries & Constraints

**Always:**

- Deteksi kamera berjalan dari app lokal tanpa cloud service.
- Endpoint harus mengembalikan status platform, waktu scan, jumlah kandidat, daftar kandidat, dan error jika scan gagal.
- Di Windows, gunakan mekanisme OS yang tersedia melalui PowerShell/PNP/CIM tanpa dependency native besar.
- Dashboard harus menampilkan section USB Camera Detection terpisah dari station watcher.
- Jika detection gagal, folder watcher yang sudah ada tetap berjalan.
- Jika platform bukan Windows, API harus memberi status unsupported dengan pesan jelas, bukan crash.
- Data device yang ditampilkan harus aman untuk UI; jangan render via `innerHTML`.

**Ask First:**

- Jika perlu memakai Sony SDK, libgphoto2, native addon, atau install driver/library eksternal.
- Jika perlu melakukan direct camera control, trigger capture, download file dari kamera, atau auto mapping USB device ke station.
- Jika perlu mengubah workflow folder-based ingestion yang sudah berjalan.

**Never:**

- Jangan mengklaim device detection berarti kamera siap capture/tether.
- Jangan menghapus atau mengganti 3-folder JPG ingestion.
- Jangan implementasikan session/customer/timer/LUT/Google Drive di spike ini.
- Jangan menyimpan data device sensitif ke log permanen.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Windows with Sony camera connected | App berjalan di Windows dan kamera muncul di Device Manager/PNP | `/api/usb-cameras` menampilkan satu atau lebih kandidat device; dashboard menunjukkan jumlah kandidat | Jika command sebagian gagal, tampilkan error tanpa mematikan app |
| Windows without camera connected | Tidak ada device Sony/camera/imaging yang present | API sukses dengan `detectedCount: 0`; dashboard memberi instruksi cek kabel/USB mode | Tidak dianggap station error |
| Non-Windows platform | App dijalankan di Linux/macOS/WSL | API mengembalikan `supported: false` dan pesan unsupported | Server tetap berjalan |
| PowerShell unavailable or blocked | Windows tetapi command scan gagal | API mengembalikan `status: error`, `error`, dan daftar kosong | Dashboard menampilkan pesan error |
| Suspicious device text | FriendlyName mengandung karakter HTML | Dashboard menampilkan sebagai teks literal | Tidak boleh eksekusi HTML/script |

</frozen-after-approval>

## Code Map

- `src/server/usb-camera-detector.ts` -- modul baru untuk scan OS-level camera/imaging devices dan normalisasi response.
- `src/server/index.ts` -- tambah endpoint `/api/usb-cameras` tanpa mengganggu endpoint station/events.
- `src/client/index.html` -- tambah panel USB Camera Detection dan polling API secara aman.
- `package.json` -- script tetap cukup untuk menjalankan spike; update hanya jika perlu.

## Tasks & Acceptance

**Execution:**

- [x] `src/server/usb-camera-detector.ts` -- implement Windows PowerShell device scan, unsupported platform response, timeout/error handling -- agar app bisa membaca kandidat kamera USB dari OS.
- [x] `src/server/index.ts` -- expose `GET /api/usb-cameras` -- agar dashboard dan operator bisa mengecek kamera terhubung.
- [x] `src/client/index.html` -- add USB detection panel with candidate list, count, status, and troubleshooting text -- agar operator mendapat feedback visual.
- [x] `src/client/index.html` -- keep all rendering DOM-safe -- mencegah device names merusak dashboard.
- [x] `_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md` -- update task checkboxes and verification notes after implementation -- menjaga trail BMAD.

**Acceptance Criteria:**

- Given app runs on Windows, when `/api/usb-cameras` is requested, then response includes `platform`, `supported`, `status`, `scannedAt`, `detectedCount`, `devices`, and optional `error`.
- Given camera-like USB/imaging device exists in Windows device list, when scan completes, then candidate appears with at least name/friendlyName, class/category, status/present info when available.
- Given no candidate exists, when scan completes, then API returns success with `detectedCount: 0` and dashboard does not show a crash/error.
- Given PowerShell scan fails, when API is requested, then server returns JSON error state and existing station watcher endpoints still work.
- Given dashboard is opened, when USB camera API returns devices, then dashboard shows detection count and device rows above/beside station cards.

## Spec Change Log

## Design Notes

- This is intentionally OS-level detection, not Sony control. It helps answer “Windows melihat kamera atau tidak?” before debugging tethering output folders.
- Initial matching should be broad enough for Sony A6000 variants: `Sony`, `A6000`, `Camera`, `Imaging`, `MTP`, `PTP`, `USB Still Image`, and common PNP camera classes.
- Keep station readiness separate: a detected USB camera does not prove the correct output folder is receiving JPGs.

## Verification

**Commands:**

- `npm run typecheck` -- expected: TypeScript passes.
- `PORT=3101 npm run dev` -- expected: server starts, station watcher still works, and dashboard loads.
- `curl http://localhost:3101/api/usb-cameras` or browser open -- expected: JSON response with detection status.
- `curl http://localhost:3101/api/usb-cameras?force=1` -- expected: bypass cache for a fresh manual USB scan.

**Manual checks:**

- With camera unplugged, verify dashboard shows zero candidates or clear unsupported/error state.
- Plug Sony A6000 in, set camera USB mode as needed, refresh/observe dashboard, and verify candidate list changes if Windows detects it.
- Copy a JPG into `data/input/camera-1` and verify existing ingestion still works.

## Suggested Review Order

**USB detection backend**

- API endpoint exposes cached or forced USB camera scans.
  [`index.ts:34`](../../src/server/index.ts#L34)

- Detector entrypoint handles cache, in-flight scans, and Windows support.
  [`usb-camera-detector.ts:126`](../../src/server/usb-camera-detector.ts#L126)

- PowerShell script collects present PNP/CIM candidate devices.
  [`usb-camera-detector.ts:78`](../../src/server/usb-camera-detector.ts#L78)

- Matching rules identify USB camera-like devices without broad software-camera noise.
  [`usb-camera-detector.ts:62`](../../src/server/usb-camera-detector.ts#L62)

- Normalization dedupes and merges device records from multiple sources.
  [`usb-camera-detector.ts:238`](../../src/server/usb-camera-detector.ts#L238)

**Dashboard integration**

- USB detection panel sits above existing folder watcher cards.
  [`index.html:89`](../../src/client/index.html#L89)

- Device rows render OS data as safe text.
  [`index.html:152`](../../src/client/index.html#L152)

- Manual refresh forces a fresh scan while interval polling remains cached.
  [`index.html:208`](../../src/client/index.html#L208)

**Existing ingest preservation**

- Folder station refresh remains separate from USB detection.
  [`index.html:235`](../../src/client/index.html#L235)
