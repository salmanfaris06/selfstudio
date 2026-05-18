@echo off
setlocal
cd /d "%~dp0"
set WSL_DISTRO=Ubuntu
set PORT=3000
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-selfstudio-admin.ps1"
