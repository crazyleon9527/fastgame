# DB schema evolution loop — migrations then ongoing optimization
# Usage: .\scripts\db-schema-loop.ps1
# Stop: Ctrl+C or close window

$IntervalSec = 600
$Sentinel = "AGENT_LOOP_TICK_db-schema"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Write-Host "DB schema loop started. Interval: ${IntervalSec}s" -ForegroundColor Cyan
Write-Host "Phase: maintenance (recurring_checks from optimization backlog)" -ForegroundColor Green
Write-Host "Backlog: docker/mysql/migrations/schema_optimization_backlog.json" -ForegroundColor DarkGray
Write-Host "Stop with Ctrl+C" -ForegroundColor DarkYellow

while ($true) {
    Start-Sleep -Seconds $IntervalSec
    $payload = @{
        prompt = @"
FastGame MySQL schema maintenance tick:
1. Read docker/mysql/migrations/schema_optimization_backlog.json recurring_checks and future_candidates
2. Pick one check (EXPLAIN slow query, index audit, archive dry-run, merchant_id backfill verify)
3. If issue found, create docker/mysql/init/24+-migration.sql fix, apply via .\scripts\apply-migration.ps1
4. Update docs/DATABASE.md if schema changed
5. Report findings; no change is OK if all checks pass
"@
    } | ConvertTo-Json -Compress
    Write-Output "$Sentinel $payload"
}
