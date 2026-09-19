# apply_clickhouse_migration.ps1 — 执行 ClickHouse 迁移 SQL（可靠方式）
#
# 背景（一次真实事故的教训）：
#   原先用 `Get-Content ... | docker exec -i clickhouse-client --multiquery` 执行
#   多语句迁移，出现**只执行了前几条就被截断**的情况：改名与 DROP 生效、
#   建表语句没执行，留下"表不见了"的半成品状态。
#
#   因此改为：把整个迁移文件 docker cp 进容器，再用 clickhouse-client 的
#   `--queries-file` 读取文件执行（不进管道）。ClickHouse 客户端会按语句
#   顺序执行并在首个错误处停止，不会静默截断。
#
# 用法:
#   powershell -ExecutionPolicy Bypass -File .\scripts\apply_clickhouse_migration.ps1 docker\clickhouse\init\05-dedup-migration.sql

param(
    [Parameter(Position = 0, Mandatory = $true)][string]$Path
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

$src = (Resolve-Path $Path).Path
if (-not (Test-Path $src)) { throw "迁移文件不存在: $Path" }

$leaf = Split-Path $src -Leaf
$containerPath = "/tmp/$leaf"

Write-Host "==> 迁移: $leaf" -ForegroundColor Cyan
docker cp $src "fastgame-clickhouse:$containerPath" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "docker cp 失败: $src" }

# 逐条执行（--queries-file 由客户端按 `;` 切分并顺序执行）
docker exec fastgame-clickhouse clickhouse-client `
    --user fastgame --password fastgame_pass --multiquery --queries-file $containerPath
$rc = $LASTEXITCODE
docker exec fastgame-clickhouse rm -f $containerPath | Out-Null

if ($rc -ne 0) {
    throw "迁移失败（exit=$rc）：$leaf —— 请注意前述语句可能已生效，修正状态后再重跑"
}

Write-Host "==> 迁移完成" -ForegroundColor Green
Write-Host "当前表："
docker exec fastgame-clickhouse clickhouse-client --user fastgame --password fastgame_pass `
    --query "SELECT name, engine FROM system.tables WHERE database='fastgame' ORDER BY name FORMAT TSV"
