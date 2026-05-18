---
title: 'Direct Camera Capture Spike'
type: 'feature'
created: '2026-05-16'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - 'D:/_Project/selfstudio/_bmad-output/planning-artifacts/prd.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-3-camera-input-spike.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-import-latest-photo-from-usb-camera.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sistem sudah bisa mendeteksi kamera USB dan mengimpor JPG terbaru dari storage kamera, tetapi operator masih harus menekan shutter manual lalu import. User ingin tahap berikutnya: sistem ini sendiri mencoba melakukan capture langsung dari kamera tanpa software tambahan.

**Approach:** Tambahkan direct capture spike berbasis Windows built-in camera interface (WIA/COM) sebagai percobaan paling ringan tanpa Sony Imaging Edge/lib eksternal. Backend menyediakan capability probe dan capture endpoint; jika kamera mendukung command capture via Windows, sistem trigger capture, mengambil file hasil transfer, menyimpannya ke folder station, lalu watcher existing meng-ingest. Jika tidak didukung, dashboard harus menjelaskan mode/kapabilitas yang gagal.

## Boundaries & Constraints

**Always:**

- Target platform Windows only untuk spike direct capture.
- Gunakan kemampuan bawaan Windows/PowerShell/WIA terlebih dahulu; tidak install Sony SDK, Imaging Edge, libgphoto2, atau native addon.
- Dashboard harus membedakan jelas antara `USB detected`, `storage import available`, dan `direct capture available`.
- Capture harus memilih station tujuan eksplisit: Camera 1/2/3.
- File capture yang berhasil harus disimpan ke folder input station existing agar pipeline watcher tetap dipakai.
- Jika kamera tidak expose WIA capture command, API harus mengembalikan error/capability result jelas, bukan crash.
- Existing import latest, USB detection, dan folder watcher tidak boleh rusak.
- Semua rendering UI tetap DOM-safe.

**Ask First:**

- Jika perlu dependency eksternal, native addon, driver khusus, Sony SDK, atau reverse-engineering PTP opcode.
- Jika perlu menghapus foto dari kamera.
- Jika perlu auto-capture berkala atau burst capture.
- Jika perlu mengubah kamera dari Mass Storage ke PC Remote sebagai asumsi wajib di produk.

**Never:**

- Jangan klaim direct capture pasti didukung Sony A6000 sebelum capability probe membuktikannya.
- Jangan mengganti folder watcher sebagai source of truth.
- Jangan implementasi session/customer/timer/LUT/Google Drive di spike ini.
- Jangan overwrite file existing di folder station.
- Jangan menjalankan command shell yang berasal dari input user.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| WIA camera supports capture | Kamera expose Windows WIA device dan take-picture command | Klik Capture Camera 1 menghasilkan JPG di `data/input/camera-1`; watcher ingest | Jika transfer gagal, tampilkan error tanpa crash |
| Camera detected only as DiskDrive | Kamera hanya muncul sebagai storage/mass storage | Capability probe menyatakan direct capture unavailable dan menyarankan mode PC Remote/PTP jika tersedia | Import Latest tetap bisa digunakan |
| No WIA device | Windows tidak expose WIA camera | API returns unsupported/no-device result | Dashboard menampilkan instruksi jelas |
| Invalid station id | Request capture station bukan `camera-1/2/3` | Request ditolak | Tidak membuat file |
| Concurrent capture | Capture station sama sedang berjalan | Request kedua ditolak 409/in-progress | Tidak duplicate/overwrite |

</frozen-after-approval>

## Code Map

- `src/server/direct-capture.ts` -- modul baru untuk WIA capability probe dan capture attempt via PowerShell COM.
- `src/server/index.ts` -- tambah endpoint `GET /api/direct-capture/capabilities` dan `POST /api/direct-capture/capture`.
- `src/server/stations.ts` -- station config existing sebagai target folder hasil capture.
- `src/client/index.html` -- tambah panel direct capture capability dan tombol Capture per station.
- `src/server/watcher.ts` -- tetap ingest file hasil capture melalui folder watcher existing.

## Tasks & Acceptance

**Execution:**

- [x] `src/server/direct-capture.ts` -- implement WIA capability probe dengan timeout dan normalized JSON result -- agar operator tahu apakah direct capture tersedia.
- [x] `src/server/direct-capture.ts` -- implement capture attempt ke station folder dengan filename unique/exclusive -- agar capture sukses masuk pipeline watcher.
- [x] `src/server/index.ts` -- expose capability and capture endpoints with validation/in-flight protection -- agar UI bisa memanggil fitur dengan aman.
- [x] `src/client/index.html` -- add Direct Capture panel, capability refresh, and capture buttons per station -- agar operator bisa mencoba capture dari dashboard.
- [x] `_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md` -- update task checkboxes and verification notes after implementation -- menjaga trail BMAD.

**Acceptance Criteria:**

- Given app runs on Windows, when capability endpoint is requested, then response includes supported/platform/status/devices/error fields.
- Given no WIA capture-capable camera exists, when dashboard loads, then it shows direct capture unavailable without breaking USB detection/import/watcher.
- Given WIA capture succeeds for a station, when capture completes, then a JPG is written to that station input folder with a collision-safe filename.
- Given copied capture JPG lands in station folder, when watcher sees it, then accepted count for that station increases.
- Given invalid station id or concurrent capture is requested, when API receives it, then request is rejected and no overwrite occurs.

## Spec Change Log

## Design Notes

- Sony A6000 may need `USB Connection → PC Remote` or MTP/PTP-like mode for direct capture; `Mass Storage` usually exposes files only and may not expose remote shutter.
- This spike intentionally tries Windows built-in WIA first because user requested no external software. If WIA fails, the result is still useful: it proves whether pure Windows control is viable on this camera/laptop.
- If WIA does not support capture, the next decision is a human-gated research track: Sony SDK/Camera Remote Command/PTP library/native helper.

## Verification

**Commands:**

- `npm run typecheck` -- expected: TypeScript passes.
- `PORT=3101 npm run dev` -- expected: dashboard loads and existing USB/import/station panels still work.
- `curl http://localhost:3101/api/direct-capture/capabilities` -- expected: JSON capability result.
- `curl -X POST http://localhost:3101/api/direct-capture/capture -H "Content-Type: application/json" -d '{"stationId":"camera-1"}'` -- expected: success with destination file or clear JSON error.

**Manual checks:**

- Try camera USB modes `PC Remote`, `MTP`, and `Mass Storage`; compare capability results.
- If capability says available, click Capture Camera 1 and verify JPG appears in `data/input/camera-1`.
- Verify Camera 1 accepted count increases after watcher processes captured file.

## Suggested Review Order

**Direct capture backend**

- Capability endpoint exposes WIA scan results for dashboard troubleshooting.
  [`index.ts:56`](../../src/server/index.ts#L56)

- Capture endpoint validates station and serializes global WIA capture attempts.
  [`index.ts:65`](../../src/server/index.ts#L65)

- WIA capability script enumerates Windows camera devices and capture commands.
  [`direct-capture.ts:64`](../../src/server/direct-capture.ts#L64)

- Capture script attempts Windows WIA Take Picture and file transfer.
  [`direct-capture.ts:99`](../../src/server/direct-capture.ts#L99)

- Capture entrypoint writes validated JPEG output into station folder.
  [`direct-capture.ts:175`](../../src/server/direct-capture.ts#L175)

**Dashboard controls**

- Direct Capture panel explains WIA availability separately from USB/import.
  [`index.html:102`](../../src/client/index.html#L102)

- Capture buttons are generated per station but gated by capability.
  [`index.html:152`](../../src/client/index.html#L152)

- Capability renderer reports available WIA devices and operator guidance.
  [`index.html:208`](../../src/client/index.html#L208)

**Existing workflow preservation**

- Station refresh keeps capture, import, and watcher cards synchronized.
  [`index.html:468`](../../src/client/index.html#L468)
