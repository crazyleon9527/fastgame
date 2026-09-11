# FastGame 本地开发 — 一键编译并启动全部 Go 微服务
# 用法: .\scripts\dev-start.ps1
# 前置: docker compose up -d 已运行基础设施

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

Write-Host "==> Building Go services..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -o bin/rgs-api.exe ./services/rgs
go build -o bin/admin-api.exe ./services/admin
go build -o bin/broadcast.exe ./services/broadcast
go build -o bin/consumer.exe ./services/consumer
go build -o bin/rollback.exe ./services/rollback

$services = @(
    @{ Name = "rgs-api";     Exe = "bin\rgs-api.exe";     Config = "services\rgs\etc\rgs-api.yaml";       Port = 18888 },
    @{ Name = "admin-api";   Exe = "bin\admin-api.exe";   Config = "services\admin\etc\admin-api.yaml"; Port = 18889 },
    @{ Name = "broadcast";   Exe = "bin\broadcast.exe";   Config = "services\broadcast\etc\broadcast.yaml"; Port = 18900 },
    @{ Name = "consumer";    Exe = "bin\consumer.exe";    Config = "services\consumer\etc\consumer.yaml";  Port = $null },
    @{ Name = "rollback";    Exe = "bin\rollback.exe";    Config = "services\rollback\etc\rollback.yaml";  Port = $null }
)

Write-Host "==> Starting services in new windows..." -ForegroundColor Cyan
foreach ($svc in $services) {
    $arg = "-f `"$($svc.Config)`""
    Start-Process -FilePath (Join-Path $Root $svc.Exe) -ArgumentList $arg -WorkingDirectory $Root -WindowStyle Normal
    if ($svc.Port) {
        Write-Host "  started $($svc.Name) on port $($svc.Port)" -ForegroundColor Green
    } else {
        Write-Host "  started $($svc.Name)" -ForegroundColor Green
    }
    Start-Sleep -Milliseconds 500
}

Write-Host ""
Write-Host "All services launched. Endpoints:" -ForegroundColor Yellow
Write-Host "  Gateway:     http://localhost:18000"
Write-Host "  Admin:       http://localhost:18000/admin/  (or sponge.localhost)"
Write-Host "  RGS API:     http://localhost:18888"
Write-Host "  Admin API:   http://localhost:18889"
Write-Host "  Broadcast:   ws://localhost:18900/ws/bigwin"
Write-Host ""
Write-Host "Ensure .env exists (copy from .env.example) and docker compose is up." -ForegroundColor DarkYellow
