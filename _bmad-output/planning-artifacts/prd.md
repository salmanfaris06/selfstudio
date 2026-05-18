---
stepsCompleted: ['step-01-init', 'step-02-discovery', 'step-02b-vision', 'step-02c-executive-summary', 'step-03-success', 'step-04-journeys', 'step-05-domain', 'step-06-innovation', 'step-07-project-type', 'step-08-scoping', 'step-09-functional', 'step-10-nonfunctional', 'step-11-polish', 'step-12-complete']
workflow_completed: true
date: '2026-05-16'
inputDocuments:
  - "D:/_Project/selfstudio/_bmad-output/brainstorming/brainstorming-session-2026-05-16-133452.md"
workflowType: 'prd'
releaseMode: single-release
documentCounts:
  productBriefs: 0
  research: 0
  brainstorming: 1
  projectDocs: 0
classification:
  projectType: web_app
  domain: event_photography_studio_operations
  complexity: medium
  projectContext: greenfield
---

# Product Requirements Document - selfstudio

**Author:** alpharize  
**Date:** 2026-05-16

## Executive Summary

Aplikasi ini adalah local-network web application untuk studio/event photo booth yang mengelola beberapa kamera dan beberapa sesi customer secara paralel. Sistem berjalan di PC admin yang terhubung ke 3 kamera Sony A6000 via USB, memantau folder input per kamera, menerima file JPG baru, memproses LUT per background/kamera, menyimpan original dan graded output, serta mengarahkan setiap foto ke folder customer/order yang benar.

Produk ini menyelesaikan masalah operasional utama dalam event photo booth multi-background: workflow manual berbasis folder mudah menyebabkan foto salah customer, file tercecer, proses grading tidak konsisten, dan admin tidak memiliki visibilitas real-time atas status kamera, sesi, dan hasil foto. Dengan model camera station, setiap kamera/background memiliki customer aktif, order number, timer, status processing, dan output destination yang eksplisit.

Target pengguna utama adalah admin/operator studio atau event photo booth yang menjalankan beberapa background/kamera bersamaan dan membutuhkan workflow capture-to-delivery yang aman, cepat, dan terstruktur. MVP berfokus pada reliabilitas lokal: JPG-only ingestion, folder watching, session routing, timer-based locking, late photo quarantine, local original+graded storage, operator dashboard, dan Google Drive upload setelah session selesai.

### What Makes This Special

Pembeda utama produk ini adalah sistem memahami konteks operasional event, bukan hanya memantau file. Setiap kamera diperlakukan sebagai station yang terikat ke background, LUT, folder input, customer aktif, order number, dan timer. Foto yang masuk otomatis dikaitkan ke session yang benar, diproses dengan LUT yang tepat, dan disiapkan sebagai output customer yang rapi.

Core insight produk ini: workflow manual berbasis folder tidak cukup aman untuk event paralel. Operator membutuhkan control room yang menunjukkan station mana yang aktif, foto terakhir masuk ke customer siapa, apakah proses original/graded berhasil, apakah ada late photo/quarantine, dan apakah hasil sudah siap di lokal maupun Google Drive.

Produk ini dipilih dibanding workflow manual karena mengurangi kesalahan operasional yang paling mahal: salah routing foto, kehilangan file, grading tidak konsisten, status proses tidak terlihat, dan upload/delivery tidak terpantau. MVP sengaja tidak mencakup live video, RAW, AI/computer vision, print/share, atau real-time cloud upload agar fokus pada capture, routing, processing, storage, dan recovery yang stabil.

## Project Classification

- **Project Type:** Local-network web application
- **Domain:** Event photography / studio operations
- **Complexity:** Medium
- **Project Context:** Greenfield
- **Primary Runtime Context:** Admin PC connected to cameras and accessed by browser over local network
- **Primary Risk Class:** Operational reliability, file-system ingestion, session routing, and event-time operator error

## Success Criteria

### User Success

Admin/operator berhasil jika dapat menjalankan sesi foto paralel di 3 camera station dengan percaya diri, tanpa memindahkan file manual, dan selalu mengetahui foto baru masuk ke customer/order yang benar.

Kriteria user success:

- Operator dapat mengonfigurasi 3 camera station sebelum event melalui Setting Camera.
- Operator dapat menjalankan minimal 3 session customer secara paralel, masing-masing satu per camera station.
- Operator dapat melihat customer/order aktif, timer, status kamera/folder watcher, last photo, jumlah foto, dan status proses di setiap station card.
- Operator dapat mengetahui ketika station tidak siap, kamera/folder bermasalah, disk hampir penuh, foto gagal diproses, atau ada late photo di quarantine.
- Operator dapat menyelesaikan session melalui timer otomatis atau tombol End Session.
- Operator dapat membuka folder hasil lokal setelah session selesai.
- Operator dapat melihat apakah Google Drive upload masih pending, berhasil, atau gagal.
- Operator dapat memulihkan error umum melalui action spesifik: recheck camera, restart watcher, retry photo processing, retry Drive upload, atau assign late photo.

Aha moment:

> “Saya tidak perlu lagi memindahkan foto manual atau khawatir foto customer tertukar; setiap station menunjukkan sesi aktif dan hasilnya otomatis masuk ke folder yang benar.”

### Business Success

Produk berhasil secara bisnis jika menggantikan workflow manual folder-based untuk operasional event/studio multi-background dan mengurangi kesalahan operasional yang berdampak pada kepuasan customer.

Kriteria business success awal:

- MVP dapat digunakan dalam simulasi event internal dengan 3 camera station aktif.
- MVP dapat digunakan oleh operator tanpa developer mendampingi setelah onboarding singkat.
- Workflow manual pemindahan foto ke folder customer dapat dikurangi atau dihilangkan.
- Kesalahan routing foto antar customer/order menjadi nol dalam uji event terkontrol.
- Hasil foto lokal dan Google Drive dapat digunakan sebagai deliverable customer/order.
- Sistem cukup stabil untuk pilot event kecil sebelum dikembangkan ke production event workflow.

Indikator 3 bulan:

- Minimal 3–5 event/simulasi berhasil dijalankan end-to-end.
- Tidak ada laporan foto hilang akibat aplikasi.
- Tidak ada laporan foto salah customer/order.
- Operator dapat menyelesaikan setup station dan readiness check tanpa bantuan teknis.
- Google Drive upload berhasil untuk mayoritas session, dengan retry aman saat gagal.

### Technical Success

Produk berhasil secara teknis jika pipeline lokal menerima, memproses, menyimpan, dan mencatat foto dari 3 camera station secara deterministik dan recoverable.

Kriteria technical success:

- 3 Sony A6000 dapat menghasilkan JPG ke 3 input folder berbeda pada 1 PC admin.
- Folder watcher tidak memproses file sebelum selesai ditulis.
- Sistem memiliki stable file detection sebelum processing.
- Setiap foto memiliki metadata: station, session, source path, original path, graded path, timestamp, status, dan quarantine reason jika relevan.
- Session state dan photo state dikelola eksplisit, bukan hanya berdasarkan keberadaan file.
- Foto hanya masuk ke session yang aktif dan valid.
- Foto setelah session lock masuk quarantine/unassigned.
- Original JPG selalu disimpan sebelum LUT processing.
- LUT failure tidak menghilangkan original.
- Auto retry 3x dan manual retry bekerja untuk processing failure.
- Local output menggunakan naming collision-safe.
- Google Drive upload hanya berjalan setelah local session completion.
- Google Drive upload retryable dan tidak menghapus file lokal saat gagal.
- App restart tidak menghilangkan metadata session/photo/upload job.
- Disk space warning dan readiness validation mencegah session dimulai dalam kondisi tidak aman.

### Measurable Outcomes

Target measurable outcomes untuk MVP validation:

- 3 camera station berjalan paralel selama minimal 2 jam simulasi.
- Minimal 300 foto total diproses dalam simulasi multi-station.
- 0 foto salah customer/order dalam simulasi terkontrol.
- 0 foto hilang setelah local save berhasil.
- 100% original JPG tersimpan sebelum graded processing.
- <2% foto masuk quarantine dalam kondisi operasional normal.
- 100% late photo setelah session lock masuk quarantine, bukan session lama.
- 100% processing failure memiliki status error dan opsi retry.
- 100% session memiliki folder hasil lokal dengan struktur konsisten.
- Google Drive upload failure tidak memblokir session baru.
- Operator dapat menyelesaikan readiness check semua station sebelum event.
- Dashboard menampilkan status utama station dengan delay yang dapat diterima untuk operasional event.

## Product Scope

### MVP - Minimum Viable Product

MVP adalah single-release operational MVP. Semua requirement yang sudah ditetapkan sebagai MVP tetap masuk scope rilis pertama; urutan milestone hanya sequencing implementasi, bukan de-scope.

MVP mencakup:

- Local-network web dashboard yang berjalan di PC admin.
- Camera Management untuk 3 Sony A6000.
- Setting Camera: camera slot name, physical device selection, device identifier, input folder, background name, default LUT, output folder rule, connection status, test capture/test watch, reconnect/refresh.
- 3 camera station cards di dashboard.
- Create session per camera station.
- Customer name + order number + timer.
- One active customer/session per camera station.
- Folder watching per camera input folder.
- JPG-only ingestion.
- Stable file detection.
- Metadata routing berdasarkan camera station dan active session.
- LUT processing per camera/background.
- Save original + graded locally.
- Timer-based session lock.
- End session via timer atau admin UI.
- Late photo quarantine/unassigned.
- Manual move from quarantine.
- Processing status per photo.
- Auto retry 3x dan manual retry.
- Session summary.
- Open local result folder.
- Google Drive upload setelah session selesai.
- Google Drive folder otomatis berdasarkan customer + order.
- Upload original + graded ke Google Drive.
- Retry failed Drive upload.
- Event readiness checklist.
- Disk space warning.
- App health indicator.
- Operator activity log.
- Config backup and restore.
- Duplicate filename protection.
- Missing LUT fallback.
- Local save vs Drive upload status terpisah.
- Startup recovery untuk pending session/photo/upload job.

### Growth Features (Post-MVP)

Tidak masuk MVP, tetapi dapat dipertimbangkan setelah capture workflow terbukti stabil:

- Customer-facing gallery.
- QR download.
- WhatsApp/email sharing.
- Print workflow.
- Advanced customer queue.
- Multi-PC camera node.
- Role-based access control granular.
- Session analytics lanjutan.
- Bulk reprocess LUT.
- Template/preset studio lanjutan.
- Auto cleanup/archive policy.
- Cloud dashboard di luar local network.
- Tablet-optimized dashboard layout.
- Export manifest tambahan selain metadata internal.

### Vision (Future)

Versi vision dapat berkembang menjadi platform operasional lengkap untuk studio/event photo booth multi-station:

- Multi-location event management.
- Cloud-first customer delivery.
- Real-time gallery sharing.
- Advanced station scheduling.
- Automated photo selection.
- AI quality check.
- Integration dengan payment/order system.
- Customer portal.
- Operator mobile companion app.
- Production-grade monitoring dan remote support.

### Explicit Out of Scope for MVP

- Live video preview.
- RAW processing.
- AI/computer vision.
- Print/share langsung.
- Real-time Google Drive upload per foto.
- Hardware-level auto camera shutdown.
- Complex role permission.
- Advanced customer queue.

## User Journeys

### Journey 1 — Operator Menjalankan 3 Station Saat Event Berjalan

Rina adalah operator event photo booth yang harus menjalankan 3 background paralel. Sebelum aplikasi ini ada, ia harus memastikan file dari tiap kamera masuk ke folder yang benar, memindahkan hasil foto ke folder customer, mengecek grading, dan mengingat customer mana yang sedang berada di background tertentu. Saat event ramai, risiko salah folder dan salah customer tinggi.

Dengan aplikasi ini, Rina membuka dashboard dari browser di jaringan lokal. Ia melihat 3 station card: Station 1, Station 2, dan Station 3. Setiap card menampilkan status kamera, folder watcher, customer aktif, order number, timer, LUT aktif, foto terakhir, jumlah foto, dan status processing. Rina membuat session untuk customer di station yang sesuai, lalu sistem mulai menerima JPG dari folder input kamera tersebut.

Saat customer mengambil foto melalui remote trigger shutter, aplikasi mendeteksi file baru, menunggu file stabil, mengaitkan foto ke session aktif di station tersebut, menyimpan original, menerapkan LUT, menyimpan graded output, dan memperbarui preview foto terakhir. Rina melihat foto masuk ke customer/order yang benar tanpa memindahkan file manual.

### Journey 2 — Operator Menghadapi Timer Habis dan Late Photo

Doni mengawasi Station 2. Customer masih mengambil foto terakhir saat timer hampir habis. Timer berakhir sebelum file selesai ditulis ke folder input. Sistem mengunci session dan tidak lagi menerima foto baru ke session tersebut.

Beberapa detik kemudian, file JPG baru muncul dari Station 2. Karena session sudah locked, aplikasi tidak memasukkan foto itu ke customer lama atau customer berikutnya. Foto masuk ke quarantine/unassigned dengan alasan `LATE_PHOTO`. Dashboard menampilkan alert bahwa ada late photo dari Station 2 setelah session customer tertentu berakhir.

Doni membuka review quarantine, melihat thumbnail, timestamp, source station, dan session terakhir. Jika foto memang milik customer sebelumnya, ia memilih action “Assign to Previous Session”. Jika foto adalah test atau tidak relevan, ia membiarkannya di quarantine.

### Journey 3 — Admin Melakukan Setup dan Readiness Check Sebelum Event

Maya adalah admin teknis studio yang menyiapkan perangkat sebelum event. Ia membuka Setting Camera dan mengonfigurasi 3 camera station. Untuk setiap station, ia memilih device/camera identifier, input folder, background name, default LUT, output folder rule, dan menjalankan test watch/test capture.

Sebelum event dimulai, Maya menjalankan Event Readiness Checklist. Sistem memvalidasi folder input dapat dibaca, output folder dapat ditulis, LUT tersedia, disk space cukup, watcher berjalan, processor siap, quarantine folder tersedia, dan Google Drive account terhubung atau statusnya diketahui.

Jika ada masalah seperti LUT hilang atau folder tidak bisa ditulis, sistem menampilkan status `ATTENTION` dengan pesan operator-friendly dan action spesifik. Maya memperbaiki masalah sebelum operator mulai menjalankan session customer.

### Journey 4 — Admin Menyelesaikan Session dan Upload ke Google Drive

Setelah session selesai, operator menekan End Session atau timer berakhir. Sistem mengunci session, memastikan local processing selesai atau error final tercatat, lalu menampilkan summary: total foto, processed, failed, late/unassigned, local folder path, dan Drive upload status.

Admin membuka folder hasil lokal untuk memastikan original dan graded output tersedia. Setelah local completion, sistem membuat folder Google Drive berdasarkan customer + order number dan mengupload original + graded. Jika internet mati atau token Google bermasalah, status upload menjadi pending/failed, tetapi local result tetap aman.

Admin dapat retry upload setelah koneksi kembali. Sistem tidak menghapus file lokal ketika upload gagal, dan Drive upload tidak memblokir session baru.

### Journey 5 — Troubleshooting Saat Ada Gagal Proses

Bima adalah admin/support yang dipanggil ketika Station 1 menampilkan status error. Dashboard menunjukkan beberapa foto gagal diproses karena LUT file tidak ditemukan. Sistem tetap menyimpan original JPG dan menandai graded output sebagai failed/retryable.

Bima membuka log station/session, melihat file yang gagal, alasan kegagalan, retry count, source path, original path, dan action yang tersedia. Setelah memperbaiki LUT, ia menjalankan Retry Photo Processing. Sistem memproses ulang dari original yang aman dan memperbarui status foto.

### Journey Requirements Summary

User journeys mengungkap capability area berikut:

- Camera station configuration dan readiness validation.
- Local-network dashboard dengan 3 station cards.
- Session creation per station dengan customer name, order number, dan timer.
- Folder watcher dengan stable file detection.
- Deterministic routing dari station ke active session.
- Explicit session/photo state machine.
- Timer-based lock dan late photo quarantine.
- Quarantine review dan manual assignment.
- Original + LUT-graded local storage.
- Processing retry dan error recovery.
- Session summary dan open result folder.
- Post-session Google Drive upload queue.
- Operator-friendly alerts, activity logs, dan troubleshooting view.

## Domain-Specific Requirements

Domain event photography / studio operations tidak memiliki compliance berat seperti healthcare atau fintech, tetapi memiliki kompleksitas operasional medium karena sistem berinteraksi dengan hardware kamera, file system lokal, operator event, storage lokal, dan cloud upload pasca-session.

### Compliance & Regulatory

- Tidak ada regulatory approval khusus untuk MVP.
- Sistem menyimpan nama customer, nomor order, metadata session, dan file foto.
- Foto customer harus diperlakukan sebagai data customer yang perlu dijaga di local storage dan Google Drive.
- Akses dashboard local-network harus dibatasi pada jaringan operasional/admin, bukan public internet.
- Akun Google Drive admin harus dikelola sebagai akun operasional resmi.

### Technical Constraints

- Sistem harus berjalan stabil di PC admin yang terhubung ke 3 Sony A6000 via USB.
- Kamera atau tethering workflow harus menghasilkan JPG ke folder input yang konsisten.
- Folder watcher tidak boleh menjadi source of truth tunggal; sistem perlu stable file detection dan metadata store.
- Local storage adalah path utama untuk keselamatan data; Google Drive adalah fulfillment/backup setelah session selesai.
- Session, photo, processing, quarantine, dan upload harus memiliki state eksplisit.
- Sistem harus tahan terhadap USB disconnect, folder missing, disk penuh, file partial write, duplicate watcher event, dan Google Drive failure.
- UI harus memberikan alert operator-friendly, bukan hanya technical logs.

### Integration Requirements

- Integrasi kamera dilakukan melalui workflow folder-based: setiap camera station memiliki input folder yang dipantau.
- Integrasi LUT dilakukan pada file JPG lokal untuk menghasilkan graded output.
- Integrasi Google Drive dilakukan setelah session selesai, menggunakan satu akun Google admin.
- Google Drive folder dibuat otomatis berdasarkan customer name + order number.
- Upload Google Drive harus retryable dan tidak menghapus file lokal saat gagal.

### Risk Mitigations

- Salah routing foto dimitigasi dengan camera station mapping, active session state, config snapshot, timer lock, dan quarantine.
- File partial/corrupt dimitigasi dengan stable file detection sebelum processing.
- Duplicate watcher event dimitigasi dengan idempotent ingest key dan filename collision protection.
- Disk penuh dimitigasi dengan readiness check dan disk warning sebelum session dimulai.
- LUT missing/corrupt dimitigasi dengan original-first save dan failed/retryable graded processing.
- Google Drive failure dimitigasi dengan post-session upload queue, retry, dan status terpisah dari local save.
- Operator error dimitigasi dengan station readiness checklist, clear primary state, action-specific buttons, dan operator activity log.

## Innovation & Novel Patterns

### Detected Innovation Areas

Produk ini bukan inovasi teknologi fundamental, tetapi memiliki inovasi workflow operasional untuk event photo booth multi-station. Nilai uniknya muncul dari penggabungan camera station model, session routing, timer lock, LUT pipeline, quarantine recovery, dan post-session cloud delivery dalam satu control room lokal.

Innovation areas:

- **Camera Station as Operational Unit:** Kamera diperlakukan sebagai station yang terkait ke background, LUT, customer aktif, order number, timer, folder input, dan output rule.
- **Session-Aware File Ingestion:** Sistem tidak hanya memonitor folder; sistem mengaitkan setiap foto ke konteks operasional yang benar.
- **Timer-Based Capture Boundary:** Timer menentukan apakah foto masuk session aktif atau quarantine.
- **Local-First Event Control Room:** Capture, processing, dan storage berjalan lokal untuk reliabilitas event; cloud upload dilakukan setelah session selesai.
- **Quarantine as Recovery Layer:** Foto ambigu/late tidak dibuang dan tidak otomatis salah routing; operator dapat meninjau dan memindahkan manual.

### Market Context & Competitive Landscape

Alternatif umum yang digantikan adalah workflow manual menggunakan folder kamera, Lightroom/manual editing, dan upload Google Drive manual. Alternatif tersebut tidak memahami customer/order/session secara otomatis dan rawan salah saat beberapa customer berjalan paralel.

Produk ini berbeda karena menggabungkan dashboard station real-time, session/customer/order context, folder watcher, LUT automation, local result packaging, Drive upload setelah session, dan operator recovery tools.

### Validation Approach

Validasi inovasi berfokus pada operasional lapangan:

- Operator dapat menjalankan 3 station paralel tanpa memindahkan file manual.
- Foto selalu masuk ke customer/order yang benar dalam simulasi event.
- Timer lock dan quarantine mengurangi risiko foto salah session.
- Operator dapat memahami status sistem tanpa membuka folder/log teknis.
- Google Drive upload pasca-session tidak mengganggu capture workflow.

### Risk Mitigation

Risiko inovasi utama adalah workflow terlalu kompleks untuk operator. Mitigasi:

- Primary state per station harus jelas.
- Action button harus spesifik sesuai masalah.
- Session start/end harus punya konfirmasi dan status eksplisit.
- Quarantine harus terlihat, bukan tersembunyi.
- Local save dan Drive upload harus dipisahkan statusnya.

## Web App Specific Requirements

### Project-Type Overview

Produk ini adalah local-network web application. Frontend diakses melalui browser oleh admin/operator di jaringan lokal, sedangkan backend berjalan di PC admin yang terhubung langsung ke kamera, folder input, storage lokal, dan Google Drive integration.

Aplikasi bukan public SaaS dan bukan static website. Sistem memerlukan backend lokal yang menjalankan background workers untuk folder watching, stable file detection, image processing, session state management, dan Drive upload queue.

### Technical Architecture Considerations

- Aplikasi harus berjalan di PC admin dan dapat diakses dari device lain melalui browser di local network.
- Backend lokal menjadi pusat koordinasi camera station config, session state, photo metadata, processing queue, quarantine, logs, dan upload queue.
- Sistem membutuhkan persistent local database; SQLite direkomendasikan untuk MVP.
- UI harus real-time atau near real-time untuk station status, last photo preview, timer, processing status, dan upload status.
- Folder watcher harus dikombinasikan dengan periodic scanner/stabilizer agar tidak kehilangan event file system.
- Image processing worker harus asynchronous agar tidak memblokir dashboard.
- Google Drive upload worker harus terpisah dari capture/processing workflow.
- Local file system dan database harus dijaga konsistensinya melalui temp file, atomic rename, dan transactional metadata update.

### Browser Matrix

MVP mendukung browser modern pada jaringan lokal:

- Chrome/Chromium terbaru sebagai target utama.
- Edge terbaru sebagai target sekunder.
- Mobile/tablet browser hanya untuk monitoring ringan jika diperlukan, bukan prioritas utama.

Aplikasi tidak membutuhkan SEO, public indexing, atau anonymous public access.

### Responsive Design

Dashboard harus optimal untuk layar laptop/desktop admin:

- 3 station cards terlihat jelas pada desktop/laptop.
- Alert global terlihat tanpa scroll berlebihan.
- Detail view dapat dibuka untuk station/session/photo/log.
- Tablet support boleh adaptif, tetapi bukan target utama MVP.

### Performance Targets

- Foto baru muncul sebagai last photo preview setelah file selesai ditulis dan diproses dalam waktu operasional yang dapat diterima.
- Dashboard status station update near real-time.
- Processing queue tidak memblokir session creation atau station monitoring.
- Google Drive upload tidak mengganggu capture, local save, atau LUT processing.
- Sistem mampu menjalankan simulasi 3 station minimal 2 jam dengan minimal 300 foto total.

### Accessibility Level

MVP menggunakan basic accessibility:

- Status tidak hanya bergantung pada warna; gunakan label teks seperti READY, LIVE, ATTENTION, LOCKED, UPLOAD_FAILED.
- Tombol critical seperti End Session memiliki konfirmasi.
- Error/action labels spesifik dan mudah dipahami operator.
- Font dan contrast cukup jelas untuk kondisi event.

### Implementation Considerations

- Jangan gunakan frontend-only architecture; sistem membutuhkan backend lokal dan worker process.
- Session/photo/upload state machine adalah inti implementation.
- Hindari rule engine kompleks untuk output naming pada MVP; gunakan struktur folder deterministik.
- Startup recovery harus tersedia untuk session/photo/upload job yang belum selesai.
- Dashboard harus menampilkan actionable state, bukan raw technical logs saja.

## Project Scoping

### Strategy & Philosophy

**Approach:** Single-release operational MVP. Semua requirement yang ditetapkan sebagai MVP tetap masuk scope rilis pertama. PRD tidak memindahkan requirement user ke fase post-MVP tanpa persetujuan eksplisit.

**MVP Philosophy:** Operational reliability MVP. Produk viable hanya jika workflow capture → routing → processing → local storage → session completion → post-session Drive upload berjalan end-to-end untuk 3 camera station.

**Resource Requirements:**

- Full-stack developer untuk local web dashboard dan backend lokal.
- Engineer dengan pengalaman file system watcher/background workers.
- Engineer atau library support untuk image/LUT processing.
- Integrator untuk Google Drive API.
- QA/operator tester untuk simulasi event dan hardware-adjacent testing.

### Complete Feature Set

**Core User Journeys Supported:**

- Operator menjalankan 3 camera station paralel saat event.
- Operator menangani timer habis dan late photo.
- Admin melakukan setup camera station dan readiness check sebelum event.
- Admin menyelesaikan session dan upload hasil ke Google Drive.
- Admin/support melakukan troubleshooting saat processing/upload gagal.

**Must-Have Capabilities:**

- Local-network web dashboard di PC admin.
- Camera Management untuk 3 Sony A6000.
- Setting Camera lengkap: slot name, physical device, identifier, input folder, background name, default LUT, output folder rule, connection status, test capture/test watch, reconnect/refresh.
- 3 camera station cards.
- Create session per station dengan customer name, order number, dan timer.
- One active session per station.
- Folder watcher per camera input folder.
- Stable file detection sebelum processing.
- JPG-only ingestion.
- Metadata routing berdasarkan station dan active session.
- Session/photo/upload state machine.
- Timer-based session lock dan End Session dari UI.
- Late photo quarantine/unassigned.
- Manual move from quarantine.
- Original-first local save.
- LUT processing per station/background.
- Save original + graded locally.
- Processing status, auto retry 3x, dan manual retry.
- Session summary.
- Open local result folder.
- Google Drive upload setelah local session completion.
- Google Drive folder otomatis berdasarkan customer + order.
- Upload original + graded.
- Retry failed Drive upload.
- Event readiness checklist.
- Disk space warning.
- App health indicator.
- Operator activity log.
- Config backup/restore.
- Duplicate filename protection.
- Missing LUT fallback.
- Local save vs Drive upload status terpisah.
- Startup recovery untuk pending session/photo/upload job.

**Nice-to-Have Capabilities:**

- UI polish lanjutan.
- Advanced filtering/search di logs.
- Bulk quarantine assignment.
- Export manifest tambahan selain metadata internal.
- Tablet-optimized dashboard layout.

### Risk Mitigation Strategy

**Technical Risks:**

- Risiko file partial/write race dimitigasi dengan stable file detection, periodic scanner, dan ingest idempotency.
- Risiko salah routing dimitigasi dengan station config snapshot, active session rule, timer lock, dan quarantine.
- Risiko data inconsistency dimitigasi dengan SQLite, state machine eksplisit, temp file + atomic rename, dan transactional metadata update.
- Risiko Google Drive duplicate/failure dimitigasi dengan upload queue, local asset mapping, retry, dan status per file/session.

**Market/Operational Risks:**

- Risiko operator bingung dimitigasi dengan primary station state, action-specific buttons, readiness checklist, dan operator-friendly alerts.
- Risiko workflow tidak cocok dimitigasi dengan simulasi event 3 station sebelum pilot event nyata.
- Risiko quarantine menjadi backlog dimitigasi dengan visible quarantine count dan manual assignment flow.

**Resource Risks:**

- Jika resource terbatas, implementation tetap mengikuti urutan: local foundation → session engine → processing pipeline → dashboard UX → Google Drive.
- Semua capability tetap bagian single-release scope; urutan hanya sequencing implementation, bukan de-scope.

## Functional Requirements

### Camera Station Management

- FR1: Admin can create and manage three logical camera stations.
- FR2: Admin can assign a physical camera/device identifier to each camera station.
- FR3: Admin can assign a unique input folder to each camera station.
- FR4: Admin can assign a background name to each camera station.
- FR5: Admin can assign a default LUT to each camera station.
- FR6: Admin can configure deterministic output folder rules for session results.
- FR7: Admin can view connection/readiness status for each camera station.
- FR8: Admin can run test capture/test watch validation for each camera station.
- FR9: Admin can refresh/reconnect/recheck a camera station when it has a device or folder issue.
- FR10: System prevents invalid station configuration such as duplicate input folders or missing required paths.
- FR11: System can backup and restore camera station configuration.

### Session Management

- FR12: Admin/operator can create a session for a specific camera station.
- FR13: Admin/operator can enter customer name, order number, and timer duration when creating a session.
- FR14: System allows only one active session per camera station.
- FR15: System tracks session state across active, ending, locked, local complete, upload pending, uploaded, and failed states.
- FR16: Operator can end a session manually from the admin UI.
- FR17: System can end a session automatically when the timer expires.
- FR18: System locks a session after it ends so new photos no longer enter that session.
- FR19: System records session summary including photo counts, failures, quarantine count, duration, local output path, and Drive upload status.
- FR20: System can recover session state after application restart.

### Photo Ingestion and Routing

- FR21: System can monitor each camera station input folder for new JPG files.
- FR22: System can identify which camera station produced a detected JPG.
- FR23: System can wait until a detected JPG is stable before processing it.
- FR24: System can route a detected JPG to the active session for its camera station.
- FR25: System sends JPGs with no eligible active session to quarantine/unassigned.
- FR26: System sends JPGs detected after session lock to quarantine/unassigned.
- FR27: System records a quarantine reason for each quarantined photo.
- FR28: Admin/operator can review quarantined photos.
- FR29: Admin/operator can manually assign a quarantined photo to an appropriate session.
- FR30: System prevents duplicate processing of the same detected JPG.

### Image Processing and Local Storage

- FR31: System can save the original JPG for each valid session photo.
- FR32: System can apply the station/session LUT to create a graded JPG.
- FR33: System can save original and graded JPGs to deterministic customer/order/station folders.
- FR34: System can preserve the original JPG even when LUT processing fails.
- FR35: System can track processing status for each photo.
- FR36: System can automatically retry failed photo processing up to three times.
- FR37: Admin/operator can manually retry failed photo processing.
- FR38: System can prevent filename collisions in local output folders.
- FR39: System can identify missing or invalid LUT conditions and mark affected processing jobs as failed/retryable.

### Dashboard and Operator Controls

- FR40: Operator can view a dashboard with three camera station cards.
- FR41: Operator can see the active customer, order number, timer, station status, LUT, photo count, and last photo preview for each station.
- FR42: Operator can see primary station states such as READY, LIVE, ATTENTION, LOCKED, and UPLOAD status.
- FR43: Operator can see actionable alerts for camera, folder, disk, processing, quarantine, and upload problems.
- FR44: Operator can access station/session detail views for deeper troubleshooting.
- FR45: Operator can open the local result folder for a completed session.
- FR46: Operator can view local save status separately from Google Drive upload status.
- FR47: Operator can view processing queue and upload queue status.
- FR48: Operator can view activity logs filtered by station/session.

### Readiness, Health, and Recovery

- FR49: System can run an event readiness checklist before sessions start, covering input folders readable, output folders writable, LUT files present/valid, disk threshold met, watcher running, processor ready, application data/state health, quarantine folder writable, and Drive status known.
- FR50: System can block session start when required readiness checks fail.
- FR51: System can show disk space warnings and prevent unsafe operation when storage falls below configurable warning/block thresholds.
- FR52: System can show application health indicators for watcher, processor, application data/state health, disk, and Drive connectivity.
- FR53: System can record operator actions such as start session, end session, reconnect, retry, config change, and Drive upload.
- FR54: System can recover pending photo processing jobs after application restart.
- FR55: System can recover pending Google Drive upload jobs after application restart.

### Google Drive Fulfillment

- FR56: Admin can connect one Google Drive admin account for cloud upload.
- FR57: System can create Google Drive folders based on customer name and order number.
- FR58: System can upload original and graded JPGs to Google Drive after local session completion.
- FR59: System can track Google Drive upload status per session and per file.
- FR60: System can retry failed Google Drive uploads.
- FR61: System preserves local files when Google Drive upload fails.
- FR62: System ensures Google Drive upload does not block new capture sessions.
- FR63: System can prevent duplicate Drive uploads for the same session asset during retry or restart using tracked upload identity and status.

## Non-Functional Requirements

### Performance

- NFR1: Dashboard station status should update near real-time enough for event operation.
- NFR2: New photos should appear in the relevant station flow after file stabilization and processing without blocking other stations.
- NFR3: Image processing must not block dashboard interaction or session controls during normal event operation.
- NFR4: Google Drive upload must not degrade capture, local save, session routing, or LUT processing performance.
- NFR5: System must support 3 active camera stations during at least a 2-hour simulation with at least 300 total JPG files.
- NFR6: Folder watcher must handle duplicate file system events without duplicate processing.

### Reliability

- NFR7: The system must never treat a partial/incomplete JPG as successfully processed.
- NFR8: The system must preserve original JPG before attempting graded/LUT output.
- NFR9: The system must maintain recoverable session, photo, processing, quarantine, and upload state after application restart.
- NFR10: Timer end, admin end, and photo ingestion boundary cases must resolve deterministically.
- NFR11: Local save success must be independent of Google Drive upload success.
- NFR12: System must surface failed watcher, failed processing, missing LUT, low disk, missing folder, and Drive upload failure states to the operator.
- NFR13: System must prevent invalid readiness state from being shown as READY.

### Data Integrity

- NFR14: Each photo record must maintain complete traceability from source folder to session, local original, graded output, quarantine state if any, and Drive upload status.
- NFR15: Local output filenames must be collision-safe.
- NFR16: Processing and upload retries must be duplicate-safe.
- NFR17: Session config must be snapshotted so later config changes do not alter historical routing interpretation.
- NFR18: Persisted application records must not point to missing final local files after save, processing, retry, or restart scenarios.

### Security and Privacy

- NFR19: Dashboard access should be limited to the local operational network for MVP.
- NFR20: Google Drive authentication must use an authorized admin account.
- NFR21: Stored customer names, order numbers, and photo files must be treated as customer data.
- NFR22: The system should avoid exposing dashboard access to public internet in MVP.
- NFR23: Activity logs should not expose unnecessary sensitive data beyond operational troubleshooting needs.

### Integration

- NFR24: Camera integration depends on reliable JPG output to configured input folders.
- NFR25: Google Drive integration must support token failure, network failure, partial upload, and retry scenarios.
- NFR26: Google Drive upload must track remote folder/file status enough to reduce duplicate uploads.
- NFR27: LUT processing must tolerate missing/invalid LUT by failing the graded job while preserving original.

### Usability and Accessibility

- NFR28: Station status must be understandable by operators under event pressure.
- NFR29: Critical states must use text labels in addition to color.
- NFR30: Critical actions such as End Session must require confirmation.
- NFR31: Error actions must be specific, such as Retry Photo Processing, Retry Drive Upload, Restart Folder Watcher, or Recheck Camera.
- NFR32: Dashboard must clearly separate local save status from Google Drive upload status.

### Operational Constraints

- NFR33: The admin PC should remain awake and connected during event operation.
- NFR34: The system should show the local network URL needed to access the dashboard from other devices.
- NFR35: Disk space must be checked before allowing new sessions when below safe thresholds.
- NFR36: Logs should be timestamped and filterable by station/session for troubleshooting.

## Implementation Sequencing

Sequencing ini bukan phasing scope; semua capability tetap bagian single-release MVP.

### Milestone 1 — Local Foundation

- Camera setting data model.
- Input folder per camera.
- Folder watcher.
- Stable JPG detection.
- Duplicate filename protection.
- Metadata persistence.
- Local output folder rule.

### Milestone 2 — Session Engine

- Create session per camera station.
- Customer name + order number + timer.
- One active session per station.
- Timer-based session lock.
- Late photo quarantine.
- Manual move from quarantine.
- End session via timer/UI.

### Milestone 3 — Processing Pipeline

- Save original.
- Apply LUT.
- Save graded.
- Processing status.
- Auto retry 3x.
- Manual retry.
- Missing LUT fallback.
- Processing queue visibility.

### Milestone 4 — Dashboard UX

- 3 station cards.
- Last photo preview.
- Timer/status/count.
- Reconnect/refresh.
- Event readiness checklist.
- Session summary.
- Open result folder.
- Disk space warning.
- App health indicator.
- Operator activity log.

### Milestone 5 — Google Drive

- Google admin account integration.
- Create Drive folder by customer + order.
- Upload original + graded after session.
- Upload status.
- Retry failed upload.
- Cloud sync summary.

## Brainstorming Reconciliation

The brainstorming input was reconciled against this PRD. Accepted ideas were incorporated into product scope, journeys, requirements, NFRs, or implementation sequencing. Rejected reverse-brainstorming ideas were intentionally excluded. No accepted MVP core or hardening idea from the brainstorming session was silently dropped.
