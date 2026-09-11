# Apply pending migrations 14-23 in order
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

$versions = @(
    "14-player-profiles",
    "15-api-governance",
    "16-game-versions",
    "17-index-optimize",
    "18-index-optimize",
    "19-normalize-merchant-refs",
    "20-audit-archive-policy",
    "21-merchant-lobby-view",
    "22-status-checks",
    "23-seed-extra-games"
)

foreach ($v in $versions) {
    & (Join-Path $Root "scripts\apply-migration.ps1") $v
}

Write-Host "All migrations 14-23 applied." -ForegroundColor Green
