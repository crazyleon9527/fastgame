# Windows 版 Makefile 替代 — 用法: .\scripts\make.ps1 <target>
# 示例: .\scripts\make.ps1 up | build | seed-admin | dev-tools

param(
    [Parameter(Position = 0)]
    [string]$Target = "help"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

function Invoke-MySQLFile([string]$SqlPath) {
    Get-Content $SqlPath -Raw | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame
}

function Build-Binaries {
    New-Item -ItemType Directory -Force -Path bin | Out-Null
    go build -o bin/rgs-api.exe ./services/rgs
    go build -o bin/consumer.exe ./services/consumer
    go build -o bin/rollback.exe ./services/rollback
    go build -o bin/admin-api.exe ./services/admin
    go build -o bin/broadcast.exe ./services/broadcast
}

switch ($Target.ToLower()) {
    "help" {
        Write-Host @"
FastGame make.ps1 targets:
  up            docker compose up -d
  down          docker compose down
  ps            docker compose ps
  init          kafka + clickhouse init jobs
  build         compile all Go services to bin/
  dev-start     build + start all Go services (separate windows)
  dev-tools     start Adminer/Kafka UI/RedisInsight
  seed          import demo merchant seed SQL
  seed-admin    create admin user
  seed-rbac     create operator/viewer test users
  migrate-rbac  apply RBAC roles migration
  migrate-db    apply all SQL migrations
  migrate-11    apply platform games migration (11)
  migrate-12    apply operations audit migration (12)
  migrate-13    apply settlement billing migration (13)
  migrate-all   apply migrations 14-18 (features + indexes)
  migrate-i18n  apply i18n dictionary migration (24)
  deploy-admin  build vue-pure-admin and deploy to web/admin
  deploy-client deploy Cocos build to web/game/
"@
    }
    "up" { docker compose up -d }
    "down" { docker compose down }
    "ps" { docker compose ps }
    "init" {
        docker compose up -d kafka clickhouse
        docker compose run --rm kafka-init
        docker compose run --rm clickhouse-init
    }
    "build" { Build-Binaries; Write-Host "Build complete." -ForegroundColor Green }
    "dev-start" { & (Join-Path $Root "scripts\dev-start.ps1") }
    "dev-tools" { docker compose --profile dev-tools up -d }
    "seed" { Invoke-MySQLFile "docker\mysql\init\02-seed.sql" }
    "seed-admin" { go run scripts/seed_admin.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame }
    "seed-rbac" { go run scripts/seed_rbac_users.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame }
    "migrate-rbac" { Invoke-MySQLFile "docker\mysql\init\10-rbac-roles-migration.sql" }
    "migrate-db" { & (Join-Path $Root "scripts\apply-migration.ps1") all }
    "migrate-11" { & (Join-Path $Root "scripts\apply-migration.ps1") "11-platform-games" }
    "migrate-12" { & (Join-Path $Root "scripts\apply-migration.ps1") "12-operations-audit" }
    "migrate-13" { & (Join-Path $Root "scripts\apply-migration.ps1") "13-settlement-billing" }
    "migrate-all" { & (Join-Path $Root "scripts\apply-all-migrations.ps1") }
    "migrate-i18n" { & (Join-Path $Root "scripts\apply-migration.ps1") "24-i18n-dictionary" }
    "deploy-admin" { & (Join-Path $Root "scripts\deploy-admin.ps1") }
    "deploy-client" { & (Join-Path $Root "scripts\deploy-client.ps1") }
    default {
        Write-Host "Unknown target: $Target" -ForegroundColor Red
        & $MyInvocation.MyCommand.Path help
        exit 1
    }
}
