# make dev (Windows)：先通过 SSH 隧道把远程后端服务转发到本机，再启动前端开发服务器。
# 流程与 scripts/dev.sh 一致（Linux/macOS 使用 dev.sh）：
#   1. 提示输入 SSH 用户名（密码由 ssh 在连接时提示输入，脚本不保存密码）
#   2. 后台建立隧道 127.0.0.1:8090 -> 隧道服务器 154.51.40.64 上的后端服务
#   3. 以 VITE_DEV_PROXY_TARGET=http://127.0.0.1:8090 启动 vite 开发服务器
#   4. 前端退出（Ctrl+C）时自动关闭隧道
$ErrorActionPreference = 'Stop'

# ===== 可按需修改的配置 =====
$TunnelHost = '154.51.40.64'   # 隧道服务器地址
$LocalPort  = 8090             # 本机访问后端的端口
$RemoteHost = '127.0.0.1'      # 后端服务在隧道服务器上的地址
$RemotePort = 8090             # 后端服务在隧道服务器上的端口
# ============================

Set-Location (Join-Path $PSScriptRoot '..')

$BackendUrl = "http://127.0.0.1:$LocalPort"

# 1. 输入用户名；密码在 ssh 连接时由 ssh 自己提示，不在脚本中出现
$SshUser = Read-Host 'SSH 用户名'
if ([string]::IsNullOrWhiteSpace($SshUser)) {
    Write-Host '错误：用户名不能为空'
    exit 1
}

Write-Host ''
Write-Host "==> 建立隧道 $BackendUrl -> $SshUser@$TunnelHost/${RemoteHost}:$RemotePort"
Write-Host '==> 连接时会提示输入密码（输入时不回显，属正常现象）'
Write-Host ''

# 2. 后台建立隧道；-N 只做端口转发不执行远程命令，绑定本机回环避免暴露到局域网
# 解析 ssh.exe：优先 PATH 里的 ssh；32 位 PowerShell 下 System32 会被 WoW64
# 重定向到 SysWOW64（那里没有 OpenSSH），需回退到 Sysnative 访问真实 System32
$sshExe = (Get-Command ssh -ErrorAction SilentlyContinue).Source
if (-not $sshExe) { $sshExe = "$env:WINDIR\Sysnative\OpenSSH\ssh.exe" }
if (-not (Test-Path $sshExe)) { $sshExe = "$env:WINDIR\System32\OpenSSH\ssh.exe" }
if (-not (Test-Path $sshExe)) {
    Write-Host '错误：未找到 ssh 客户端，请安装 Windows OpenSSH 或将 ssh 加入 PATH'
    exit 1
}

$tunnel = Start-Process -FilePath $sshExe -ArgumentList @(
    '-N',
    '-o', 'ExitOnForwardFailure=yes',
    '-o', 'StrictHostKeyChecking=accept-new',
    '-o', 'ServerAliveInterval=30',
    '-o', 'ServerAliveCountMax=3',
    '-L', "127.0.0.1:${LocalPort}:${RemoteHost}:${RemotePort}",
    "$SshUser@$TunnelHost"
) -PassThru -NoNewWindow

try {
    # 3. 等待本地端口就绪（ssh 认证通过后才会开始监听；最多等 ~10 秒）
    $tunnelReady = $false
    foreach ($i in 1..50) {
        if ($tunnel.HasExited) {
            Write-Host "错误：隧道建立失败（请检查用户名/密码，或本机能否访问 ${TunnelHost}:22）"
            exit 1
        }
        $client = New-Object System.Net.Sockets.TcpClient
        try {
            $async = $client.BeginConnect('127.0.0.1', $LocalPort, $null, $null)
            if ($async.AsyncWaitHandle.WaitOne(200) -and $client.Connected) {
                $tunnelReady = $true
                break
            }
        } catch {
            # 端口未就绪，继续等待
        } finally {
            $client.Close()
        }
        Start-Sleep -Milliseconds 200
    }
    if (-not $tunnelReady) {
        Write-Host '警告：10 秒内未能确认隧道就绪，仍继续启动前端（后端请求可能暂时失败）'
    }

    # 4. 启动前端开发服务器，API 代理指向隧道后的后端
    #    pnpm 不在 PATH 时回退到 corepack pnpm（Node 自带 corepack）
    function Invoke-Pnpm {
        if (Get-Command pnpm -ErrorAction SilentlyContinue) {
            & pnpm @args
        } elseif (Get-Command corepack -ErrorAction SilentlyContinue) {
            & corepack pnpm @args
        } else {
            Write-Host '错误：未找到 pnpm，请先安装（npm i -g pnpm 或 corepack enable）'
            exit 127
        }
    }

    Write-Host "==> 隧道就绪，后端地址：$BackendUrl"
    Write-Host '==> 启动前端开发服务器（Ctrl+C 退出并自动断开隧道）'
    Write-Host ''
    $env:VITE_DEV_PROXY_TARGET = $BackendUrl
    Invoke-Pnpm --dir frontend run dev
} finally {
    if ($tunnel -and -not $tunnel.HasExited) {
        Write-Host ''
        Write-Host "==> 关闭 SSH 隧道（PID $($tunnel.Id)）"
        Stop-Process -Id $tunnel.Id -Force -ErrorAction SilentlyContinue
    }
}
