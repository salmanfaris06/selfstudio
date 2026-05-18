---
title: 'Sony PTP Diagnostic Spike'
type: 'feature'
created: '2026-05-16'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - 'D:/_Project/selfstudio/_bmad-output/planning-artifacts/prd.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md'
  - 'D:/_Project/selfstudio/_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sony A6000/ILCE-6000 sudah terdeteksi sebagai WPD di mode PC Remote, tetapi Windows WIA melaporkan `Can Capture: No` dan kamera stuck di `Connecting USB`. Sistem perlu menjelaskan apakah direct capture bisa dilakukan tanpa software tambahan, atau apakah perlu backend PTP/Sony native.

**Approach:** Tambahkan Sony/PTP diagnostic spike yang membaca status device Sony dari Windows, membandingkannya dengan kemampuan WIA, dan menampilkan readiness matrix untuk jalur direct capture. Endpoint capture PTP dibuat safe-fail: tidak mengklaim bisa capture tanpa backend PTP native, tetapi memberi pesan teknis jelas dan next action.

## Boundaries & Constraints

**Always:**

- Jangan install dependency native, driver, Sony SDK, Imaging Edge, atau libusb tanpa approval.
- Diagnosis harus membedakan USB/WPD detected, WIA capture capability, dan PTP/native backend availability.
- Endpoint harus JSON-safe dan tidak crash jika device tidak ada.
- Dashboard harus menjelaskan mengapa kamera stuck `Connecting USB` saat PC Remote menunggu handshake Sony/PTP.
- Existing USB detection, import latest, direct WIA capture, dan watcher tetap berjalan.

**Ask First:**

- Jika ingin menambah native helper, libusb/WinUSB, Sony SDK, atau PTP command implementation sungguhan.
- Jika ingin mengubah driver Windows device kamera.
- Jika ingin reverse engineer Sony opcodes langsung.

**Never:**

- Jangan klaim capture PTP berhasil tanpa benar-benar mengirim shutter command.
- Jangan mematikan atau mengubah device driver otomatis.
- Jangan implement session/customer/timer/LUT/Google Drive di spike ini.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| ILCE-6000 WPD detected | Kamera PC Remote muncul sebagai WPD Sony | `/api/sony-ptp/capabilities` menunjukkan usb/wpd detected, WIA capture unavailable, PTP backend unavailable | Dashboard memberi next step native backend |
| No Sony device | Kamera tidak terhubung | Capability shows no Sony/PTP candidate | Tidak crash |
| WIA capture available | WIA reports capture-capable device | Matrix shows WIA path available and suggests Direct Capture panel | PTP endpoint tetap safe-fail jika backend unavailable |
| PTP capture requested | User klik PTP Capture | Returns clear unsupported/backend-missing JSON | Tidak membuat file palsu |

</frozen-after-approval>

## Code Map

- `src/server/sony-ptp-diagnostic.ts` -- modul baru untuk build diagnostic matrix dari USB detection dan WIA capabilities.
- `src/server/index.ts` -- tambah endpoint `/api/sony-ptp/capabilities` dan `/api/sony-ptp/capture`.
- `src/client/index.html` -- tambah panel Sony/PTP Diagnostic dan tombol safe-fail PTP capture.

## Tasks & Acceptance

**Execution:**

- [x] `src/server/sony-ptp-diagnostic.ts` -- implement capability matrix for Sony/WPD/WIA/native backend -- agar operator paham batas direct capture saat ini.
- [x] `src/server/index.ts` -- expose Sony/PTP diagnostic and safe capture endpoints -- agar dashboard bisa membaca diagnosis.
- [x] `src/client/index.html` -- render Sony/PTP panel and safe PTP capture action -- agar status PC Remote terlihat jelas.
- [x] `_bmad-output/implementation-artifacts/spec-sony-ptp-diagnostic-spike.md` -- update tasks and verification -- menjaga trail BMAD.

**Acceptance Criteria:**

- Given ILCE-6000 appears in USB detection as Sony/WPD, when PTP capabilities endpoint is requested, then response marks Sony/WPD detected and native PTP backend unavailable.
- Given WIA capability count is zero, when dashboard renders, then it explains Windows WIA cannot trigger capture and PC Remote needs Sony/PTP handshake.
- Given PTP capture is requested, when no native backend exists, then API returns `success:false` with a clear unsupported message and no file is created.
- Given existing station watcher is running, when diagnostic endpoints are used, then station APIs still return normally.

## Spec Change Log

## Verification

**Commands:**

- `npm run typecheck` -- expected: TypeScript passes.
- `PORT=3101 npm run dev` -- expected: dashboard loads.
- `curl http://localhost:3101/api/sony-ptp/capabilities` -- expected: JSON diagnostic matrix.
- `curl -X POST http://localhost:3101/api/sony-ptp/capture -H "Content-Type: application/json" -d '{"stationId":"camera-1"}'` -- expected: safe unsupported JSON unless native backend is later added.

## Suggested Review Order

**Sony/PTP diagnostic backend**

- Capability endpoint exposes the Sony/PTP readiness matrix.
  [`index.ts:89`](../../src/server/index.ts#L89)

- Safe capture endpoint explains missing native backend without fake files.
  [`index.ts:97`](../../src/server/index.ts#L97)

- Diagnostic combines USB detection with WIA capture capabilities.
  [`sony-ptp-diagnostic.ts:38`](../../src/server/sony-ptp-diagnostic.ts#L38)

- PTP capture request returns explicit next actions instead of pretending support.
  [`sony-ptp-diagnostic.ts:84`](../../src/server/sony-ptp-diagnostic.ts#L84)

**Dashboard diagnostic UX**

- Sony/PTP panel separates PC Remote/PTP status from WIA capture.
  [`index.html:102`](../../src/client/index.html#L102)

- Safe PTP action shows why native backend is required.
  [`index.html:164`](../../src/client/index.html#L164)

- Diagnostic renderer surfaces USB/WPD/WIA/native backend readiness.
  [`index.html:200`](../../src/client/index.html#L200)
