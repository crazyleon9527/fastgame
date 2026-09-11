# 应用 MySQL 迁移脚本
# 用法:
#   .\scripts\apply-migration.ps1 11-platform-games
#   .\scripts\apply-migration.ps1 all

param(
    [Parameter(Position = 0)]
    [string]$Version = "all"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$InitDir = Join-Path $Root "docker\mysql\init"

function Apply-SqlFile([string]$Path) {
    if (-not (Test-Path $Path)) {
        Write-Host "SKIP (not found): $Path" -ForegroundColor Yellow
        return
    }
    Write-Host "Applying: $(Split-Path $Path -Leaf)" -ForegroundColor Cyan
    # docker cp avoids Windows pipe encoding corruption for non-ASCII SQL
    $leaf = Split-Path $Path -Leaf
    $containerPath = "/tmp/$leaf"
    docker cp $Path "fastgame-mysql:$containerPath"
    if ($LASTEXITCODE -ne 0) {
        throw "docker cp failed: $Path"
    }
    docker exec fastgame-mysql mysql --default-character-set=utf8mb4 -ufastgame -pfastgame_pass fastgame -e "source $containerPath"
    if ($LASTEXITCODE -ne 0) {
        throw "Migration failed: $Path"
    }
    docker exec fastgame-mysql rm -f $containerPath | Out-Null
}

# 检查 MySQL 容器
$running = docker ps --filter "name=fastgame-mysql" --format "{{.Names}}" 2>$null
if (-not $running) {
    Write-Host "ERROR: fastgame-mysql container not running. Run: docker compose up -d mysql" -ForegroundColor Red
    exit 1
}

if ($Version -eq "all") {
    Get-ChildItem $InitDir -Filter "*-migration.sql" | Sort-Object Name | ForEach-Object {
        Apply-SqlFile $_.FullName
    }
    Apply-SqlFile (Join-Path $InitDir "02-seed.sql")
} else {
    $pattern = "*$Version*"
    $files = Get-ChildItem $InitDir -Filter $pattern
    if (-not $files) {
        Write-Host "ERROR: No migration matching '$Version' in $InitDir" -ForegroundColor Red
        exit 1
    }
    foreach ($f in $files) { Apply-SqlFile $f.FullName }
}

Write-Host "Done." -ForegroundColor Green
try {
    $tables = docker exec fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame -N -e "SHOW TABLES;" 2>$null
    if ($tables) { $tables | Write-Host }
} catch {
    # mysql password warning on stderr should not fail the migration
}
