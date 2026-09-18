# gen_comment_repair.ps1 — 生成"注释修复"迁移与其回滚脚本
#
# 只处理**基表中真正损坏**的列注释（ASCII 问号 或 双重编码乱码）。
# 修复内容取自 docker/mysql/migrations/comment_manifest.json（由迁移文件提取的权威中文）。
#
# 关键设计：用 information_schema 的 COLUMN_TYPE / IS_NULLABLE / COLUMN_DEFAULT / EXTRA
# 原样重建列定义，只替换 COMMENT，确保类型、默认值、auto_increment、on update 都不漂移。
#
# 用法:
#   powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_repair.ps1
# 输出:
#   docker/mysql/migrations/29-repair-column-comments-migration.sql   （修复）
#   docker/mysql/migrations/29-repair-column-comments-rollback.sql （回滚）
#   docker/mysql/migrations/comment_damage.json             （损坏明细，供审计）

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

$enc  = New-Object System.Text.UTF8Encoding($false)
$utf8 = [System.Text.Encoding]::UTF8
$tmp  = Join-Path ([System.IO.Path]::GetTempPath()) "fastgame_dbdoc"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

function Invoke-Sql([string]$query) {
  $f = Join-Path $tmp ("q_" + [guid]::NewGuid().ToString("N") + ".sql")
  [System.IO.File]::WriteAllText($f, $query, $enc)
  try {
    docker cp $f "fastgame-mysql:/tmp/q.sql" 2>&1 | Out-Null
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $out = docker exec fastgame-mysql mysql --default-character-set=utf8mb4 -uroot -pfastgame_root -N -B -e "source /tmp/q.sql" 2>&1 |
           Where-Object { $_ -notmatch 'Using a password' }
    $ErrorActionPreference = $prev
    return $out
  } finally { Remove-Item $f -Force -ErrorAction SilentlyContinue }
}

function Test-Damaged([string]$c) {
  if ([string]::IsNullOrWhiteSpace($c)) { return $false }
  if ($c -match '\?') { return $true }
  if ($c -match '[^\x00-\x7F]') {
    # 含非 ASCII，但既不是 CJK 也不是常见全角标点 → 大概是乱码
    if ($c -notmatch '[\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]') { return $true }
  }
  return $false
}

# 只取基表列（视图列不存注释，显示的是引用列的注释，不需要也不应单独修）
$rows = Invoke-Sql @'
SELECT c.TABLE_NAME, c.COLUMN_NAME, c.COLUMN_TYPE, c.IS_NULLABLE,
       IFNULL(c.COLUMN_DEFAULT,''), IFNULL(c.EXTRA,''), IFNULL(c.COLUMN_COMMENT,'')
FROM information_schema.COLUMNS c
JOIN information_schema.TABLES t
  ON t.TABLE_SCHEMA=c.TABLE_SCHEMA AND t.TABLE_NAME=c.TABLE_NAME
WHERE c.TABLE_SCHEMA='fastgame' AND t.TABLE_TYPE='BASE TABLE'
ORDER BY c.TABLE_NAME, c.ORDINAL_POSITION;
'@

# 目标注释来源：优先用"已中文化"的清单（comment_manifest_zh.json），
# 回退到迁移文件提取的清单（comment_manifest.json）。
# 注意：本脚本**只会在目标含中文时改动数据库**，绝不用英文覆盖中文，
# 否则会把已中文化的注释改回英文。
$zhPath  = Join-Path $Root "docker\mysql\migrations\comment_manifest_zh.json"
$migPath = Join-Path $Root "docker\mysql\migrations\comment_manifest.json"
$manifestPath = if (Test-Path $zhPath) { $zhPath } else { $migPath }
Write-Host ("目标注释清单: {0}" -f (Split-Path $manifestPath -Leaf))
$manifest = [System.IO.File]::ReadAllText($manifestPath, $utf8) | ConvertFrom-Json
$manifestCols = @{}
foreach ($p in $manifest.column_comments.PSObject.Properties) { $manifestCols[$p.Name] = $p.Value }
$manifestTables = @{}
foreach ($p in $manifest.table_comments.PSObject.Properties) { $manifestTables[$p.Name] = $p.Value }

# 表注释同样按清单差分（部分表注释也是双重编码乱码，如 pending_transactions）
$tblRows = Invoke-Sql "SELECT TABLE_NAME, IFNULL(TABLE_COMMENT,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA='fastgame' AND TABLE_TYPE='BASE TABLE' ORDER BY TABLE_NAME;"
$tableFixes = @()
foreach ($line in @($tblRows)) {
  if ([string]::IsNullOrWhiteSpace($line)) { continue }
  $p = $line -split "`t"
  $t = $p[0].Trim()
  $cur = if ($p.Count -gt 1) { $p[1] } else { '' }
  if (-not $manifestTables.ContainsKey($t)) { continue }
  if ($cur -eq $manifestTables[$t]) { continue }
  $tableFixes += [pscustomobject]@{ Table = $t; Current = $cur; Fixed = $manifestTables[$t] }
}

# 重建列定义（只改 COMMENT）
function Build-Definition([string]$type, [string]$nullable, [string]$def, [string]$extra, [string]$comment) {
  # DEFAULT_GENERATED 是 MySQL 内部标记，不能写进 MODIFY COLUMN 的 DDL（会报 ERROR 1064）
  $extra = ($extra -replace '\bDEFAULT_GENERATED\b', '').Trim()
  $extra = ($extra -replace '\s+', ' ').Trim()

  $sb = $type
  if ($nullable -eq 'NO') { $sb += ' NOT NULL' } else { $sb += ' NULL' }
  if ($def -ne '' -and $def -ne 'NULL') {
    if ($def -match '^(CURRENT_TIMESTAMP|current_timestamp)') { $sb += " DEFAULT $def" }
    else { $sb += " DEFAULT '$def'" }
  }
  if ($extra -ne '') { $sb += ' ' + $extra }
  $sb += " COMMENT '" + ($comment -replace "'", "''") + "'"
  return $sb
}

$fixes = @()
$unrepairable = @()
foreach ($line in @($rows)) {
  if ([string]::IsNullOrWhiteSpace($line)) { continue }
  $p = $line -split "`t"
  if ($p.Count -lt 7) { continue }
  $t = $p[0].Trim(); $col = $p[1].Trim()
  $cur = $p[6]
  $key = "$t.$col"
  # 判据：与权威清单不一致 → 修复。清单没有该列时，退回启发式判断是否损坏。
  $needsFix = $false
  if ($manifestCols.ContainsKey($key)) {
    if ($cur -ne $manifestCols[$key]) { $needsFix = $true }
  } elseif (Test-Damaged $cur) {
    $unrepairable += $key
    continue
  }
  if (-not $needsFix) { continue }
  # 守卫：目标必须含中文，否则会拿英文清单把已中文化的注释改回英文（回退陷阱）
  if ($manifestCols[$key] -notmatch '[\u4e00-\u9fff]') { continue }
  $fixes += [pscustomobject]@{
    Table = $t; Column = $col; Current = $cur; Fixed = $manifestCols[$key]
    Definition = (Build-Definition $p[2] $p[3] $p[4] $p[5] $manifestCols[$key])
    OldDefinition = (Build-Definition $p[2] $p[3] $p[4] $p[5] $cur)
  }
}

$fixes = @($fixes); $tableFixes = @($tableFixes)
if ($fixes.Count -eq 0 -and $tableFixes.Count -eq 0) { Write-Host "没有需要修复的注释。"; exit 0 }

$fixes = @($fixes | Sort-Object Table, Column)

# ---- 修复脚本 ----
$sb = New-Object System.Text.StringBuilder
[void]$sb.AppendLine("-- 29-repair-column-comments-migration.sql")
[void]$sb.AppendLine("-- 修复被错误 charset 毁掉的表/列注释（ASCII 问号 / 双重编码乱码）。")
[void]$sb.AppendLine("-- 注释内容取自 docker/mysql/migrations/comment_manifest.json（由迁移文件提取）。")
[void]$sb.AppendLine("-- 列定义由 information_schema 重建，只替换 COMMENT，类型/默认值/extra 不变。")
[void]$sb.AppendLine("-- 由 scripts/gen_comment_repair.ps1 生成，请勿手工编辑。")
[void]$sb.AppendLine("USE fastgame;")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("SET NAMES utf8mb4;")
[void]$sb.AppendLine("")
if ($tableFixes.Count -gt 0) {
  [void]$sb.AppendLine("-- 表注释")
  foreach ($t in ($tableFixes | Sort-Object Table)) {
    $esc = $t.Fixed -replace "'", "''"
    [void]$sb.AppendLine("ALTER TABLE ``$($t.Table)`` COMMENT = '$esc';")
  }
}
$curTable = ''
foreach ($f in $fixes) {
  if ($f.Table -ne $curTable) {
    [void]$sb.AppendLine("")
    [void]$sb.AppendLine("-- $($f.Table) 列注释")
    $curTable = $f.Table
  }
  [void]$sb.AppendLine("ALTER TABLE ``$($f.Table)`` MODIFY COLUMN ``$($f.Column)`` $($f.Definition);")
}
[void]$sb.AppendLine("")
[void]$sb.AppendLine("INSERT IGNORE INTO schema_migrations (version, description)")
[void]$sb.AppendLine("VALUES ('29-repair-column-comments', 'Restore Chinese column comments damaged by wrong charset');")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("SELECT '29-repair-column-comments applied' AS note;")
$fixPath = Join-Path $Root "docker\mysql\migrations\29-repair-column-comments-migration.sql"
[System.IO.File]::WriteAllText($fixPath, $sb.ToString(), $enc)

# ---- 回滚脚本 ----
$rb = New-Object System.Text.StringBuilder
[void]$rb.AppendLine("-- 29-repair-column-comments-rollback.sql")
[void]$rb.AppendLine("-- 回滚：把表/列注释恢复为修复前的值（HEX 确认过的原始损坏内容）。")
[void]$rb.AppendLine("-- 由 scripts/gen_comment_repair.ps1 生成。")
[void]$rb.AppendLine("USE fastgame;")
[void]$rb.AppendLine("")
[void]$rb.AppendLine("SET NAMES utf8mb4;")
[void]$rb.AppendLine("")
if ($tableFixes.Count -gt 0) {
  [void]$rb.AppendLine("-- 表注释")
  foreach ($t in ($tableFixes | Sort-Object Table)) {
    $esc = $t.Current -replace "'", "''"
    [void]$rb.AppendLine("ALTER TABLE ``$($t.Table)`` COMMENT = '$esc';")
  }
}
$curTable = ''
foreach ($f in $fixes) {
  if ($f.Table -ne $curTable) {
    [void]$rb.AppendLine("")
    [void]$rb.AppendLine("-- $($f.Table) 列注释")
    $curTable = $f.Table
  }
  [void]$rb.AppendLine("ALTER TABLE ``$($f.Table)`` MODIFY COLUMN ``$($f.Column)`` $($f.OldDefinition);")
}
[void]$rb.AppendLine("")
[void]$rb.AppendLine("DELETE FROM schema_migrations WHERE version = '29-repair-column-comments';")
$rbPath = Join-Path $Root "docker\mysql\migrations\29-repair-column-comments-rollback.sql"
[System.IO.File]::WriteAllText($rbPath, $rb.ToString(), $enc)

# ---- 损坏明细（审计留档）----
$manifestOut = [ordered]@{
  generated_at_note = 'comments damaged by wrong charset at table-creation time'
  total_damaged_columns = $fixes.Count
  total_damaged_tables = $tableFixes.Count
  unrepairable = $unrepairable
  table_details = $tableFixes | ForEach-Object { [ordered]@{ table=$_.Table; before=$_.Current; after=$_.Fixed } }
  column_details = $fixes | ForEach-Object { [ordered]@{ table=$_.Table; column=$_.Column; before=$_.Current; after=$_.Fixed } }
}
[System.IO.File]::WriteAllText((Join-Path $Root "docker\mysql\migrations\comment_damage.json"),
  ($manifestOut | ConvertTo-Json -Depth 6), $enc)

Write-Host ("生成完毕：修复 {0} 个表注释 + {1} 个列注释" -f $tableFixes.Count, $fixes.Count)
if ($unrepairable.Count -gt 0) { Write-Host ("无法修复（清单缺注释）: {0} 处" -f $unrepairable.Count) -ForegroundColor Yellow }
Write-Host "  修复: docker/mysql/migrations/29-repair-column-comments-migration.sql"
Write-Host "  回滚: docker/mysql/migrations/29-repair-column-comments-rollback.sql"
Write-Host "  明细: docker/mysql/migrations/comment_damage.json"
