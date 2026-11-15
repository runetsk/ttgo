<#
.SYNOPSIS
  Start the TTGO dev stack (Go backend + Vite frontend) in the background.

.DESCRIPTION
  Builds the backend with the sqlite_fts5 tag, then launches the backend and the
  Vite dev server as detached processes. PIDs and logs go to .dev/ at the repo
  root; stop everything with scripts/dev-stop.ps1.

  Overridable via environment:
    TTGO_BACKEND_PORT   (default 8090 - 8080 is often taken)
    TTGO_FRONTEND_PORT  (default 5173)
    DB_PATH             (default backend/tracker.db)
    ADMIN_EMAIL / ADMIN_PASSWORD (default admin@example.com / changeme123)
#>
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$devDir = Join-Path $root '.dev'
$binDir = Join-Path $devDir 'bin'
$backendDir = Join-Path $root 'backend'
$frontendDir = Join-Path $root 'frontend'

$backendPort = if ($env:TTGO_BACKEND_PORT) { [int]$env:TTGO_BACKEND_PORT } else { 8090 }
$frontendPort = if ($env:TTGO_FRONTEND_PORT) { [int]$env:TTGO_FRONTEND_PORT } else { 5173 }

function Test-Port([int]$port) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $client.Connect('127.0.0.1', $port)
        return $true
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

function Test-Running([string]$name) {
    $pidFile = Join-Path $devDir "$name.pid"
    if (-not (Test-Path $pidFile)) { return $false }
    $procId = [int](Get-Content $pidFile -Raw)
    return [bool](Get-Process -Id $procId -ErrorAction SilentlyContinue)
}

function Wait-Port([int]$port, [string]$label, [System.Diagnostics.Process]$proc, [string]$log) {
    for ($i = 0; $i -lt 60; $i++) {
        if (Test-Port $port) { return }
        if ($proc.HasExited) {
            Write-Host "$label exited early. Last log lines ($log):" -ForegroundColor Red
            Get-Content $log -Tail 20 -ErrorAction SilentlyContinue
            exit 1
        }
        Start-Sleep -Milliseconds 500
    }
    Write-Host "$label did not open port $port within 30s - see $log" -ForegroundColor Red
    exit 1
}

foreach ($svc in 'backend', 'frontend') {
    if (Test-Running $svc) {
        Write-Host "$svc is already running (see .dev/$svc.pid). Run scripts/dev-stop.ps1 first." -ForegroundColor Yellow
        exit 1
    }
}
foreach ($port in $backendPort, $frontendPort) {
    if (Test-Port $port) {
        Write-Host "Port $port is already in use by another process." -ForegroundColor Red
        exit 1
    }
}
if (-not (Test-Path (Join-Path $frontendDir 'node_modules/vite/bin/vite.js'))) {
    Write-Host "frontend/node_modules is missing - run 'npm ci' in frontend/ first." -ForegroundColor Red
    exit 1
}

New-Item -ItemType Directory -Force $binDir | Out-Null

# --- Backend -----------------------------------------------------------------
Write-Host 'Building backend...'
$serverExe = Join-Path $binDir 'ttgo-server.exe'
Push-Location $backendDir
try {
    go build -tags sqlite_fts5 -o $serverExe ./cmd/server
    if ($LASTEXITCODE -ne 0) { Write-Host 'Backend build failed.' -ForegroundColor Red; exit 1 }
} finally {
    Pop-Location
}

# Child processes inherit this session's environment; set what they need and
# restore afterwards so a dot-sourced/in-session run leaves no residue.
$envVars = @{
    LISTEN_ADDR        = ":$backendPort"
    CORS_ORIGIN        = "http://localhost:$frontendPort"
    ADMIN_EMAIL        = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } else { 'admin@example.com' }
    ADMIN_PASSWORD     = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { 'changeme123' }
    TTGO_DEV_API_TARGET = "http://localhost:$backendPort"
}
$saved = @{}
foreach ($k in $envVars.Keys) {
    $saved[$k] = [Environment]::GetEnvironmentVariable($k)
    [Environment]::SetEnvironmentVariable($k, $envVars[$k])
}

try {
    $backendLog = Join-Path $devDir 'backend.log'
    $backend = Start-Process -FilePath $serverExe -WorkingDirectory $backendDir `
        -RedirectStandardOutput $backendLog -RedirectStandardError (Join-Path $devDir 'backend.err.log') `
        -WindowStyle Hidden -PassThru
    Set-Content (Join-Path $devDir 'backend.pid') $backend.Id

    # --- Frontend ------------------------------------------------------------
    # Run vite through node directly (not npm.cmd) so the recorded PID is the
    # server itself and stopping it leaves no orphans.
    $frontendLog = Join-Path $devDir 'frontend.log'
    $frontend = Start-Process -FilePath 'node' `
        -ArgumentList 'node_modules/vite/bin/vite.js', '--port', $frontendPort, '--strictPort' `
        -WorkingDirectory $frontendDir `
        -RedirectStandardOutput $frontendLog -RedirectStandardError (Join-Path $devDir 'frontend.err.log') `
        -WindowStyle Hidden -PassThru
    Set-Content (Join-Path $devDir 'frontend.pid') $frontend.Id
} finally {
    foreach ($k in $saved.Keys) { [Environment]::SetEnvironmentVariable($k, $saved[$k]) }
}

Wait-Port $backendPort 'Backend' $backend (Join-Path $devDir 'backend.err.log')
Wait-Port $frontendPort 'Frontend' $frontend $frontendLog

Write-Host ''
Write-Host "Backend   http://localhost:$backendPort   (pid $($backend.Id))" -ForegroundColor Green
Write-Host "Frontend  http://localhost:$frontendPort   (pid $($frontend.Id))" -ForegroundColor Green
Write-Host "Logs in .dev/   Stop with: scripts/dev-stop.ps1"
