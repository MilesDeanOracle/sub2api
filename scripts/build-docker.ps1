# make build-docker (Windows)：使用 WSL 里的 Docker 打包镜像，并上传到远端服务器。
# 流程（与 scripts/build-docker.sh 一致，Linux/macOS 使用 .sh 版本）：
#   1. 提示输入 SSH 用户名（密码由 ssh/scp 在连接时提示输入，脚本不保存密码）
#   2. 用 WSL Docker 构建镜像 sub2api:<版本号>（版本号取 backend/cmd/server/VERSION）
#   3. docker save 导出 tar 到本地 dist/ 目录（已被 gitignore）
#   4. 上传 tar 到远端服务器 /data/docker-img/ 目录
#      （ssh 建目录、scp 上传，各提示一次密码）
$ErrorActionPreference = 'Stop'

# ===== 可按需修改的配置 =====
$ServerHost = '154.51.40.64'   # 远端服务器地址
$RemoteDir  = '/data/docker-img' # 远端镜像存放目录
$ImageName  = 'sub2api'        # 镜像名
# 构建期代理：WSL 内容器直连网络不通（Windows 代理 TUN 不转发 WSL 直连流量），
# 构建以 --network=host 运行并通过此代理下载依赖（apk/go/npm）；置空则直连
$BuildProxy = 'http://127.0.0.1:7897'
# Go 模块源：goproxy.cn 经本机代理会挂流（go 无超时会永久卡住），改用阿里云源
$GoProxy = 'https://mirrors.aliyun.com/goproxy/,direct'
# ============================

Set-Location (Join-Path $PSScriptRoot '..')
$RepoRoot = (Get-Location).Path

# 1. 输入用户名；密码在 ssh/scp 连接时由其自己提示，不在脚本中出现
$SshUser = Read-Host 'SSH 用户名'
if ([string]::IsNullOrWhiteSpace($SshUser)) {
    Write-Host '错误：用户名不能为空'
    exit 1
}

# 2. 版本号：backend/cmd/server/VERSION，读取失败回退 dev
$Version = 'dev'
$VersionFile = Join-Path $RepoRoot 'backend\cmd\server\VERSION'
if (Test-Path $VersionFile) {
    $v = (Get-Content $VersionFile -Raw).Trim()
    if ($v) { $Version = $v }
}
$Timestamp = Get-Date -Format 'yyyyMMdd-HHmm'
$Tag = "${ImageName}:${Version}"
$TarName = "${ImageName}-${Version}-${Timestamp}.tar"
$OutDir = Join-Path $RepoRoot 'dist'
$TarPath = Join-Path $OutDir $TarName
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

Write-Host ''
Write-Host "==> 目标镜像：$Tag"
Write-Host "==> 本地产物：$TarPath"
Write-Host ''

# 3. 解析 ssh/scp：32 位 PowerShell 下 System32 会被 WoW64 重定向到 SysWOW64，
#    需回退到 Sysnative 访问真实 System32（与 dev.ps1 一致）
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

# 4. 检查 WSL 与其中的 docker
$wslExe = (Get-Command wsl -ErrorAction SilentlyContinue).Source
if (-not $wslExe) {
    Write-Host '错误：未找到 wsl 命令，请先安装 WSL2 并在其中配置 Docker'
    exit 1
}
Write-Host '==> 检查 WSL Docker ...'
$dockerVersion = (& $wslExe docker version --format '{{.Server.Version}}' 2>$null)
if ($LASTEXITCODE -ne 0 -or -not $dockerVersion) {
    Write-Host '错误：WSL 中不可用 docker（docker version 失败）。请确认 WSL 内 Docker 已启动'
    exit 1
}
Write-Host "==> WSL Docker 就绪（server v$dockerVersion）"

# 5. 仓库路径转 WSL 路径（E:\x\y -> /mnt/e/x/y）
#    注：不经 wsl 转发 wslpath —— wsl.exe 转发参数时会吃掉反斜杠，故本地直接转换
if ($RepoRoot -notmatch '^([A-Za-z]):[\\/](.*)$') {
    Write-Host "错误：无法识别的仓库路径：$RepoRoot"
    exit 1
}
$WslRepo = '/mnt/{0}/{1}' -f $Matches[1].ToLower(), ($Matches[2] -replace '\\', '/')
$WslTar = "$WslRepo/dist/$TarName"

# 6. 构建镜像（构建上下文为整个仓库，根目录 Dockerfile）
Write-Host ''
Write-Host "==> docker build -t $Tag $WslRepo"
Write-Host '    （首次构建需下载基础镜像与依赖，耗时较长）'
Write-Host ''
$buildArgs = @('docker', 'build')
if ($BuildProxy) {
    $buildArgs += @(
        '--network=host',
        "--build-arg=HTTP_PROXY=$BuildProxy",
        "--build-arg=HTTPS_PROXY=$BuildProxy",
        '--build-arg=NO_PROXY=127.0.0.1,localhost'
    )
}
$buildArgs += @("--build-arg=GOPROXY=$GoProxy", '-t', $Tag, $WslRepo)
& $wslExe @buildArgs
if ($LASTEXITCODE -ne 0) {
    Write-Host '错误：docker build 失败'
    exit 1
}

# 7. 导出 tar
Write-Host ''
Write-Host "==> docker save -o $WslTar $Tag"
& $wslExe docker save -o $WslTar $Tag
if ($LASTEXITCODE -ne 0) {
    Write-Host '错误：docker save 失败'
    exit 1
}
$TarSize = '{0:N1} MB' -f ((Get-Item $TarPath).Length / 1MB)
Write-Host "==> 镜像已导出：$TarPath（$TarSize）"

# 8. 上传：ssh 建目录 + scp 上传（各提示一次密码）
Write-Host ''
Write-Host "==> 在服务器上创建目录 $RemoteDir（会提示输入密码）"
& $sshExe -o 'StrictHostKeyChecking=accept-new' "${SshUser}@${ServerHost}" "mkdir -p $RemoteDir"
if ($LASTEXITCODE -ne 0) {
    Write-Host '错误：创建远端目录失败（请检查用户名/密码，或对该目录是否有写权限）'
    exit 1
}

Write-Host ''
Write-Host "==> 上传 $TarName -> ${SshUser}@${ServerHost}:$RemoteDir/（会提示输入密码）"
& $scpExe $TarPath "${SshUser}@${ServerHost}:${RemoteDir}/"
if ($LASTEXITCODE -ne 0) {
    Write-Host '错误：上传失败'
    exit 1
}

Write-Host ''
Write-Host '==> 完成！可在服务器上加载镜像：'
Write-Host "    docker load -i $RemoteDir/$TarName"
Write-Host "    docker tag ${ImageName}:${Version} ${ImageName}:latest   # 如需 latest 标签"
