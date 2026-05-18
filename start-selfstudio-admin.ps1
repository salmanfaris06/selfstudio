# Selfstudio operator startup script
# Run this file with PowerShell as Administrator for full one-click USB attach support.

$ErrorActionPreference = 'Stop'
Set-Location -Path $PSScriptRoot

# Match the WSL distro used for gphoto2.
$env:WSL_DISTRO = if ($env:WSL_DISTRO) { $env:WSL_DISTRO } else { 'Ubuntu' }
$env:PORT = if ($env:PORT) { $env:PORT } else { '3000' }

Write-Host "Starting Selfstudio..." -ForegroundColor Cyan
Write-Host "Project: $PSScriptRoot"
Write-Host "WSL_DISTRO: $env:WSL_DISTRO"
Write-Host "PORT: $env:PORT"
Write-Host ""
Write-Host "Open dashboard: http://localhost:$env:PORT" -ForegroundColor Green
Write-Host "Use gPhoto2 Helper -> Setup + Capture Camera 1/2/3." -ForegroundColor Green
Write-Host ""

npm run dev
