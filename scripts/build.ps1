$ErrorActionPreference = "Stop"

function Require-Command($Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command '$Name' was not found. Install it and rerun this script."
  }
}

Require-Command "node"
Require-Command "npm"
Require-Command "go"

$Root = Split-Path -Parent $PSScriptRoot
$AgentPath = Join-Path $Root "apps/agent"
$WebPath = Join-Path $Root "apps/web"
$DistAgent = Join-Path $Root "dist/agent"

New-Item -ItemType Directory -Force -Path $DistAgent | Out-Null

Write-Host "== Building Selfstudio =="
Write-Host "Building Go agent..."
Push-Location $AgentPath
go test ./...
go build -o (Join-Path $DistAgent "selfstudio-agent.exe") ./cmd/selfstudio-agent
Pop-Location

Write-Host "Building Next.js dashboard..."
Push-Location $WebPath
npm install
npm run build
Pop-Location

Write-Host "Build complete."
