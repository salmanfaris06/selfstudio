@echo off
setlocal EnableExtensions

rem Selfstudio setup from scratch launcher.
rem This file self-elevates to Administrator, then runs the PowerShell setup script.

cd /d "%~dp0"

net session >nul 2>&1
if %errorlevel% neq 0 (
  echo Requesting Administrator permission...
  powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0setup-selfstudio-from-scratch.ps1"
set EXIT_CODE=%errorlevel%

echo.
if not "%EXIT_CODE%"=="0" (
  echo Setup exited with code %EXIT_CODE%.
) else (
  echo Setup completed.
)
pause
exit /b %EXIT_CODE%
