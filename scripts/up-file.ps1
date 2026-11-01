# make up-file <tar路径> (Windows)：跳过构建，直接上传已打包的 docker 镜像 tar 到远端服务器。
# 与 scripts/build-docker.ps1 的第 8 步相同，只是复用本地已有产物（如 dist\*.tar），
# 适合构建完成后重传、或 build-docker 只跑到 save 就中断的场景。
# 流程（与 scripts/up-file.sh 一致，Linux/macOS 使用 .sh 版本）：
#   1. 校验 tar 文件存在
#   2. 提示输入 SSH 用户名（密码由 ssh/scp 在连接时提示输入，脚本不保存密码）
#   3. 上传 tar 到远端服务器 /data/docker-img/ 目录
#      （ssh 建目录、scp 上传，各提示一次密码）
param($TarPath)
$ErrorActionPreference = 'Stop'

# ===== 可按需修改的配置（与 scripts/build-docker.ps1 保持一致） =====
$ServerHost = '154.51.40.64'   # 远端服务器地址
$RemoteDir  = '/data/docker-img' # 远端镜像存放目录
# ============================

# 1. 参数：tar 文件路径；缺省或传入目录时，自动选取该目录（默认 dist\）下最新的 *.tar
if ([string]::IsNullOrWhiteSpace($TarPath) -or $TarPath -eq 'latest') {
    $TarPath = 'dist'
}
if (Test-Path $TarPath -PathType Container) {
    $picked = Get-ChildItem -Path $TarPath -Filter '*.tar' -File | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if (-not $picked) {
        Write-Host "错误：目录 $TarPath 下没有 .tar 文件（可先运行 make build-docker 生成）"
        exit 1
    }
    Write-Host '==> 未指定文件，自动选择目录内最新 tar'
    $TarItem = $picked
} else {
    if (-not (Test-Path $TarPath -PathType Leaf)) {
        Write-Host "错误：找不到 tar 文件：$TarPath"
        Write-Host '用法：make up-file [tar文件路径]（不带参数自动上传 dist/ 下最新的 tar）'
        Write-Host '      例如：make up-file dist/sub2api-0.2.19-20260101-1200.tar'
        exit 1
    }
    $TarItem = Get-Item $TarPath
}
$TarName = $TarItem.Name
$TarSize = '{0:N1} MB' -f ($TarItem.Length / 1MB)

Write-Host ''
Write-Host "==> 上传文件：$($TarItem.FullName)（$TarSize）"

# 2. 输入用户名；密码在 ssh/scp 连接时由其自己提示，不在脚本中出现
$SshUser = Read-Host 'SSH 用户名'
if ([string]::IsNullOrWhiteSpace($SshUser)) {
    Write-Host '错误：用户名不能为空'
    exit 1
}

# 3. 解析 ssh/scp：32 位 PowerShell 下 System32 会被 WoW64 重定向到 SysWOW64，
#    需回退到 Sysnative 访问真实 System32（与 build-docker.ps1 一致）
function Resolve-Tool {
    param($Name, $Relative)
    $exe = (Get-Command $Name -ErrorAction SilentlyContinue).Source
    if (-not $exe) { $exe = Join-Path $env:WINDIR (Join-Path 'Sysnative' $Relative) }
    if (-not (Test-Path $exe)) { $exe = Join-Path $env:WINDIR (Join-Path 'System32' $Relative) }
    return $exe
}
$sshExe = Resolve-Tool 'ssh' 'OpenSSH\ssh.exe'
$scpExe = Resolve-Tool 'scp' 'OpenSSH\scp.exe'
if (-not (Test-Path $sshExe) -or -not (Test-Path $scpExe)) {
    Write-Host '错误：未找到 ssh/scp 客户端，请安装 Windows OpenSSH 或将其加入 PATH'
    exit 1
}

Write-Host ''
Write-Host "==> 目标位置：${SshUser}@${ServerHost}:$RemoteDir/$TarName"
Write-Host ''

# 4. 上传：ssh 建目录 + scp 上传（各提示一次密码）
Write-Host "==> 在服务器上创建目录 $RemoteDir（会提示输入密码）"
& $sshExe -o 'StrictHostKeyChecking=accept-new' "${SshUser}@${ServerHost}" "mkdir -p $RemoteDir"
if ($LASTEXITCODE -ne 0) {
    Write-Host '错误：创建远端目录失败（请检查用户名/密码，或对该目录是否有写权限）'
    exit 1
}

Write-Host ''
Write-Host "==> 上传 $TarName -> ${SshUser}@${ServerHost}:$RemoteDir/（会提示输入密码）"
& $scpExe $TarItem.FullName "${SshUser}@${ServerHost}:${RemoteDir}/"
if ($LASTEXITCODE -ne 0) {
    Write-Host '错误：上传失败'
    exit 1
}

Write-Host ''
Write-Host '==> 完成！可在服务器上加载镜像：'
Write-Host "    docker load -i $RemoteDir/$TarName"
