# make dev-local (Windows): start the local Go backend and Vite frontend.
$ErrorActionPreference = 'Stop'

$RepoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
Set-Location $RepoRoot

function Import-LocalEnv {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return
    }

    foreach ($line in Get-Content -LiteralPath $Path -Encoding UTF8) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith('#')) {
            continue
        }
        if ($trimmed.StartsWith('export ')) {
            $trimmed = $trimmed.Substring(7).TrimStart()
        }

        $separator = $trimmed.IndexOf('=')
        if ($separator -le 0) {
            continue
        }

        $name = $trimmed.Substring(0, $separator).Trim()
        if ($name -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
            continue
        }
        if (-not [string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($name, 'Process'))) {
            continue
        }

        $value = $trimmed.Substring($separator + 1).Trim()
        if ($value.Length -ge 2) {
            $first = $value[0]
            $last = $value[$value.Length - 1]
            if (($first -eq '"' -and $last -eq '"') -or ($first -eq "'" -and $last -eq "'")) {
                $value = $value.Substring(1, $value.Length - 2)
            }
        }
        [Environment]::SetEnvironmentVariable($name, $value, 'Process')
    }
}

function Set-DefaultEnv {
    param([string]$Name, [string]$Value)

    if ([string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($Name, 'Process'))) {
        [Environment]::SetEnvironmentVariable($Name, $Value, 'Process')
    }
}

function Test-TcpEndpoint {
    param(
        [string]$HostName,
        [int]$Port,
        [int]$TimeoutMilliseconds = 1000
    )

    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $task = $client.ConnectAsync($HostName, $Port)
        return $task.Wait($TimeoutMilliseconds) -and $client.Connected
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

function Stop-ProcessTree {
    param(
        [System.Diagnostics.Process]$Process,
        [string]$Label = 'process',
        [switch]$Previous
    )

    if (-not $Process) {
        return
    }
    $Process.Refresh()
    if ($Process.HasExited) {
        return
    }

    $scope = if ($Previous) { 'previous dev-local' } else { 'dev-local' }
    Write-Host "==> Stopping $scope $Label (PID $($Process.Id))"
    $taskkill = Join-Path $env:SystemRoot 'System32\taskkill.exe'
    & $taskkill /PID $Process.Id /T /F 2>$null | Out-Null
}

function Save-TrackedProcess {
    param(
        [System.Diagnostics.Process]$Process,
        [string]$Path,
        [string]$Kind
    )

    $Process.Refresh()
    @{
        pid = $Process.Id
        start_time_utc_ticks = $Process.StartTime.ToUniversalTime().Ticks
        repo_root = $RepoRoot
        kind = $Kind
    } | ConvertTo-Json -Compress | Set-Content -LiteralPath $Path -Encoding UTF8
}

function Stop-TrackedProcess {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return
    }
    try {
        $state = Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json
        if ([System.IO.Path]::GetFullPath([string]$state.repo_root) -ne $RepoRoot) {
            return
        }
        $process = Get-Process -Id ([int]$state.pid) -ErrorAction SilentlyContinue
        if ($process) {
            $actualTicks = $process.StartTime.ToUniversalTime().Ticks
            if ($actualTicks -eq [int64]$state.start_time_utc_ticks) {
                Stop-ProcessTree $process ([string]$state.kind) -Previous
            }
        }
    } catch {
        Write-Host "Warning: ignored invalid dev-local process state at $Path"
    } finally {
        Remove-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
    }
}

function Stop-RecognizedDevLocalListener {
    param(
        [int]$Port,
        [string]$Kind
    )

    $connection = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if (-not $connection) {
        return
    }

    $currentPid = [int]$connection.OwningProcess
    $seen = @{}
    $candidatePid = $null
    while ($currentPid -gt 0 -and -not $seen.ContainsKey($currentPid)) {
        $seen[$currentPid] = $true
        $item = Get-CimInstance Win32_Process -Filter "ProcessId = $currentPid" -ErrorAction SilentlyContinue
        if (-not $item) {
            break
        }
        $commandLine = [string]$item.CommandLine
        if ($commandLine -match '(?i)scripts[\\/]dev-local\.ps1') {
            $candidatePid = [int]$item.ProcessId
        } elseif (
            $Kind -eq 'backend' -and
            $item.Name -eq 'go.exe' -and
            $commandLine -match '(?i)\brun\s+\./cmd/server(?:\s|$)'
        ) {
            $candidatePid = [int]$item.ProcessId
        } elseif (
            $item.Name -eq 'node.exe' -and
            $commandLine.IndexOf($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase) -ge 0 -and
            $commandLine -match '(?i)vite'
        ) {
            $candidatePid = [int]$item.ProcessId
        }
        $currentPid = [int]$item.ParentProcessId
    }

    if ($candidatePid) {
        $process = Get-Process -Id $candidatePid -ErrorAction SilentlyContinue
        Stop-ProcessTree $process $Kind -Previous
        Start-Sleep -Milliseconds 500
    }
}

Import-LocalEnv (Join-Path $RepoRoot '.env.local')

$DefaultDataDir = Join-Path $RepoRoot '.dev\local'
Set-DefaultEnv 'DATA_DIR' $DefaultDataDir
Set-DefaultEnv 'AUTO_SETUP' 'true'
Set-DefaultEnv 'SERVER_HOST' '127.0.0.1'
Set-DefaultEnv 'SERVER_PORT' '8080'
Set-DefaultEnv 'SERVER_MODE' 'debug'
Set-DefaultEnv 'DATABASE_HOST' '127.0.0.1'
Set-DefaultEnv 'DATABASE_PORT' '5432'
Set-DefaultEnv 'DATABASE_USER' 'postgres'
Set-DefaultEnv 'DATABASE_PASSWORD' ''
Set-DefaultEnv 'DATABASE_DBNAME' 'sub2api'
Set-DefaultEnv 'DATABASE_SSLMODE' 'disable'
Set-DefaultEnv 'REDIS_HOST' '127.0.0.1'
Set-DefaultEnv 'REDIS_PORT' '6379'
Set-DefaultEnv 'REDIS_USERNAME' ''
Set-DefaultEnv 'REDIS_PASSWORD' ''
Set-DefaultEnv 'REDIS_DB' '0'
Set-DefaultEnv 'REDIS_ENABLE_TLS' 'false'
Set-DefaultEnv 'TZ' 'Asia/Shanghai'
Set-DefaultEnv 'VITE_DEV_PORT' '3000'

$BackendPort = [int]$env:SERVER_PORT
$FrontendPort = [int]$env:VITE_DEV_PORT
$BackendUrl = "http://127.0.0.1:$BackendPort"
$env:VITE_DEV_PROXY_TARGET = $BackendUrl
$DataDir = [System.IO.Path]::GetFullPath($env:DATA_DIR)
$LogDir = Join-Path $DataDir 'logs'
$BackendStateFile = Join-Path $DataDir 'backend-process.json'
New-Item -ItemType Directory -Path $LogDir -Force | Out-Null

# Clean up only processes previously started by this repository's dev-local command.
Stop-TrackedProcess $BackendStateFile
Stop-RecognizedDevLocalListener $BackendPort 'backend'
Stop-RecognizedDevLocalListener $FrontendPort 'frontend'

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host 'Error: Go was not found. Install Go and add it to PATH.'
    exit 127
}

$pnpm = Get-Command pnpm -ErrorAction SilentlyContinue
$pnpmPrefix = @()
if (-not $pnpm) {
    $pnpm = Get-Command corepack -ErrorAction SilentlyContinue
    $pnpmPrefix = @('pnpm')
}
if (-not $pnpm) {
    Write-Host 'Error: pnpm/corepack was not found. Install Node.js and pnpm first.'
    exit 127
}

if (-not (Test-TcpEndpoint $env:DATABASE_HOST ([int]$env:DATABASE_PORT))) {
    Write-Host "Error: cannot connect to PostgreSQL at $($env:DATABASE_HOST):$($env:DATABASE_PORT)."
    exit 1
}
if (-not (Test-TcpEndpoint $env:REDIS_HOST ([int]$env:REDIS_PORT))) {
    Write-Host "Error: cannot connect to Redis at $($env:REDIS_HOST):$($env:REDIS_PORT)."
    exit 1
}
if (Test-TcpEndpoint '127.0.0.1' $BackendPort 300) {
    Write-Host "Error: backend port $BackendPort is already in use."
    exit 1
}
if (Test-TcpEndpoint '127.0.0.1' $FrontendPort 300) {
    Write-Host "Error: frontend port $FrontendPort is already in use."
    exit 1
}

$BackendOutLog = Join-Path $LogDir 'backend.out.log'
$BackendErrLog = Join-Path $LogDir 'backend.err.log'

Write-Host "==> PostgreSQL: $($env:DATABASE_HOST):$($env:DATABASE_PORT)/$($env:DATABASE_DBNAME)"
Write-Host "==> Redis: $($env:REDIS_HOST):$($env:REDIS_PORT) (DB $($env:REDIS_DB))"
Write-Host "==> Starting local backend: $BackendUrl"
Write-Host "==> Backend logs: $LogDir"

$backend = $null
$frontendExitCode = 0
try {
    $backend = Start-Process -FilePath (Get-Command go).Source `
        -ArgumentList @('run', './cmd/server') `
        -WorkingDirectory (Join-Path $RepoRoot 'backend') `
        -RedirectStandardOutput $BackendOutLog `
        -RedirectStandardError $BackendErrLog `
        -WindowStyle Hidden `
        -PassThru
    Save-TrackedProcess $backend $BackendStateFile 'backend'

    $backendReady = $false
    foreach ($attempt in 1..240) {
        $backend.Refresh()
        if ($backend.HasExited) {
            break
        }
        try {
            $response = Invoke-WebRequest -Uri "$BackendUrl/health" -UseBasicParsing -TimeoutSec 1
            if ($response.StatusCode -eq 200) {
                $backendReady = $true
                break
            }
        } catch {
            # The backend is still compiling, migrating, or starting.
        }
        Start-Sleep -Milliseconds 500
    }

    if (-not $backendReady) {
        Write-Host 'Error: local backend failed to start. Recent logs:'
        if (Test-Path -LiteralPath $BackendOutLog) {
            Get-Content -LiteralPath $BackendOutLog -Tail 60
        }
        if (Test-Path -LiteralPath $BackendErrLog) {
            Get-Content -LiteralPath $BackendErrLog -Tail 60
        }
        exit 1
    }

    Write-Host "==> Backend ready. Starting frontend: http://127.0.0.1:$FrontendPort"
    Write-Host '==> Press Ctrl+C to stop both services.'
    Write-Host ''

    & $pnpm.Source @pnpmPrefix --dir frontend run dev --host 127.0.0.1 --strictPort
    $frontendExitCode = $LASTEXITCODE
} finally {
    Stop-ProcessTree $backend 'backend'
    Remove-Item -LiteralPath $BackendStateFile -Force -ErrorAction SilentlyContinue
}

exit $frontendExitCode
