---
title: '3-Camera Input Spike'
type: 'feature'
created: '2026-05-16'
status: 'done'
baseline_commit: 'NO_VCS'
context:
  - 'D:/_Project/selfstudio/_bmad-output/planning-artifacts/prd.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sistem belum membuktikan risiko teknis paling dasar: menerima input JPG dari 3 camera station/folder berbeda secara stabil di satu PC admin. Sebelum session, LUT, dan Google Drive dibuat, kita perlu memastikan folder input per kamera dapat dipantau, file baru tidak diproses saat masih ditulis, dan asal station tercatat benar.

**Approach:** Bangun local web app minimal untuk technical spike 3-camera input: backend lokal memantau 3 folder, melakukan stable JPG detection, mencegah duplicate ingest event, menyimpan log metadata sederhana, dan frontend sederhana menampilkan status tiap station. Uji awal memakai folder simulasi; nanti folder tersebut bisa diarahkan ke output tethering Sony A6000.

## Boundaries & Constraints

**Always:**

- Buat spike fokus pada 3 input folder/camera station saja.
- Dukung hanya `.jpg` dan `.jpeg` untuk accepted photo input.
- Setiap accepted file harus punya metadata: timestamp, station id, source path, filename, file size, modified time, dan status.
- File harus melewati stable file detection sebelum dicatat sebagai accepted.
- Duplicate watcher events untuk file yang sama tidak boleh membuat record ganda.
- Jika satu folder station bermasalah, station lain yang valid tetap berjalan.
- Implementasi harus runnable secara lokal di Windows project folder ini.

**Ask First:**

- Jika perlu mengganti target dari folder-based watcher ke direct Sony SDK/control.
- Jika perlu menambah session/customer/order/timer ke spike ini.
- Jika perlu menambah LUT processing, Google Drive upload, auth, atau database production.
- Jika dependency runtime utama tidak tersedia dan perlu mengubah pendekatan secara signifikan.

**Never:**

- Jangan implementasi RAW, live preview/video, AI/computer vision, print/share, atau direct camera shutdown.
- Jangan membuat integrasi Google Drive dalam spike ini.
- Jangan menganggap watcher event tunggal berarti file sudah selesai ditulis.
- Jangan membuat session routing seolah-olah sudah final; spike ini hanya membuktikan station input.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Three folders ready | App start dengan `data/input/camera-1`, `camera-2`, `camera-3` tersedia | Tiga station tampil sebagai running/ready | Jika folder tidak ada, hanya station terkait error |
| Single JPG arrives | Copy `.jpg` ke folder Camera 1 | Satu accepted ingest record untuk `camera-1`; last file/count Camera 1 update | Jika file belum stabil, tunggu sampai stabil |
| Near-simultaneous JPGs | Copy JPG ke ketiga folder dalam waktu berdekatan | Semua file diterima dan station attribution benar | Error salah satu station tidak menghentikan station lain |
| Duplicate watcher event | OS memicu lebih dari satu event untuk file sama | Hanya satu accepted ingest record dibuat | Duplicate dicatat/diabaikan tanpa menaikkan count |
| Non-JPG file | Copy `.txt`, `.png`, `.raw`, atau format lain | File diabaikan dan tidak masuk accepted count | Optional ignored log boleh dibuat, tapi bukan accepted photo |
| Missing folder | Salah satu input folder tidak ada saat startup | Station itu menampilkan folder error; station lain tetap aktif | Error harus terlihat di API/UI sederhana |

</frozen-after-approval>

## Code Map

- `package.json` -- scripts, runtime dependencies, dan command untuk menjalankan spike.
- `tsconfig.json` -- konfigurasi TypeScript jika Node/TS digunakan.
- `src/server/index.ts` -- entrypoint local backend dan static dashboard server.
- `src/server/stations.ts` -- konfigurasi default 3 camera station dan input paths.
- `src/server/watcher.ts` -- orchestration watcher per station.
- `src/server/stable-file.ts` -- utility stable JPG detection.
- `src/server/ingest-log.ts` -- append/read JSONL ingest event log.
- `src/server/state.ts` -- in-memory station state dan duplicate key registry.
- `src/client/index.html` -- dashboard sederhana 3 station cards.
- `data/input/camera-1/.gitkeep` -- folder simulasi Camera 1.
- `data/input/camera-2/.gitkeep` -- folder simulasi Camera 2.
- `data/input/camera-3/.gitkeep` -- folder simulasi Camera 3.
- `data/logs/.gitkeep` -- folder log runtime.

## Tasks & Acceptance

**Execution:**

- [x] `package.json` -- create minimal local app scripts -- agar spike bisa dijalankan dengan satu command.
- [x] `src/server/stations.ts` -- define 3 station configs -- menjaga mapping folder ke station eksplisit.
- [x] `src/server/stable-file.ts` -- implement stable file detector -- mencegah partial write diproses.
- [x] `src/server/ingest-log.ts` -- implement JSONL metadata logging -- membuat hasil spike auditable.
- [x] `src/server/state.ts` -- implement station state + duplicate tracking -- mendukung UI status dan duplicate protection.
- [x] `src/server/watcher.ts` -- implement 3 folder watchers -- menerima input dari 3 station paralel.
- [x] `src/server/index.ts` -- expose local dashboard/API -- admin dapat melihat status tanpa membuka log manual.
- [x] `src/client/index.html` -- build simple station dashboard -- tampilkan folder, status, count, last file, last error.
- [x] `data/input/*/.gitkeep` dan `data/logs/.gitkeep` -- create runtime folders -- memudahkan simulasi copy JPG.
- [x] Add verification path -- document command/manual test in output -- memastikan user bisa mencoba spike.

**Acceptance Criteria:**

- Given app started with all three input folders present, when dashboard is opened, then Camera 1–3 show ready/running state.
- Given a JPG is copied into Camera 1 folder, when the file becomes stable, then Camera 1 count increments and log contains one accepted record for Camera 1.
- Given JPGs are copied into all three station folders, when detection completes, then each file is attributed to the correct station.
- Given a non-JPG file is copied into any station folder, when watcher sees it, then accepted photo count does not increase.
- Given duplicate watcher events occur for the same file, when processing completes, then only one accepted record exists for that file identity.
- Given one input folder is missing on startup, when the app runs, then only that station shows an error and other stations remain usable.

## Spec Change Log

## Suggested Review Order

**Entry point and station orchestration**

- Server startup wires station state, watchers, API, and dashboard hosting.
  [`index.ts:14`](../../src/server/index.ts#L14)

- Watcher setup keeps each camera folder isolated and ignores startup backlog.
  [`watcher.ts:19`](../../src/server/watcher.ts#L19)

- Default stations map three logical cameras to three local input folders.
  [`stations.ts:11`](../../src/server/stations.ts#L11)

**Stable ingest and duplicate protection**

- Stable-file polling rejects non-JPGs, zero-byte files, and endless writes.
  [`stable-file.ts:25`](../../src/server/stable-file.ts#L25)

- File event handling waits for stability before logging accepted metadata.
  [`watcher.ts:55`](../../src/server/watcher.ts#L55)

- Runtime state tracks accepted paths to suppress duplicate watcher events.
  [`state.ts:87`](../../src/server/state.ts#L87)

- JSONL logging records accepted files and tolerates corrupt history lines.
  [`ingest-log.ts:17`](../../src/server/ingest-log.ts#L17)

**Operator dashboard**

- Dashboard renders three station cards from API data without innerHTML injection.
  [`index.html:78`](../../src/client/index.html#L78)

- Dashboard refresh polls station state every 1.5 seconds.
  [`index.html:107`](../../src/client/index.html#L107)

**Configuration and runnable spike scaffold**

- Package scripts define local dev and typecheck commands.
  [`package.json:6`](../../package.json#L6)

- TypeScript config keeps the spike strict and Node ESM-compatible.
  [`tsconfig.json:2`](../../tsconfig.json#L2)

## Verification

**Commands:**

- `npm install` -- expected: dependencies install successfully.
- `npm run dev` -- expected: local server starts and prints dashboard URL.
- Manual copy JPG files into `data/input/camera-1`, `data/input/camera-2`, and `data/input/camera-3` -- expected: dashboard counts and `data/logs/ingest-events.jsonl` update correctly.

**Manual checks:**

- Open dashboard URL, verify three station cards.
- Copy one JPG per folder and verify station attribution.
- Copy a non-JPG file and verify it is ignored.
- Temporarily rename one input folder and restart app; verify station-specific error only.
