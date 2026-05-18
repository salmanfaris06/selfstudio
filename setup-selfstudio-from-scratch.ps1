# Selfstudio setup from scratch
# Run via "1. INSTALL.bat". The BAT requests Administrator rights.
# Goal: setup dari mesin Windows yang masih kosong sebisa mungkin.
# Catatan: instalasi WSL pertama kali bisa butuh restart Windows dan pembuatan user Ubuntu manual.

$ErrorActionPreference = 'Stop'
Set-Location -Path $PSScriptRoot

$Distro = if ($env:WSL_DISTRO) { $env:WSL_DISTRO } else { 'Ubuntu' }
$Port = if ($env:PORT) { $env:PORT } else { '3000' }
$SonyVidPidPattern = '054c:094e'
$NeedsRerun = $false

function Write-Step([string] $Message) {
  Write-Host ""
  Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Ok([string] $Message) {
  Write-Host "[OK] $Message" -ForegroundColor Green
}

function Write-Warn([string] $Message) {
  Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Test-Command([string] $Name) {
  return $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Refresh-Path {
  $machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  $extraPaths = @(
    "$env:ProgramFiles\nodejs",
    "$env:ProgramFiles\usbipd-win"
  ) | Where-Object { Test-Path $_ }

  $env:Path = (@($machinePath, $userPath) + $extraPaths | Where-Object { $_ }) -join ';'
}

function Invoke-Logged([string] $Command, [string[]] $Arguments, [switch] $AllowFailure) {
  Write-Host "> $Command $($Arguments -join ' ')" -ForegroundColor DarkGray
  $process = Start-Process -FilePath $Command -ArgumentList $Arguments -NoNewWindow -Wait -PassThru
  if ($process.ExitCode -ne 0 -and -not $AllowFailure) {
    throw "Command gagal ($($process.ExitCode)): $Command $($Arguments -join ' ')"
  }
  return $process.ExitCode
}

function Ensure-Winget {
  if (Test-Command 'winget') {
    Write-Ok "winget tersedia"
    return $true
  }

  Write-Warn "winget tidak ditemukan. Install dependency otomatis tidak bisa dilakukan."
  Write-Warn "Install manual: App Installer dari Microsoft Store, lalu jalankan setup ini lagi."
  return $false
}

function Ensure-Node {
  Write-Step "Cek/install Node.js LTS"
  Refresh-Path
  if ((Test-Command 'node') -and (Test-Command 'npm')) {
    Write-Ok "Node.js dan npm tersedia"
    & node --version
    & npm --version
    return
  }

  if (-not (Ensure-Winget)) {
    throw "Node.js belum ada dan winget tidak tersedia. Install Node.js LTS dari https://nodejs.org/ lalu jalankan setup lagi."
  }

  Write-Warn "Node.js belum ditemukan. Mencoba install OpenJS.NodeJS.LTS via winget..."
  Invoke-Logged -Command 'winget' -Arguments @(
    'install', '--id', 'OpenJS.NodeJS.LTS', '-e',
    '--accept-package-agreements', '--accept-source-agreements'
  )
  Refresh-Path

  if (-not ((Test-Command 'node') -and (Test-Command 'npm'))) {
    throw "Node.js sudah dicoba install, tapi belum terdeteksi di PATH. Tutup terminal, buka lagi, lalu jalankan setup ini."
  }

  Write-Ok "Node.js dan npm berhasil tersedia"
  & node --version
  & npm --version
}

function Ensure-Usbipd {
  Write-Step "Cek/install usbipd-win"
  Refresh-Path
  if (Test-Command 'usbipd') {
    Write-Ok "usbipd tersedia"
    & usbipd --version
    return
  }

  if (-not (Ensure-Winget)) {
    throw "usbipd belum ada dan winget tidak tersedia. Install usbipd-win dari https://github.com/dorssel/usbipd-win/releases lalu jalankan setup lagi."
  }

  Write-Warn "usbipd belum ditemukan. Mencoba install dorssel.usbipd-win via winget..."
  Invoke-Logged -Command 'winget' -Arguments @(
    'install', '--id', 'dorssel.usbipd-win', '-e',
    '--accept-package-agreements', '--accept-source-agreements'
  )
  Refresh-Path

  if (-not (Test-Command 'usbipd')) {
    throw "usbipd sudah dicoba install, tapi belum terdeteksi di PATH. Tutup terminal, buka lagi, lalu jalankan setup ini."
  }

  Write-Ok "usbipd berhasil tersedia"
  & usbipd --version
}

function Ensure-WslCommand {
  Write-Step "Cek WSL Windows feature"
  if (Test-Command 'wsl.exe') {
    Write-Ok "wsl.exe tersedia"
    return
  }

  Write-Warn "wsl.exe belum tersedia. Mencoba aktifkan WSL dan Virtual Machine Platform..."
  Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Windows-Subsystem-Linux -NoRestart | Out-Null
  Enable-WindowsOptionalFeature -Online -FeatureName VirtualMachinePlatform -NoRestart | Out-Null
  $script:NeedsRerun = $true
  throw "WSL feature sudah diaktifkan. Restart Windows, lalu jalankan setup ini lagi."
}

function Get-WslDistros {
  $raw = & wsl.exe -l -q 2>$null
  if ($LASTEXITCODE -ne 0) {
    return @()
  }

  return @($raw | ForEach-Object { $_.Trim([char]0).Trim() } | Where-Object { $_ })
}

function Ensure-WslDistro {
  Write-Step "Cek/install WSL distro '$Distro'"
  Ensure-WslCommand

  & wsl.exe --status
  $distros = Get-WslDistros
  if ($distros -contains $Distro) {
    Write-Ok "WSL distro '$Distro' ditemukan"
    return
  }

  Write-Warn "WSL distro '$Distro' belum ditemukan. Mencoba install otomatis."
  Write-Warn "Jika Windows meminta restart, restart dulu lalu jalankan setup ini lagi."
  Write-Warn "Jika Ubuntu terbuka dan meminta username/password Linux, isi dulu sampai selesai, lalu jalankan setup ini lagi."

  $exitCode = Invoke-Logged -Command 'wsl.exe' -Arguments @('--install', '-d', $Distro) -AllowFailure
  $script:NeedsRerun = $true

  if ($exitCode -ne 0) {
    Write-Warn "wsl --install gagal atau butuh tindakan manual. Mencoba install distro saja lewat wsl --install -d $Distro."
  }

  throw "Instalasi WSL/Ubuntu sudah dipicu. Selesaikan prompt/restart jika ada, lalu jalankan setup ini lagi."
}

function Assert-WslUsable {
  Write-Step "Cek Ubuntu/WSL sudah bisa menjalankan bash"
  $exitCode = Invoke-Logged -Command 'wsl.exe' -Arguments @('-d', $Distro, '--', 'bash', '-lc', 'echo WSL_READY') -AllowFailure
  if ($exitCode -ne 0) {
    throw "WSL distro '$Distro' belum siap. Buka Ubuntu sekali dari Start Menu, buat username/password Linux, lalu jalankan setup ini lagi."
  }
  Write-Ok "WSL bash siap"
}

function Invoke-WslBash([string] $Script, [switch] $AllowFailure) {
  $args = @('-d', $Distro, '--', 'bash', '-lc', $Script)
  return Invoke-Logged -Command 'wsl.exe' -Arguments $args -AllowFailure:$AllowFailure
}

function Get-UsbipdSonyBusId {
  $output = & usbipd list 2>$null
  if ($LASTEXITCODE -ne 0) {
    return $null
  }

  $line = $output | Where-Object { $_.ToLowerInvariant().Contains($SonyVidPidPattern) } | Select-Object -First 1
  if (-not $line) {
    return $null
  }

  $parts = ($line -split '\s+') | Where-Object { $_ }
  if ($parts.Count -lt 1) {
    return $null
  }

  return $parts[0]
}

try {
  Write-Host "Selfstudio Setup From Scratch" -ForegroundColor Magenta
  Write-Host "Project : $PSScriptRoot"
  Write-Host "WSL     : $Distro"
  Write-Host "PORT    : $Port"
  Write-Host ""

  Ensure-Node
  Ensure-WslDistro
  Assert-WslUsable
  Ensure-Usbipd

  Write-Step "Pastikan folder runtime tersedia"
  $runtimeDirs = @(
    'data/input/camera-1',
    'data/input/camera-2',
    'data/input/camera-3',
    'data/logs',
    'data/tmp'
  )
  foreach ($dir in $runtimeDirs) {
    New-Item -ItemType Directory -Force -Path (Join-Path $PSScriptRoot $dir) | Out-Null
  }
  Write-Ok "Folder data/input, data/logs, data/tmp siap"

  Write-Step "Install dependency Node.js project"
  if (Test-Path (Join-Path $PSScriptRoot 'package-lock.json')) {
    npm ci
  } else {
    npm install
  }
  Write-Ok "Dependency Node.js project siap"

  Write-Step "Cek TypeScript"
  npm run typecheck
  Write-Ok "Typecheck lolos"

  Write-Step "Install/check gPhoto2 di WSL"
  $checkGphoto = Invoke-WslBash -Script 'command -v gphoto2 >/dev/null 2>&1' -AllowFailure
  if ($checkGphoto -ne 0) {
    Write-Warn "gphoto2 belum ada di WSL. Mencoba install via apt. Password Linux/WSL mungkin diminta."
    Invoke-WslBash -Script 'sudo apt-get update && sudo apt-get install -y gphoto2'
  } else {
    Write-Ok "gphoto2 sudah tersedia di WSL"
  }
  Invoke-WslBash -Script 'gphoto2 --version | head -n 1' -AllowFailure | Out-Null

  Write-Step "Cari Sony A6000 USB device untuk usbipd"
  $busId = Get-UsbipdSonyBusId
  if (-not $busId) {
    Write-Warn "Sony A6000 belum ditemukan oleh usbipd."
    Write-Warn "Pastikan kamera ON, USB terhubung, mode USB kamera = PC Remote, lalu jalankan setup ini lagi jika ingin auto-attach."
  } else {
    Write-Ok "Sony A6000 ditemukan di busid $busId"

    Write-Step "Share/bind Sony USB device ke usbipd"
    Invoke-Logged -Command 'usbipd' -Arguments @('bind', '--busid', $busId) -AllowFailure | Out-Null

    Write-Step "Attach Sony USB device ke WSL '$Distro'"
    Invoke-Logged -Command 'usbipd' -Arguments @('attach', '--busid', $busId, '--wsl', $Distro) -AllowFailure | Out-Null

    Write-Step "Cek deteksi kamera oleh gPhoto2"
    Invoke-WslBash -Script 'gphoto2 --auto-detect' -AllowFailure | Out-Null
  }

  Write-Step "Setup environment startup"
  $env:WSL_DISTRO = $Distro
  $env:PORT = $Port
  Write-Ok "WSL_DISTRO=$Distro"
  Write-Ok "PORT=$Port"

  Write-Host ""
  Write-Host "Setup selesai." -ForegroundColor Green
  Write-Host "Untuk menjalankan aplikasi:" -ForegroundColor Green
  Write-Host "  2. RUN.bat"
  Write-Host "Dashboard:" -ForegroundColor Green
  Write-Host "  http://localhost:$Port"
  Write-Host ""
  Write-Host "Flow kamera fisik:" -ForegroundColor Cyan
  Write-Host "  1. Buka dashboard"
  Write-Host "  2. gPhoto2 Helper -> Start Camera Trigger Camera 1/2/3"
  Write-Host "  3. Tekan shutter fisik di kamera"
  Write-Host ""
} catch {
  Write-Host ""
  Write-Host "Setup belum selesai." -ForegroundColor Yellow
  Write-Host $_.Exception.Message -ForegroundColor Red
  Write-Host ""
  Write-Host "Jika ini adalah setup pertama WSL, urutan normalnya:" -ForegroundColor Cyan
  Write-Host "  1. Jalankan 1. INSTALL.bat sebagai Administrator"
  Write-Host "  2. Jika diminta restart, restart Windows"
  Write-Host "  3. Buka Ubuntu dari Start Menu, buat username/password Linux jika diminta"
  Write-Host "  4. Jalankan 1. INSTALL.bat lagi"
  Write-Host ""
  exit 1
}
