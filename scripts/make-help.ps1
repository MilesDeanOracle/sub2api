# make help (Windows)：列出 Makefile 中带 ## 注解的目标
# Windows 下 make 用 cmd 执行命令，没有 awk，因此用 PowerShell 实现（Linux/macOS 用 Makefile 内联 awk）
$lines = Get-Content -Path 'Makefile' -Encoding UTF8
$seen = @{}
foreach ($line in $lines) {
    if ($line -match '^##\s?(.*)$') {
        Write-Host $Matches[1]
    } elseif ($line -match '^([a-zA-Z0-9_-]+):.*##\s?(.*)$') {
        # ifeq/else 双分支会让同一目标带两份注解，按目标名去重
        if (-not $seen.ContainsKey($Matches[1])) {
            $seen[$Matches[1]] = $true
            Write-Host ('  {0,-24} {1}' -f $Matches[1], $Matches[2])
        }
    }
}
