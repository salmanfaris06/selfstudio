# Selfstudio

Selfstudio adalah aplikasi web lokal untuk membantu operasional event/studio/photo booth dengan beberapa kamera Sony A6000. Aplikasi berjalan di PC admin Windows, lalu dashboard dibuka dari browser lokal.

## Kebutuhan Utama

- Windows 10/11
- Internet saat setup pertama
- Sony A6000 via USB
- Kamera diset ke **USB Connection / USB Mode: PC Remote** untuk mode gPhoto2
- Jalankan script setup/run sebagai **Administrator** supaya `usbipd` bisa bind/attach USB ke WSL

Setup otomatis akan mencoba memasang:

- Node.js LTS
- WSL + Ubuntu
- usbipd-win
- dependency npm project
- gPhoto2 di Ubuntu WSL

> Catatan: setup WSL pertama kali bisa tetap membutuhkan restart Windows dan pembuatan username/password Ubuntu secara manual.

## Urutan Setup Dari Nol

### 1. Download project

Clone dari GitHub:

```bat
git clone https://github.com/salmanfaris06/selfstudio.git
cd selfstudio
```

Atau download ZIP dari GitHub, extract, lalu buka folder `selfstudio`.

### 2. Siapkan kamera

1. Nyalakan Sony A6000.
2. Sambungkan kamera ke PC admin via USB.
3. Set kamera ke mode **PC Remote**.
4. Pastikan kamera tidak sedang di mode Mass Storage jika ingin memakai trigger/capture via gPhoto2.

### 3. Jalankan installer

Klik kanan file:

```txt
1. INSTALL.bat
```

Pilih:

```txt
Run as Administrator
```

Script akan mengecek/memasang Node.js, WSL, Ubuntu, usbipd, dependency project, dan gPhoto2.

### 4. Jika diminta restart

Jika setup WSL meminta restart:

1. Restart Windows.
2. Setelah masuk Windows lagi, lanjutkan ke langkah berikutnya.

### 5. Jika Ubuntu meminta user/password

Buka aplikasi **Ubuntu** dari Start Menu.

Jika muncul prompt:

```txt
Enter new UNIX username:
New password:
Retype new password:
```

Isi username dan password Linux/Ubuntu. Simpan password ini karena bisa diminta saat install gPhoto2.

Setelah Ubuntu selesai setup, tutup jendela Ubuntu.

### 6. Jalankan installer lagi

Klik kanan lagi:

```txt
1. INSTALL.bat
```

Pilih:

```txt
Run as Administrator
```

Setup akan lanjut dari bagian yang belum selesai.

### 7. Jika install gPhoto2 meminta password

Saat script menjalankan `sudo apt-get install -y gphoto2`, Ubuntu bisa meminta password. Masukkan password Ubuntu yang dibuat pada langkah 5.

### 8. Pastikan setup selesai

Setup berhasil jika muncul pesan seperti:

```txt
Setup selesai.
Untuk menjalankan aplikasi:
  start-selfstudio-admin.bat
Dashboard:
  http://localhost:3000
```

Di folder project, script run utama adalah:

```txt
2. RUN.bat
```

## Urutan Menjalankan Aplikasi

### 1. Jalankan aplikasi

Klik kanan:

```txt
2. RUN.bat
```

Pilih:

```txt
Run as Administrator
```

### 2. Buka dashboard

Buka browser:

```txt
http://localhost:3000
```

### 3. Cek kamera di dashboard

Di panel **gPhoto2 Helper**, pastikan kamera terdeteksi sebagai kurang lebih:

```txt
Sony Alpha-A6000 (Control)
```

Jika belum terdeteksi, pastikan:

- Kamera ON
- USB tersambung
- Mode kamera **PC Remote**
- Setup dijalankan sebagai Administrator
- `1. INSTALL.bat` sudah dijalankan ulang setelah restart/Ubuntu setup

### 4. Jalankan mode shutter fisik

Untuk Camera 1:

1. Klik **Start Camera Trigger Camera 1**.
2. Tekan shutter fisik di kamera Sony A6000.
3. Foto akan di-download oleh gPhoto2 dan diproses watcher.

Untuk kamera lain, gunakan tombol:

- **Start Camera Trigger Camera 2**
- **Start Camera Trigger Camera 3**

## Lokasi Folder Penting

Input kamera:

```txt
data/input/camera-1
data/input/camera-2
data/input/camera-3
```

Log runtime:

```txt
data/logs
```

Dokumen planning/implementation:

```txt
_bmad-output/planning-artifacts
_bmad-output/implementation-artifacts
```

## Command Developer

Install dependency manual:

```bat
npm ci
```

Jalankan dev server:

```bat
npm run dev
```

Typecheck:

```bat
npm run typecheck
```

## Troubleshooting Singkat

### WSL belum tersedia

Jalankan `1. INSTALL.bat` sebagai Administrator. Jika diminta restart, restart Windows lalu jalankan lagi.

### Ubuntu belum siap

Buka Ubuntu dari Start Menu, buat username/password Linux, lalu jalankan `1. INSTALL.bat` lagi.

### Kamera tidak muncul di gPhoto2

Cek:

- Kamera dalam mode **PC Remote**
- Kabel USB data, bukan kabel charge-only
- Kamera ON
- Jalankan `1. INSTALL.bat` sebagai Administrator agar `usbipd bind/attach` berjalan

### Port 3000 sudah dipakai

Set environment variable `PORT` sebelum menjalankan server, atau ubah script run sesuai kebutuhan.

Contoh PowerShell:

```powershell
$env:PORT = "3101"
npm run dev
```
