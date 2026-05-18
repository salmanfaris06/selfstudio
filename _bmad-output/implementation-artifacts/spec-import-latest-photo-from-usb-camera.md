---
title: 'Import Latest Photo From USB Camera'
type: 'feature'
created: '2026-05-16'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - 'D:/_Project/selfstudio/_bmad-output/planning-artifacts/prd.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-3-camera-input-spike.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Kamera Sony A6000 sudah terdeteksi Windows sebagai `Sony DSC USB Device` / `DiskDrive`, tetapi sistem belum bisa mengambil foto dari storage kamera dan menyalinnya ke folder station. Operator ingin memakai sistem ini saja tanpa membuka software tethering tambahan.

**Approach:** Tambahkan fitur import latest photo dari kamera USB yang terdeteksi sebagai storage/disk drive. Backend mencari volume/drive letter kamera Sony, memindai folder DCIM untuk JPG/JPEG terbaru, menyalinnya ke folder station yang dipilih (`camera-1`, `camera-2`, atau `camera-3`), lalu folder watcher existing akan ingest file tersebut seperti input JPG biasa.

## Boundaries & Constraints

**Always:**

- Implementasi target Windows karena device terdeteksi sebagai DiskDrive di Windows.
- Fokus pada opsi A: import latest photo dari kamera/storage, bukan trigger shutter.
- Import harus hanya mengambil `.jpg`/`.jpeg`; RAW dan video diabaikan.
- Import harus memilih station tujuan eksplisit: Camera 1/2/3.
- File hasil copy harus masuk ke folder input station existing agar pipeline watcher tetap dipakai.
- Nama file hasil copy harus collision-safe dan menyertakan indikasi asal import.
- API harus mengembalikan hasil jelas: success/error, source file, destination path, station id, file size, modified time.
- Jika kamera/drive/DCIM tidak ditemukan, tampilkan error yang membantu operator.
- Existing USB detection dan folder watcher tidak boleh rusak.

**Ask First:**

- Jika perlu melakukan trigger capture/shutter remote.
- Jika perlu memakai Sony SDK/libgphoto2/native USB library/driver eksternal.
- Jika perlu menghapus foto dari kamera setelah import.
- Jika perlu auto-import berkala tanpa klik operator.

**Never:**

- Jangan klaim fitur ini bisa mengambil gambar baru dari kamera; ini hanya import latest existing JPG.
- Jangan implementasikan session/customer/timer/LUT/Google Drive di spike ini.
- Jangan copy RAW/video.
- Jangan overwrite file existing di folder station.
- Jangan mengubah mapping station folder yang sudah berjalan.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Sony storage mounted | `Sony DSC USB Device` muncul sebagai drive dengan DCIM berisi JPG | Klik Import Latest ke Camera 1 menyalin JPG terbaru ke `data/input/camera-1`; watcher meng-ingest file | Jika copy gagal, API mengembalikan error tanpa crash |
| No camera drive | Kamera tidak mounted sebagai logical drive | Import API gagal dengan pesan `camera storage not found` | Dashboard menampilkan instruksi cek USB mode Mass Storage/MTP |
| No JPG on camera | Drive ditemukan tetapi tidak ada JPG/JPEG | Import API gagal dengan pesan `no JPG found` | Tidak membuat file output kosong |
| Duplicate filename | JPG terbaru punya nama sama dengan file sebelumnya | Destination name dibuat unik/collision-safe | Tidak overwrite file lama |
| Wrong station id | Request station bukan `camera-1/2/3` | API menolak request | Tidak copy file ke folder random |

</frozen-after-approval>

## Code Map

- `src/server/camera-storage.ts` -- modul baru untuk menemukan volume kamera Windows, scan DCIM, pilih JPG terbaru, dan copy ke folder station.
- `src/server/index.ts` -- tambah endpoint `POST /api/camera-import/latest` dan JSON body parsing.
- `src/server/stations.ts` -- gunakan station config existing sebagai tujuan import.
- `src/client/index.html` -- tambah tombol Import Latest ke Camera 1/2/3 dan tampilkan hasil/error.
- `src/server/watcher.ts` -- tetap ingest file hasil copy melalui folder watcher existing.

## Tasks & Acceptance

**Execution:**

- [x] `src/server/camera-storage.ts` -- implement Windows drive discovery untuk Sony disk drive, recursive DCIM JPG scan, latest-file selection, dan safe copy -- agar sistem bisa import foto tanpa software tambahan.
- [x] `src/server/index.ts` -- add JSON body parser and `POST /api/camera-import/latest` -- agar dashboard bisa meminta import ke station tertentu.
- [x] `src/client/index.html` -- add import controls per station and render result/error safely -- agar operator bisa klik import dari UI.
- [x] `src/server/camera-storage.ts` -- validate station id and ensure destination path stays inside station input folder -- mencegah copy ke lokasi tidak valid.
- [x] `_bmad-output/implementation-artifacts/spec-import-latest-photo-from-usb-camera.md` -- update task checkboxes and verification notes after implementation -- menjaga trail BMAD.

**Acceptance Criteria:**

- Given Sony camera storage is mounted and contains JPGs, when operator clicks Import Latest for Camera 1, then latest JPG is copied into `data/input/camera-1` with unique filename.
- Given copied JPG lands in station folder, when watcher sees it, then accepted count for that station increases.
- Given no Sony camera drive is available, when import is requested, then API returns JSON error and server remains running.
- Given no JPG exists under camera storage/DCIM, when import is requested, then no destination file is created and dashboard shows a clear error.
- Given invalid station id is posted, when API receives it, then request is rejected and no copy occurs.

## Spec Change Log

## Design Notes

- Karena device muncul sebagai `DiskDrive`, jalur paling realistis tanpa software tambahan adalah import dari mounted storage, bukan remote capture.
- Untuk Sony A6000, operator kemungkinan perlu mode `Mass Storage` agar memory card muncul sebagai drive Windows. Mode `PC Remote` lebih cocok untuk trigger capture, tapi itu di luar spike ini.
- File hasil import sengaja masuk ke folder watcher agar pipeline sebelumnya tetap menjadi source of truth.

## Verification

**Commands:**

- `npm run typecheck` -- expected: TypeScript passes.
- `PORT=3101 npm run dev` -- expected: server starts and dashboard loads.
- `curl -X POST http://localhost:3101/api/camera-import/latest -H "Content-Type: application/json" -d '{"stationId":"camera-1"}'` -- expected: latest JPG copied or clear JSON error.

**Manual checks:**

- Set kamera ke Mass Storage if needed, connect USB, verify Windows can browse camera/card files.
- Click Import Latest for Camera 1 and verify a JPG appears in `data/input/camera-1`.
- Verify Camera 1 accepted count increases after watcher processes imported file.

## Suggested Review Order

**Import backend**

- Endpoint accepts station target and returns success/error JSON.
  [`index.ts:45`](../../src/server/index.ts#L45)

- Import entrypoint validates station, platform, folder readiness, and camera storage.
  [`camera-storage.ts:75`](../../src/server/camera-storage.ts#L75)

- Windows volume discovery targets Sony/DSC/camera disk devices.
  [`camera-storage.ts:52`](../../src/server/camera-storage.ts#L52)

- DCIM scanner chooses the newest stable JPG/JPEG candidate.
  [`camera-storage.ts:179`](../../src/server/camera-storage.ts#L179)

- Exclusive copy creates collision-safe files inside station input folders.
  [`camera-storage.ts:243`](../../src/server/camera-storage.ts#L243)

**Dashboard controls**

- Import controls render one action per configured station.
  [`index.html:137`](../../src/client/index.html#L137)

- Import request posts station id and preserves per-station result state.
  [`index.html:168`](../../src/client/index.html#L168)

- Station refresh keeps watcher cards and import controls aligned.
  [`index.html:320`](../../src/client/index.html#L320)
