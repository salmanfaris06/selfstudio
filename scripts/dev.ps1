$ErrorActionPreference = "Stop"

function Require-Command($Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command '$Name' was not found. Install it and rerun this script."
  }
}

function Test-TcpPort($Name, $Value) {
  $Value = $Value.Trim()
  if ($Value -notmatch '^[1-9][0-9]{0,4}$') {
    throw "$Name must be a TCP port between 1 and 65535."
  }

  $PortNumber = [int]$Value
  if ($PortNumber -gt 65535) {
    throw "$Name must be a TCP port between 1 and 65535."
  }
}

function Require-EnvValue($Name, $Value) {
  if ([string]::IsNullOrWhiteSpace($Value)) {
    throw "$Name is required. Set it in your PowerShell session or local .env before starting Selfstudio."
  }
}

function Get-LanIpAddress {
  $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object { $_.IPAddress -notlike "127.*" -and $_.PrefixOrigin -ne "WellKnown" } |
    Select-Object -First 1 -ExpandProperty IPAddress

  if ($ip) { return $ip }
  return $null
}

Require-Command "node"
Require-Command "npm"
Require-Command "go"

$Root = Split-Path -Parent $PSScriptRoot
$AgentPath = Join-Path $Root "apps/agent"
$WebPath = Join-Path $Root "apps/web"
$LocalDataDir = if ($env:SELFSTUDIO_LOCAL_DATA_DIR) { $env:SELFSTUDIO_LOCAL_DATA_DIR } else { Join-Path $Root "local-data" }
$AgentHost = if ($env:SELFSTUDIO_AGENT_HOST) { $env:SELFSTUDIO_AGENT_HOST } else { "127.0.0.1" }
$AgentPort = if ($env:SELFSTUDIO_AGENT_PORT) { $env:SELFSTUDIO_AGENT_PORT } else { "8080" }
$WebPort = if ($env:SELFSTUDIO_WEB_PORT) { $env:SELFSTUDIO_WEB_PORT } else { "3000" }
$AuthPin = $env:SELFSTUDIO_AUTH_PIN
$ApiUrl = if ($env:NEXT_PUBLIC_SELFSTUDIO_API_URL) { $env:NEXT_PUBLIC_SELFSTUDIO_API_URL } else { "http://localhost:$AgentPort" }
$LanIp = Get-LanIpAddress

Test-TcpPort "SELFSTUDIO_AGENT_PORT" $AgentPort
Test-TcpPort "SELFSTUDIO_WEB_PORT" $WebPort
Require-EnvValue "SELFSTUDIO_AUTH_PIN" $AuthPin

Write-Host "== Selfstudio Local Development =="
Write-Host "Agent health: http://localhost:$AgentPort/health"
Write-Host "Dashboard local: http://localhost:$WebPort"
if ($LanIp) {
  if ($AgentHost -eq "127.0.0.1") {
    Write-Host "Dashboard LAN: unavailable because agent is bound to 127.0.0.1"
  } else {
    Write-Host "Dashboard LAN: http://$LanIp`:$WebPort"
  }
} else {
  Write-Host "Dashboard LAN: detection unavailable"
}

Write-Host "Starting Go agent..."
$AgentCommand = @"
Set-Location -LiteralPath '$($AgentPath.Replace("'", "''"))'
`$env:SELFSTUDIO_AGENT_HOST = '$($AgentHost.Replace("'", "''"))'
`$env:SELFSTUDIO_AGENT_PORT = '$AgentPort'
`$env:SELFSTUDIO_LOCAL_DATA_DIR = '$($LocalDataDir.Replace("'", "''"))'
`$env:SELFSTUDIO_AUTH_PIN = '$($AuthPin.Replace("'", "''"))'
go run ./cmd/selfstudio-agent
"@
Start-Process powershell -ArgumentList @("-NoExit", "-Command", $AgentCommand) -ErrorAction Stop

Write-Host "Starting Next.js dashboard..."
$WebCommand = @"
Set-Location -LiteralPath '$($WebPath.Replace("'", "''"))'
`$env:NEXT_PUBLIC_SELFSTUDIO_API_URL = '$($ApiUrl.Replace("'", "''"))'
npm install
npm run dev -- --port $WebPort
"@
Start-Process powershell -ArgumentList @("-NoExit", "-Command", $WebCommand) -ErrorAction Stop
