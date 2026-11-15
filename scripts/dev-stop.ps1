<#
.SYNOPSIS
  Stop the TTGO dev stack started by scripts/dev-start.ps1.
#>
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$devDir = Join-Path $root '.dev'

# Expected process names guard against killing an unrelated process that
# reused a stale PID.
$services = [ordered]@{ frontend = 'node'; backend = 'ttgo-server' }

foreach ($name in $services.Keys) {
    $pidFile = Join-Path $devDir "$name.pid"
    if (-not (Test-Path $pidFile)) {
        Write-Host "$name - not running (no pid file)"
        continue
    }
    $procId = [int](Get-Content $pidFile -Raw)
    $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
    if (-not $proc) {
        Write-Host "$name - not running (stale pid $procId)"
    } elseif ($proc.ProcessName -ne $services[$name]) {
        Write-Host "$name - pid $procId is now '$($proc.ProcessName)', leaving it alone" -ForegroundColor Yellow
    } else {
        # /T takes down any children (e.g. esbuild under vite).
        taskkill /PID $procId /T /F | Out-Null
        Write-Host "$name - stopped (pid $procId)" -ForegroundColor Green
    }
    Remove-Item $pidFile -Force
}
