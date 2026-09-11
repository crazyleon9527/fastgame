# 将 Cocos Creator Web Mobile 构建产物部署到 Gateway 静态目录
# 前置: 在 client/ 目录完成 Cocos 构建 (build/web-mobile)
# 用法: .\scripts\deploy-client.ps1

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Src = Join-Path $Root "client\build\web-mobile"
$Dest = Join-Path $Root "web\game"

if (-not (Test-Path $Src)) {
    Write-Host "ERROR: 未找到 $Src" -ForegroundColor Red
    Write-Host "请先在 Cocos Creator 中构建 Web Mobile，输出到 client/build/web-mobile" -ForegroundColor Yellow
    exit 1
}

Write-Host "==> Deploying game client from $Src to $Dest" -ForegroundColor Cyan

# 保留 README 占位说明（若存在）
$readmeBackup = $null
$readmePath = Join-Path $Dest "README.md"
if (Test-Path $readmePath) {
    $readmeBackup = Get-Content $readmePath -Raw
}

Get-ChildItem $Dest -Exclude "README.md" | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
Copy-Item -Path (Join-Path $Src "*") -Destination $Dest -Recurse -Force

if ($readmeBackup) {
    Set-Content -Path $readmePath -Value $readmeBackup -NoNewline
}

Write-Host "Game client deployed. Restart gateway if needed:" -ForegroundColor Green
Write-Host "  docker compose restart gateway" -ForegroundColor Yellow
Write-Host "访问: http://localhost:18000/game/" -ForegroundColor Yellow
