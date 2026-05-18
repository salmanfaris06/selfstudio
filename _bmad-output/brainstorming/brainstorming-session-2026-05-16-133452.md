---
stepsCompleted: [1, 2, 3, 4]
session_active: false
workflow_completed: true
inputDocuments: []
session_topic: 'Aplikasi pusat untuk menghubungkan satu sistem dengan banyak device kamera, mendeteksi kamera yang tersedia, dan menerima data dari masing-masing kamera.'
session_goals: 'Merancang ide produk/MVP awal yang realistis untuk aplikasi multi-kamera, termasuk fitur inti, alur penggunaan, dan kemungkinan arah teknis tingkat tinggi.'
selected_approach: 'AI-Recommended Techniques'
techniques_used: ['Question Storming', 'SCAMPER Method', 'Resource Constraints']
ideas_generated: [108]
context_file: ''
---

# Brainstorming Session Results

**Facilitator:** alpharize
**Date:** 2026-05-16

## Session Overview

**Topic:** Aplikasi pusat untuk menghubungkan satu sistem dengan banyak device kamera, mendeteksi kamera yang tersedia, dan menerima data dari masing-masing kamera.

**Goals:** Merancang ide produk/MVP awal yang realistis untuk aplikasi multi-kamera, termasuk fitur inti, alur penggunaan, dan kemungkinan arah teknis tingkat tinggi.

### Context Guidance

_Tidak ada context file tambahan yang diberikan._

### Session Setup

Sesi ini akan berfokus pada bentuk MVP produk: fitur apa yang wajib ada di versi awal, bagaimana pengguna menambahkan/mendeteksi kamera, bagaimana aplikasi membedakan data dari tiap kamera, serta use case awal yang paling masuk akal untuk dibangun terlebih dahulu.

## Technique Selection

**Approach:** AI-Recommended Techniques
**Analysis Context:** Aplikasi pusat multi-kamera dengan fokus pada rancangan produk/MVP awal yang realistis.

**Recommended Techniques:**

- **Question Storming:** Direkomendasikan untuk memperjelas pertanyaan produk dan teknis sebelum menentukan scope MVP.
- **SCAMPER Method:** Digunakan untuk menghasilkan ide fitur secara sistematis dari berbagai sudut: substitute, combine, adapt, modify, put to other uses, eliminate, reverse.
- **Resource Constraints:** Digunakan untuk mengubah kumpulan ide menjadi MVP ramping yang realistis dibangun dengan resource terbatas.

**AI Rationale:** Karena ruang produk masih cukup luas — jenis kamera, sumber data, protokol, pengguna, dan use case belum dikunci — sesi dimulai dari eksplorasi pertanyaan, dilanjutkan dengan generasi fitur terstruktur, lalu diakhiri dengan pembatasan scope agar menghasilkan MVP yang bisa dieksekusi.

## Technique Execution Results

### Question Storming - Elemen 1: Kamera dan Data Dasar

**User Inputs Captured:**

- Kamera target awal: Sony A6000
- Deteksi kamera: otomatis
- Data yang diterima: gambar/foto hasil tangkapan kamera, bukan video/rekaman
- Mode data: real-time
- Arah data: kamera mengirim data ke aplikasi
- Identitas kamera: perlu camera_id/lokasi/nama/status
- Target MVP: 3 kamera
- Live preview: tidak wajib
- Recovery disconnect: manual di kamera, lalu tombol refresh/reconnect di sistem
- Storage: simpan gambar/foto, tidak menyimpan video

### Question Storming - Elemen 2: Koneksi USB dan Transfer Gambar

**User Inputs Captured:**

- Koneksi kamera: USB
- Topologi: 3 kamera Sony A6000 terhubung ke 1 PC yang sama
- Trigger capture: customer melakukan take manual melalui kamera dengan remote trigger shutter
- Transfer file: setelah capture, gambar otomatis ditransfer ke aplikasi
- Kamera mendukung remote control/tethered shooting
- Concurrent transfer: 3 kamera bisa mengirim gambar hampir bersamaan
- Folder routing: masing-masing kamera mengirim data gambar ke folder yang telah ditentukan
- Sinkronisasi urutan: tidak wajib, karena 1 kamera merepresentasikan 1 background/event/sesi
- Dashboard: foto perlu langsung muncul setelah diterima
- Format: perlu keputusan JPG vs RAW untuk kebutuhan color grading LUT
- Penggunaan data: hanya disimpan, belum untuk AI/computer vision

### Question Storming - Elemen 3: Dashboard MVP dan Session/Event

**User Inputs Captured:**

- Admin membuat session/event terlebih dahulu
- Satu kamera selalu terkait satu background tetap
- Foto otomatis masuk berdasarkan customer/session, kamera, background, tanggal, dan struktur output terkait
- Semua foto langsung disimpan, tidak perlu kurasi/manual selection untuk MVP
- Nama session menggunakan nama customer
- Setiap background/kamera punya LUT berbeda
- User perlu bisa mengganti LUT dari dashboard
- Sistem perlu status proses: received → grading → saved → ready
- Foto hasil grading perlu bisa di-download/export setelah sesi/event selesai
- Print/share belum dibutuhkan untuk MVP

### Question Storming - Elemen 4: Alur Session, Multi-Customer, Timer, dan Akses Browser

**User Inputs Captured:**

- Pada Create Session, admin harus mengisi timer sesi/event
- Sistem bisa menjalankan beberapa customer/session
- Identitas session: nama customer + nomor order
- Folder output dibuat otomatis oleh aplikasi
- Original dan graded image disimpan dua-duanya
- LUT tidak bisa diubah ketika session/event sedang berlangsung
- Jika processing gagal, sistem melakukan auto retry 3x terlebih dahulu
- Setelah auto retry gagal, sistem menyediakan tombol retry grading manual
- Export cukup dengan membuka folder hasil, tidak perlu ZIP untuk MVP
- Aplikasi bisa diakses dari device lain lewat browser, berjalan di PC lokal/jaringan lokal

### Question Storming - Elemen 5: Routing Foto dan Camera Management

**User Inputs Captured:**

- Sistem mengetahui foto baru berdasarkan nama kamera/device camera yang mengirim/menghasilkan file
- Dashboard perlu menu **Setting Camera** untuk camera management
- Di Setting Camera, admin bisa menentukan Camera 1 menggunakan device kamera apa, Camera 2 menggunakan device kamera apa, dan seterusnya
- Model routing MVP menggunakan **Opsi A: satu active customer/session per background/kamera**
- Artinya setiap kamera/background bisa melayani customer berbeda pada waktu yang sama

### SCAMPER - Substitute

**Ideas Captured:**

**[MVP #28]: Substitute Direct Camera Control With Folder Watching**
_Concept_: Daripada aplikasi langsung mengontrol kamera Sony A6000 secara penuh, MVP cukup memonitor input folder yang sudah ditentukan untuk tiap kamera. Kamera/tethering tool bertugas menyimpan foto ke folder, aplikasi bertugas memproses foto setelah masuk.
_Novelty_: Mengganti integrasi kamera kompleks dengan mekanisme file-system event yang lebih cepat dibangun.

**[MVP #29]: Substitute Live Preview With Last Captured Preview**
_Concept_: Alih-alih menampilkan live feed dari kamera, dashboard hanya menampilkan foto terakhir dari tiap kamera/background.
_Novelty_: Mengurangi beban teknis besar karena produk ini memang fokus pada hasil foto, bukan video monitoring.

**[MVP #30]: Substitute Auto Device Recovery With Manual Reconnect**
_Concept_: Daripada membangun sistem auto-healing kompleks untuk disconnect kamera, MVP menyediakan tombol refresh/reconnect setelah operator memperbaiki kamera secara manual.
_Novelty_: Memindahkan recovery fisik ke operator, sementara sistem hanya mengelola rescan dan reconnect state.

**User Response:** Tidak ada substitute tambahan dari user pada tahap ini.

### SCAMPER - Combine

**Ideas Captured:**

**[UX #31]: Combine Camera Status, Active Session, and Last Photo in One Station Card**
_Concept_: Setiap kamera/background ditampilkan sebagai satu card besar yang berisi status koneksi, customer aktif, timer, LUT aktif, foto terakhir, jumlah foto, dan tombol reconnect.
_Novelty_: Admin tidak perlu pindah-pindah halaman untuk melihat kondisi satu kamera; semua konteks penting ada di satu station card.

**[Workflow #32]: Combine Create Session With Assign Camera**
_Concept_: Saat admin membuat session customer, admin langsung memilih camera/background mana yang akan digunakan. Jadi session tidak dibuat terpisah lalu di-assign belakangan.
_Novelty_: Mengurangi langkah operasional dan mencegah session aktif tanpa kamera.

**[Processing #33]: Combine Save Original, Apply LUT, and Save Graded Into One Pipeline**
_Concept_: Begitu foto masuk, sistem menjalankan pipeline otomatis: detect file → copy original → apply LUT → save graded → update dashboard.
_Novelty_: Proses editing tidak menjadi tahap manual terpisah; semua terjadi sebagai satu alur ingestion.

**[Admin #34]: Combine Camera Test and Folder Watch Test**
_Concept_: Tombol test di Setting Camera tidak hanya mengecek device terhubung, tapi juga memastikan folder input bisa dimonitor dan file baru bisa dideteksi.
_Novelty_: Validasi kamera dan validasi folder digabung sebagai “station readiness check”.

**[Export #35]: Combine Session Completion Trigger With Open Output Folder**
_Concept_: Session bisa selesai karena timer habis atau karena admin menekan tombol End Session. Setelah session selesai, sistem menyelesaikan proses pending lalu menampilkan tombol “Open Result Folder”.
_Novelty_: Completion flow mendukung otomatisasi berbasis timer dan kontrol manual admin, sementara export tetap ringan melalui folder hasil.

**[Session #36]: Dual End-Session Control**
_Concept_: Setiap session punya countdown timer dan tombol End Session. Jika timer habis, session otomatis masuk completed; jika admin klik End Session, sistem meminta konfirmasi lalu menyelesaikan session lebih awal.
_Novelty_: Menggabungkan disiplin waktu dengan fleksibilitas operator.

### SCAMPER - Adapt

**Ideas Captured:**

**[Adapt #37]: POS Shift Model for Session Lifecycle**
_Concept_: Adaptasi dari sistem kasir/POS: session seperti “shift transaksi”. Admin membuka session, sistem mencatat semua aktivitas selama session, lalu session ditutup dan menghasilkan folder hasil.
_Novelty_: Membuat konsep session lebih familiar dan audit-friendly.

**[Adapt #38]: Print Queue Model for Image Processing**
_Concept_: Adaptasi dari printer queue: setiap foto yang masuk menjadi job dengan status queued, processing, done, atau failed. Admin bisa melihat job yang gagal dan menjalankan retry.
_Novelty_: Processing gambar menjadi transparan seperti antrean print, bukan proses tersembunyi.

**[Adapt #39]: Airport Gate Model for Camera Stations**
_Concept_: Tiap kamera/background diperlakukan seperti gate bandara. Ada customer aktif, status gate, waktu tersisa, dan aktivitas terakhir. Customer berikutnya bisa menunggu di queue.
_Novelty_: Membuat multi-customer per background lebih mudah dipahami secara visual.

**[Adapt #40]: Download Manager Model for File Arrival**
_Concept_: Foto yang masuk divisualkan seperti download manager: filename, progress/status, source camera, retry count, dan destination folder.
_Novelty_: Admin dapat memahami transfer/proses foto secara real-time tanpa istilah teknis berat.

**[Adapt #41]: Studio Preset Model for LUT and Background**
_Concept_: Background + LUT + output rule dikemas sebagai preset studio. Misalnya “Red Background Preset” otomatis punya LUT merah, folder rule, dan naming rule.
_Novelty_: Mengurangi konfigurasi berulang; admin memilih preset daripada mengatur banyak field satu-satu.

**User Response:** Semua pola adaptasi dipilih sebagai inspirasi untuk MVP.

**[Product #42]: Studio Station Operating System**
_Concept_: Aplikasi bertindak seperti sistem operasi mini untuk studio/event multi-background. Setiap camera station punya customer aktif, timer, preset background, job foto, status proses, dan folder hasil.
_Novelty_: Produk bukan hanya file watcher, tapi command center operasional untuk sesi foto paralel.

### SCAMPER - Modify

**Ideas Captured:**

**[Reliability #43]: Enlarge Visibility of Errors**
_Concept_: Error penting seperti kamera disconnect, folder tidak bisa diakses, LUT missing, atau grading gagal harus muncul besar di station card, bukan hanya log kecil.
_Novelty_: Admin lapangan bisa cepat bertindak tanpa membuka halaman teknis.

**[UX #44]: Minimize Admin Typing During Event**
_Concept_: Saat event berlangsung, admin sebaiknya hanya mengetik customer name, order number, dan timer. Field lain seperti camera, background, LUT, dan folder otomatis dari preset.
_Novelty_: Mengurangi salah input saat kondisi ramai.

**[Processing #45]: Modify Retry Into Visible Retry Counter**
_Concept_: Auto retry 3x harus terlihat, misalnya `Retry 1/3`, `Retry 2/3`, `Retry 3/3`, lalu `Needs Manual Retry`.
_Novelty_: Operator memahami sistem sedang mencoba memperbaiki masalah, bukan freeze.

**[Session #46]: Enlarge Timer Importance**
_Concept_: Timer session harus terlihat jelas di station card, dengan warna berubah saat hampir habis: normal, warning, expired.
_Novelty_: Timer menjadi alat operasional utama, bukan detail kecil.

**[Storage #47]: Modify Folder Structure for Human Readability**
_Concept_: Output folder dibuat sangat mudah dibaca: `OrderNumber_CustomerName/Camera1_BackgroundName/original` dan `/graded`.
_Novelty_: Admin/customer bisa memahami hasil tanpa membuka aplikasi.

**User Concern:** Ketika timer sesi habis, customer tidak boleh mengambil foto lagi atau kamera/session harus berhenti menerima foto baru agar tidak ada foto baru yang masuk setelah sesi habis.

**[Session #48]: Timer-Based Capture Lock**
_Concept_: Saat timer session habis, sistem mengunci session sehingga foto baru dari kamera tersebut tidak lagi dimasukkan ke customer/order tersebut. Station berubah dari active menjadi closed/completed.
_Novelty_: Timer bukan sekadar informasi, tapi kontrol operasional yang mencegah foto masuk ke sesi yang salah.

**[Data #49]: Late Photo Quarantine**
_Concept_: Jika ada file foto baru masuk setelah session selesai, sistem tidak langsung membuangnya. Foto dimasukkan ke folder/status “late/unassigned” agar admin bisa mengecek apakah perlu dipindahkan manual atau diabaikan.
_Novelty_: Mencegah kehilangan data sambil tetap menjaga aturan session tidak menerima foto baru.

**[UX #50]: Expired Session Visual Lock**
_Concept_: Station card berubah jelas saat timer habis: tombol capture/session inactive, warna abu-abu/merah, label “Session Ended”, dan tidak menerima foto baru.
_Novelty_: Admin dan operator langsung tahu bahwa background/kamera tersebut sudah tidak aktif untuk customer itu.

**[Operations #51]: Camera Stop Instruction After Timer**
_Concept_: Saat timer hampir habis atau habis, dashboard memberi instruksi eksplisit: “Stop taking photos for this customer.” Jika memungkinkan, sistem juga bisa mencoba memutus mode intake dari folder watcher untuk session aktif.
_Novelty_: Menggabungkan kontrol sistem dengan arahan manusia di lapangan.

**Decision:** User memilih opsi B — late photos masuk quarantine/unassigned, bukan di-ignore dan bukan otomatis masuk session berikutnya.

### SCAMPER - Put to Other Uses

**Ideas Captured:**

**[Analytics #52]: Session Productivity Summary**
_Concept_: Setelah session selesai, sistem menampilkan ringkasan sederhana: jumlah foto diterima, jumlah foto berhasil digrading, jumlah gagal, jumlah late photos, dan durasi sesi.
_Novelty_: Data operasional yang sudah ada diubah menjadi laporan kualitas sesi tanpa fitur analytics besar.

**[Audit #53]: Photo Processing Log**
_Concept_: Setiap foto punya log kecil: waktu diterima, kamera asal, LUT yang dipakai, retry count, status akhir, dan output path.
_Novelty_: Jika ada komplain hasil/foto hilang, admin bisa melacak prosesnya.

**[Training #54]: Event Readiness Checklist**
_Concept_: Setting camera dan test watch bisa dipakai sebagai checklist sebelum event: semua kamera connected, folder valid, LUT tersedia, output folder siap.
_Novelty_: Fitur teknis berubah menjadi alat SOP operasional.

**[Support #55]: Error Evidence Bundle**
_Concept_: Jika terjadi error, sistem bisa menyimpan informasi dasar seperti foto gagal, error message, retry count, dan kamera asal.
_Novelty_: Memudahkan debugging tanpa meminta operator menjelaskan masalah teknis secara manual.

**[Business #56]: Customer Delivery Folder**
_Concept_: Folder hasil yang rapi bisa langsung menjadi folder delivery ke customer/order, bukan hanya storage internal.
_Novelty_: Struktur teknis output sekaligus menjadi produk akhir yang siap diberikan.

**Decision:** User memilih semua ide Put to Other Uses masuk MVP.

### SCAMPER - Eliminate

**Ideas Proposed / Revised:**

**[Scope #57]: Eliminate Live Video Streaming**
_Concept_: MVP tidak menampilkan live feed/video dari kamera. Dashboard hanya menampilkan foto terakhir dan status kamera/session.
_Novelty_: Menghindari kompleksitas streaming, latency, bandwidth, dan integrasi kamera yang tidak diperlukan.

**[Scope #58]: Eliminate RAW Format Entirely**
_Concept_: MVP tidak menggunakan RAW sama sekali. Sistem hanya menerima, memproses, menyimpan, dan mengelola JPG.
_Novelty_: Menghapus beban format berat dan menjaga pipeline cepat untuk event real-time.

**[Storage #59]: Local + Google Drive Cloud Storage**
_Concept_: Penyimpanan foto MVP memiliki dua tujuan: lokal di komputer admin dan cloud menggunakan Google Drive. Local storage menjadi primary output, Google Drive menjadi backup/sinkronisasi/delivery cloud.
_Novelty_: MVP tetap operasional lokal, tetapi hasil tetap punya salinan cloud untuk keamanan dan akses lanjutan.

**[Scope #60]: Eliminate Print/Share Feature**
_Concept_: MVP belum punya fitur print langsung, WhatsApp share, email, QR download, atau social sharing. Output cukup folder hasil dan Google Drive sync.
_Novelty_: Fokus pada capture-processing-storage dulu, bukan distribusi eksternal kompleks.

**[Scope #61]: Eliminate Complex User Roles**
_Concept_: MVP cukup punya admin/operator sederhana. Tidak perlu role granular seperti owner, editor, viewer, technician.
_Novelty_: Mengurangi kompleksitas permission.

**[Scope #62]: Eliminate AI/Computer Vision**
_Concept_: MVP tidak melakukan face detection, object detection, quality scoring, background validation, atau auto-selection foto.
_Novelty_: Produk fokus pada workflow foto multi-kamera, bukan analitik gambar.

**[Scope #63]: Eliminate Advanced Customer Queue**
_Concept_: MVP tidak perlu sistem antrean kompleks lintas background. Cukup satu active session per camera station.
_Novelty_: Model operasional tetap jelas tanpa queue management besar.

**[Scope #64]: Eliminate Auto Camera Shutdown**
_Concept_: Saat timer habis, aplikasi mengunci session dan late photos masuk quarantine. MVP tidak perlu benar-benar mematikan kamera fisik.
_Novelty_: Mengontrol data intake tanpa memaksa kontrol hardware yang rumit.

**Decision:** RAW dihapus sepenuhnya. Cloud sync tidak dieliminasi; Google Drive masuk MVP sebagai penyimpanan kedua selain lokal.

**[Storage #65]: Dual Destination Save Status**
_Concept_: Setiap foto punya dua status penyimpanan: local save dan Google Drive sync. Contoh: `Local: saved`, `Drive: syncing/synced/failed`.
_Novelty_: Admin tahu apakah foto sudah aman di lokal saja atau sudah berhasil naik ke cloud.

**[Cloud #66]: Background-Aware Google Drive Folder Structure**
_Concept_: Struktur folder di Google Drive mengikuti struktur lokal: customer/order → camera/background → original/graded.
_Novelty_: File cloud tetap rapi dan konsisten dengan hasil di komputer admin.

**[Reliability #67]: Cloud Sync Retry Queue**
_Concept_: Jika upload Google Drive gagal karena internet bermasalah, foto masuk queue dan sistem mencoba upload ulang nanti. Local file tetap dianggap aman.
_Novelty_: Event tetap berjalan walaupun internet tidak stabil.

**[UX #68]: Session Cloud Sync Summary**
_Concept_: Setelah session selesai, dashboard menampilkan ringkasan: total foto, local saved, Drive synced, Drive pending, Drive failed.
_Novelty_: Admin bisa tahu apakah folder customer sudah lengkap di cloud sebelum sesi dianggap benar-benar selesai secara delivery.

**Google Drive Decisions:**

- Google Drive menggunakan akun Google admin yang sama untuk semua event
- Upload ke Google Drive dilakukan setelah session selesai, bukan real-time per foto
- Jika internet mati, sistem menyimpan lokal dulu dan sync otomatis/diulang ketika koneksi tersedia
- Folder Google Drive dibuat otomatis berdasarkan customer + nomor order
- File yang di-upload: original + graded

**[Cloud #69]: Post-Session Drive Upload Button**
_Concept_: Setelah session selesai, dashboard menampilkan tombol “Upload to Google Drive” atau menjalankan upload otomatis dengan status progress. Jika gagal, admin bisa retry upload dari session detail.
_Novelty_: Cloud sync ditempatkan setelah pekerjaan capture selesai sehingga tidak mengganggu sesi foto real-time.

### SCAMPER - Reverse

**Ideas Proposed but Rejected:**

**[Reverse #70]: Session Owns Camera Instead of Camera Owns Session**
_Concept_: Session/customer “meminjam” satu camera station selama timer berjalan. Setelah timer habis, station otomatis bebas lagi.
_Novelty_: Membuat status kamera seperti resource booking.

**[Reverse #71]: Configure Presets Before Cameras**
_Concept_: Admin membuat preset background dulu, lalu kamera dipasangkan ke preset.
_Novelty_: Workflow lebih dekat dengan cara bisnis berpikir: background dulu, hardware belakangan.

**[Reverse #72]: Treat Failed Processing as First-Class Inbox**
_Concept_: Semua kegagalan masuk halaman khusus “Needs Attention” yang bisa diproses ulang.
_Novelty_: Error handling menjadi bagian produk, bukan sekadar log teknis.

**[Reverse #73]: Local Is Source of Truth, Drive Is Mirror**
_Concept_: Local storage menjadi source of truth, Google Drive mirror/delivery setelah session selesai.
_Novelty_: Aplikasi tetap tangguh saat internet buruk.

**[Reverse #74]: Late Photos Are Not Errors, They Are Unassigned Assets**
_Concept_: Foto yang masuk setelah timer habis dianggap asset unassigned, bukan error.
_Novelty_: Menghindari kehilangan foto sambil menjaga integritas session.

**[Reverse #75]: Dashboard Starts From Stations, Not Sessions**
_Concept_: Halaman utama dimulai dari 3 camera station aktif, bukan daftar session.
_Novelty_: Interface mengikuti cara kerja lapangan.

**Decision:** User tidak menyetujui ide Reverse yang diusulkan. Reverse ideas tidak dimasukkan sebagai keputusan MVP.

### Resource Constraints - MVP Final Scope

**Constraint:** MVP harus tetap realistis, tetapi user memutuskan semua core features berikut harus masuk MVP.

**MVP Core — Wajib Ada:**

**[MVP #76]: Admin Web Dashboard via Local Network**
_Concept_: Aplikasi berjalan di PC utama yang terhubung ke kamera via USB, tapi dashboard bisa diakses dari device lain melalui browser di jaringan lokal.
_Novelty_: Admin tidak harus selalu berada di PC kamera.

**[MVP #77]: Camera Management Settings**
_Concept_: Menu setting untuk 3 kamera: slot name, physical device, identifier, input folder, background name, default LUT, output rule, connection status, test watch, reconnect/refresh.
_Novelty_: Sistem punya kontrol jelas atas kamera fisik dan station operasional.

**[MVP #78]: Three Camera Station Dashboard**
_Concept_: Dashboard menampilkan 3 station/card kamera dengan active customer, order number, timer, status kamera, LUT aktif, last photo, photo count, dan tombol reconnect.
_Novelty_: Operasional event bisa dipantau per background/kamera.

**[MVP #79]: Create Session Per Camera Station**
_Concept_: Admin membuat session untuk station tertentu dengan nama customer, nomor order, timer, dan pilihan kamera/background yang sudah dikonfigurasi.
_Novelty_: Mendukung satu active customer per kamera/background.

**[MVP #80]: Folder Watching Per Camera**
_Concept_: Sistem memonitor input folder tiap kamera untuk mendeteksi JPG baru.
_Novelty_: Menghindari direct camera SDK/control yang kompleks.

**[MVP #81]: JPG Ingestion and Metadata Routing**
_Concept_: Saat JPG baru masuk, sistem menentukan camera slot, active session, customer/order, background, timestamp, dan menyimpan metadata.
_Novelty_: Foto otomatis masuk ke konteks customer yang benar.

**[MVP #82]: LUT Processing Per Camera/Background**
_Concept_: Aplikasi menerapkan LUT sesuai kamera/background/session yang terkunci saat session aktif.
_Novelty_: Hasil graded otomatis tanpa proses editing manual.

**[MVP #83]: Save Original + Graded Locally**
_Concept_: Sistem menyimpan original JPG dan graded JPG ke folder lokal otomatis berdasarkan customer + order + kamera/background.
_Novelty_: Output rapi dan siap digunakan.

**[MVP #84]: Timer-Based Session Lock**
_Concept_: Saat timer habis atau admin klik End Session, session terkunci dan tidak menerima foto baru. Foto baru setelah session selesai masuk quarantine/unassigned.
_Novelty_: Mencegah foto masuk ke customer yang salah.

**[MVP #85]: Processing Status and Retry**
_Concept_: Foto memiliki status received, grading, saved, failed. Sistem auto retry 3x, lalu menyediakan retry manual.
_Novelty_: Pipeline lebih tahan error.

**[MVP #86]: Session Summary**
_Concept_: Setelah session selesai, sistem menampilkan total foto, processed, failed, late/unassigned, durasi, dan output folder.
_Novelty_: Admin mendapat laporan ringkas tanpa analytics kompleks.

**[MVP #87]: Open Result Folder**
_Concept_: Setelah session selesai, admin bisa membuka folder hasil lokal langsung dari UI.
_Novelty_: Export sederhana tanpa ZIP/print/share.

**[MVP #88]: Google Drive Upload After Session**
_Concept_: Setelah session selesai, sistem membuat folder Google Drive berdasarkan customer + order lalu upload original + graded. Jika gagal, upload bisa retry.
_Novelty_: Cloud storage masuk tanpa mengganggu real-time capture.

**[MVP #89]: Event Readiness Checklist**
_Concept_: Sebelum event, sistem mengecek kamera connected, folder valid, LUT tersedia, dan output path siap.
_Novelty_: Mengurangi risiko gagal saat customer sudah mulai.

**Decision:** User menyatakan semua 14 core features harus masuk MVP.

### Resource Constraints - Implementation Milestones

**Accepted Milestone Order:** User memilih opsi A — urutan milestone yang diusulkan cocok.

**Milestone 1 — Local Foundation**

- Camera setting data model
- Input folder per camera
- Folder watcher
- Detect JPG baru
- Save metadata
- Local output folder rule

**Milestone 2 — Session Engine**

- Create session per camera station
- Customer + order + timer
- One active session per station
- Timer-based lock
- Late photo quarantine

**Milestone 3 — Processing Pipeline**

- Save original
- Apply LUT
- Save graded
- Processing status
- Auto retry 3x
- Manual retry

**Milestone 4 — Dashboard UX**

- 3 station cards
- Last photo preview
- Timer/status/count
- Reconnect/refresh
- Session summary
- Open result folder
- Event readiness checklist

**Milestone 5 — Google Drive**

- Google admin account integration
- Create Drive folder
- Upload original + graded after session
- Upload status
- Retry failed upload

### Resource Constraints - Final Hardening Ideas

**[Hardening #98]: Config Backup and Restore**
_Concept_: Camera settings, folder paths, background names, and LUT mappings can be exported/imported as a config file.
_Novelty_: Jika PC perlu reinstall atau setting rusak, admin tidak perlu konfigurasi ulang dari nol.

**[Hardening #99]: Safe File Detection Delay**
_Concept_: Folder watcher menunggu file selesai ditulis sebelum memproses JPG, misalnya dengan size-stability check selama beberapa detik.
_Novelty_: Mencegah aplikasi memproses file yang belum selesai ditransfer dari kamera.

**[Hardening #100]: Duplicate Filename Protection**
_Concept_: Jika file dengan nama sama masuk, sistem tetap membuat nama output unik menggunakan timestamp/counter.
_Novelty_: Menghindari overwrite foto customer saat kamera melakukan reset numbering.

**[Hardening #101]: Disk Space Warning**
_Concept_: Dashboard memberi warning jika storage lokal hampir penuh sebelum atau saat event berjalan.
_Novelty_: Mencegah kegagalan event karena folder output tidak bisa menyimpan foto baru.

**[Hardening #102]: Missing LUT Fallback**
_Concept_: Jika LUT hilang/rusak, sistem menyimpan original dan menandai grading failed, bukan menghentikan seluruh session.
_Novelty_: Satu masalah LUT tidak membuat semua capture gagal.

**[Hardening #103]: Session Cannot Start Without Readiness Pass**
_Concept_: Session tidak bisa dimulai jika camera station belum lolos minimal readiness check: input folder valid, output folder siap, LUT tersedia.
_Novelty_: Menggeser error discovery dari saat customer foto ke sebelum sesi dimulai.

**[Hardening #104]: Manual Move From Quarantine**
_Concept_: Admin bisa memindahkan late/unassigned photo ke session tertentu jika memang foto tersebut valid.
_Novelty_: Quarantine tidak hanya tempat buangan, tapi recovery path untuk kasus operasional nyata.

**[Hardening #105]: Processing Queue Visibility**
_Concept_: Dashboard menampilkan jumlah foto yang sedang menunggu proses per station.
_Novelty_: Admin tahu apakah sistem sedang sibuk atau benar-benar macet.

**[Hardening #106]: Google Drive Upload Only After Local Completion**
_Concept_: Upload Drive hanya boleh dimulai setelah semua local processing untuk session selesai atau error final tercatat.
_Novelty_: Cloud tidak membawa state setengah matang dari session yang masih berjalan.

**[Hardening #107]: App Health Indicator**
_Concept_: Dashboard memiliki indikator health sederhana: folder watcher running, processor running, Drive sync available, disk OK.
_Novelty_: Admin mendapat gambaran kesehatan sistem tanpa membuka log teknis.

**[Hardening #108]: Operator Activity Log**
_Concept_: Tindakan penting seperti start session, end session, reconnect, retry, upload Drive, dan config change dicatat.
_Novelty_: Memudahkan audit ketika ada kebingungan di lapangan.

**Decision:** User memilih semua hardening #98–#108 masuk MVP.

**Final MVP Hardening Included:**

- Config Backup and Restore
- Safe File Detection Delay
- Duplicate Filename Protection
- Disk Space Warning
- Missing LUT Fallback
- Session Cannot Start Without Readiness Pass
- Manual Move From Quarantine
- Processing Queue Visibility
- Google Drive Upload Only After Local Completion
- App Health Indicator
- Operator Activity Log

## Idea Organization and Prioritization

**Session Achievement Summary:**

- **Total Ideas Generated:** 108 ideas/decisions/guardrails
- **Creative Techniques Used:** Question Storming, SCAMPER Method, Resource Constraints
- **Session Focus:** MVP aplikasi multi-kamera untuk 3 Sony A6000 via USB, menerima JPG real-time dari folder per kamera, memproses LUT, menyimpan original + graded, mengelola session/customer/order/timer, dan upload Google Drive setelah session selesai.

### Thematic Organization

#### Theme 1 — Core Product Concept

Produk diposisikan sebagai **studio/event photo session manager**, bukan CCTV/video streaming platform.

Key ideas:

- Multi-Camera Photo Ingestion Hub
- Studio Station Operating System
- Customer-Based Session Workspace
- Timed Customer Session
- Dual End-Session Control

#### Theme 2 — Camera Management

Camera management menjadi pusat konfigurasi sebelum event berjalan.

MVP camera setting wajib mencakup:

1. Camera Slot Name
2. Physical Device Selection
3. Device Name / Serial / Identifier
4. Input Folder
5. Background Name
6. Default LUT
7. Output Folder Rule
8. Connection Status
9. Test Capture / Test Watch
10. Reconnect / Refresh Device

#### Theme 3 — Session and Routing Logic

Routing berbasis camera station + active session.

Rules:

- 1 camera/background = 1 active customer/session
- Foto diarahkan berdasarkan camera slot/device/folder
- Timer habis = session terkunci
- Foto setelah timer habis masuk quarantine/unassigned

#### Theme 4 — Image Processing Pipeline

Pipeline MVP:

```text
Input Folder
→ detect JPG
→ wait until file stable
→ identify camera slot
→ find active session
→ copy original
→ apply LUT
→ save graded
→ update metadata/status
→ display latest photo
```

Key decisions:

- JPG only
- No RAW
- Original + graded saved
- LUT per camera/background
- LUT locked during active session
- Auto retry 3x + manual retry

#### Theme 5 — Dashboard and Operator UX

Dashboard utama menampilkan 3 camera station cards dengan:

- active customer
- order number
- timer
- kamera/background
- LUT aktif
- last photo
- jumlah foto
- status processing
- error/retry
- reconnect
- end session
- open result folder
- Google Drive upload status

#### Theme 6 — Storage and Output

Storage strategy:

- Local storage di komputer admin
- Google Drive upload setelah session selesai
- Google Drive menggunakan akun admin yang sama
- Folder otomatis berdasarkan customer + order
- Upload original + graded
- Jika internet mati, local save tetap jalan dan Drive retry nanti

#### Theme 7 — Reliability, Audit, and Readiness

MVP hardening included:

- Config Backup and Restore
- Safe File Detection Delay
- Duplicate Filename Protection
- Disk Space Warning
- Missing LUT Fallback
- Session Cannot Start Without Readiness Pass
- Manual Move From Quarantine
- Processing Queue Visibility
- Google Drive Upload Only After Local Completion
- App Health Indicator
- Operator Activity Log

### Prioritization Results

Karena semua core + hardening dipilih masuk MVP, prioritas ditentukan berdasarkan urutan implementasi.

#### Milestone 1 — Local Foundation

- Camera setting data model
- Input folder per camera
- Folder watcher
- Detect JPG baru
- Safe file detection delay
- Duplicate filename protection
- Save metadata
- Local output folder rule

#### Milestone 2 — Session Engine

- Create session per camera station
- Customer name + order number + timer
- One active session per station
- Timer-based session lock
- Late photo quarantine
- Manual move from quarantine
- End session via timer/UI

#### Milestone 3 — Processing Pipeline

- Save original
- Apply LUT
- Save graded
- Processing status
- Auto retry 3x
- Manual retry
- Missing LUT fallback
- Processing queue visibility

#### Milestone 4 — Dashboard UX

- 3 station cards
- Last photo preview
- Timer/status/count
- Reconnect/refresh
- Event readiness checklist
- Session summary
- Open result folder
- Disk space warning
- App health indicator
- Operator activity log

#### Milestone 5 — Google Drive

- Google admin account integration
- Create Drive folder by customer + order
- Upload original + graded after session
- Upload status
- Retry failed upload
- Cloud sync summary

### Breakthrough Concepts

1. **Folder Watching Instead of Direct Camera Control** — makes MVP significantly more realistic.
2. **Camera Station as Operational Unit** — maps technical camera devices to real event operations.
3. **Timer as Capture Gate** — prevents photos from entering the wrong session after time expires.
4. **Late Photo Quarantine** — preserves data without contaminating completed sessions.
5. **JPG-Only LUT Pipeline** — prioritizes speed and reliability over RAW flexibility.
6. **Google Drive After Session, Not Real-Time** — avoids cloud latency affecting capture.
7. **Readiness Check Before Session Start** — catches failures before customer interaction.

## Action Planning

### Week 1 — Local Foundation Prototype

**Immediate Next Steps:**

1. Define camera slot config model.
2. Implement folder watcher for 3 input folders.
3. Detect stable JPG files safely.
4. Generate local output folder structure.
5. Store metadata for each detected photo.

**Success Indicators:**

- New JPG files are detected reliably.
- Files are not processed before transfer completes.
- Metadata records correctly identify camera slot and timestamp.

### Week 2 — Session and Routing

**Immediate Next Steps:**

1. Implement create session per camera station.
2. Add customer name, order number, and timer.
3. Route incoming photos to active station session.
4. Lock session after timer/end session.
5. Move late photos to quarantine/unassigned.

**Success Indicators:**

- Photos route to correct customer/order.
- Timer correctly stops session intake.
- Late photos do not enter completed sessions.

### Week 3 — LUT Processing

**Immediate Next Steps:**

1. Save original JPG.
2. Apply selected LUT per camera/background.
3. Save graded JPG.
4. Add processing status.
5. Implement auto retry 3x and manual retry.

**Success Indicators:**

- Original and graded files are both produced.
- Failed grading is visible and retryable.
- Missing LUT does not destroy original capture.

### Week 4 — Dashboard and Google Drive

**Immediate Next Steps:**

1. Build 3 station dashboard cards.
2. Show timer/status/photo count/last preview.
3. Add event readiness checklist.
4. Add session summary and open result folder.
5. Integrate Google Drive post-session upload.
6. Add Drive upload status and retry.

**Success Indicators:**

- Admin can operate sessions from browser.
- Session result is visible locally.
- Google Drive folder is created and uploaded after session completion.

## Session Summary and Insights

**Key Achievements:**

- Defined a clear MVP product concept for a multi-camera event photo session manager.
- Converted an initially broad idea into specific workflows, data rules, failure handling, and implementation milestones.
- Identified the most important architectural simplification: folder watching instead of direct camera control.
- Established explicit MVP boundaries: no video, no RAW, no AI, no print/share, no real-time Drive upload.
- Added operational hardening appropriate for live event use.

**Session Reflections:**

The strongest outcome of this brainstorming session is that the product became grounded in real event operations: camera stations, customer sessions, timer control, folder-based routing, and post-session delivery. The MVP is ambitious but bounded, with clear technical and operational decisions.

**Final MVP Statement:**

Build a local-network web application running on an admin PC connected to 3 Sony A6000 cameras via USB. The app manages camera settings, customer/order sessions with timers, monitors per-camera JPG input folders, processes incoming photos with per-background LUTs, stores original and graded outputs locally, prevents late photos from entering completed sessions, provides operator dashboard/status/retry/readiness tools, and uploads original + graded session results to Google D
