# gen_comment_translate.ps1 — 生成"注释中文化"迁移
#
# 把 translations_comments.json 里的简体中文译法叠加到已提取的注释清单上，
# 然后与线上库现状做差分，只对"仍是英文/与目标不一致"的注释生成 ALTER。
#
# 设计上与 gen_comment_repair.ps1 一致：
#   - 列定义由 information_schema 的 COLUMN_TYPE/IS_NULLABLE/COLUMN_DEFAULT/EXTRA 重建
#   - 只替换 COMMENT，不改类型、默认值、auto_increment、on update
#   - 同时产出可回滚脚本
#
# 用法:
#   powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_translate.ps1
# 输出:
#   docker/mysql/migrations/30-translate-comments-migration.sql
#   docker/mysql/migrations/30-translate-comments-rollback.sql
#   docker/mysql/migrations/comment_manifest_zh.json   （合并后的权威清单：迁移文件 + 译文）

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

function Test-HasCJK([string]$s) { return ($s -match '[\u4e00-\u9fff]') }

# ---------- 1. 读入迁移文件清单 + 译文，合并出目标状态 ----------
$migPath = Join-Path $Root "docker\mysql\migrations\comment_manifest.json"
$trPath  = Join-Path $Root "docker\mysql\migrations\translations_comments.json"
$cmPath  = Join-Path $Root "docker\mysql\migrations\translations_common_columns.json"
$mig = [System.IO.File]::ReadAllText($migPath, $utf8) | ConvertFrom-Json
$tr  = [System.IO.File]::ReadAllText($trPath,  $utf8) | ConvertFrom-Json
$cm  = [System.IO.File]::ReadAllText($cmPath,  $utf8) | ConvertFrom-Json

$targetTables = @{}
foreach ($p in $mig.table_comments.PSObject.Properties)  { $targetTables[$p.Name] = $p.Value }
foreach ($p in $tr.table_comments.PSObject.Properties)   { if ($p.Name -notmatch '^_') { $targetTables[$p.Name] = $p.Value } }   # 译文覆盖迁移文件里的英文；_note 等元数据键跳过

$targetCols = @{}
foreach ($p in $mig.column_comments.PSObject.Properties) { $targetCols[$p.Name] = $p.Value }
foreach ($p in $tr.column_comments.PSObject.Properties)  { if ($p.Name -notmatch '^_') { $targetCols[$p.Name] = $p.Value } }

# 通用列名词典：仅用于"当前完全没有注释"的列，按列名统一补注释
$commonByName = @{}
foreach ($p in $cm.by_name.PSObject.Properties) { if ($p.Name -notmatch '^_') { $commonByName[$p.Name] = $p.Value } }

# 译文表里若出现拼写错误的键，这里会暴露出来（键不在实际表结构中）
$liveColKeys = @{}
$liveTblKeys = @{}
foreach ($line in @(Invoke-Sql "SELECT TABLE_NAME, COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='fastgame' ORDER BY TABLE_NAME, ORDINAL_POSITION;")) {
  if ([string]::IsNullOrWhiteSpace($line)) { continue }
  $p = $line -split "`t"
  $liveColKeys["$($p[0]).$($p[1])"] = $true
  $liveTblKeys[$p[0]] = $true
}
$badKeys = @()
foreach ($k in $tr.table_comments.PSObject.Properties.Name)  { if (-not $liveTblKeys.ContainsKey($k)) { $badKeys += "table:$k" } }
foreach ($k in $tr.column_comments.PSObject.Properties.Name) { if (-not $liveColKeys.ContainsKey($k)) { $badKeys += "column:$k" } }
if ($badKeys.Count -gt 0) {
  Write-Host "错误：译文表里存在数据库中没有的表/列名：" -ForegroundColor Red
  $badKeys | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
  exit 1
}

# ---------- 2. 与线上现状差分 ----------
$tblRows = Invoke-Sql "SELECT TABLE_NAME, IFNULL(TABLE_COMMENT,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA='fastgame' AND TABLE_TYPE='BASE TABLE' ORDER BY TABLE_NAME;"
$colRows = Invoke-Sql @'
SELECT c.TABLE_NAME, c.COLUMN_NAME, c.COLUMN_TYPE, c.IS_NULLABLE,
       IFNULL(c.COLUMN_DEFAULT,''), IFNULL(c.EXTRA,''), IFNULL(c.COLUMN_COMMENT,'')
FROM information_schema.COLUMNS c
JOIN information_schema.TABLES t ON t.TABLE_SCHEMA=c.TABLE_SCHEMA AND t.TABLE_NAME=c.TABLE_NAME
WHERE c.TABLE_SCHEMA='fastgame' AND t.TABLE_TYPE='BASE TABLE'
ORDER BY c.TABLE_NAME, c.ORDINAL_POSITION;
'@

function Build-Definition([string]$type, [string]$nullable, [string]$def, [string]$extra, [string]$comment) {
  # EXTRA 里的 DEFAULT_GENERATED 是 MySQL 内部标记，不能出现在 MODIFY COLUMN 的 DDL 中
  # （带上会报 ERROR 1064）。只保留 auto_increment 与 on update CURRENT_TIMESTAMP。
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

$tableFixes = @()
foreach ($line in @($tblRows)) {
  if ([string]::IsNullOrWhiteSpace($line)) { continue }
  $p = $line -split "`t"
  $t = $p[0].Trim()
  $cur = if ($p.Count -gt 1) { $p[1] } else { '' }
  if (-not $targetTables.ContainsKey($t)) { continue }
  if ($cur -eq $targetTables[$t]) { continue }
  # 目标必须是中文；当前为空（补注释）或为英文（翻译）都处理；已有中文则跳过，避免与 29 号修复迁移重复
  if (-not (Test-HasCJK $targetTables[$t])) { continue }
  if (-not [string]::IsNullOrWhiteSpace($cur) -and (Test-HasCJK $cur)) { continue }
  $tableFixes += [pscustomobject]@{ Table = $t; Current = $cur; Target = $targetTables[$t] }
}

$colFixes = @()
foreach ($line in @($colRows)) {
  if ([string]::IsNullOrWhiteSpace($line)) { continue }
  $p = $line -split "`t"
  if ($p.Count -lt 7) { continue }
  $t = $p[0].Trim(); $col = $p[1].Trim(); $cur = $p[6]
  $key = "$t.$col"

  # 目标注释的来源优先级：精确译法/迁移文件 > 通用列名词典（仅当当前无注释）
  $target = $null
  if ($targetCols.ContainsKey($key)) {
    $target = $targetCols[$key]
  } elseif ([string]::IsNullOrWhiteSpace($cur) -and $commonByName.ContainsKey($col)) {
    $target = $commonByName[$col]
  }

  if (-not $target) { continue }
  if ($cur -eq $target) { continue }
  if (-not (Test-HasCJK $target)) { continue }
  if (Test-HasCJK $cur) { continue }
  $colFixes += [pscustomobject]@{
    Table = $t; Column = $col; Current = $cur; Target = $target
    Definition    = (Build-Definition $p[2] $p[3] $p[4] $p[5] $target)
    OldDefinition = (Build-Definition $p[2] $p[3] $p[4] $p[5] $cur)
  }
}

# 注意：单个元素时 PowerShell 会把数组展平成标量，.Count 取不到，必须用 @() 包住
$tableFixes = @($tableFixes)
$colFixes   = @($colFixes)

if ($tableFixes.Count -eq 0 -and $colFixes.Count -eq 0) {
  Write-Host "所有表/列注释都已是简体中文，无需翻译。"
  exit 0
}

# 管道会吃掉单元素数组的数组性，必须再用 @() 包一层，否则 .Count 失效
$tableFixes = @($tableFixes | Sort-Object Table)
$colFixes   = @($colFixes   | Sort-Object Table, Column)

# ---------- 3. 生成迁移 ----------
$sb = New-Object System.Text.StringBuilder
[void]$sb.AppendLine("-- 30-translate-comments-migration.sql")
[void]$sb.AppendLine("-- 把英文的表/列注释改为简体中文（枚举值与专有名词保留英文）。")
[void]$sb.AppendLine("-- 译法定义在 docker/mysql/migrations/translations_comments.json。")
[void]$sb.AppendLine("-- 列定义由 information_schema 重建，只替换 COMMENT，类型/默认值/extra 不变。")
[void]$sb.AppendLine("-- 由 scripts/gen_comment_translate.ps1 生成，请勿手工编辑。")
[void]$sb.AppendLine("USE fastgame;")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("SET NAMES utf8mb4;")
[void]$sb.AppendLine("")
if ($tableFixes.Count -gt 0) {
  [void]$sb.AppendLine("-- 表注释")
  foreach ($t in $tableFixes) {
    $esc = $t.Target -replace "'", "''"
    [void]$sb.AppendLine("ALTER TABLE ``$($t.Table)`` COMMENT = '$esc';")
  }
  [void]$sb.AppendLine("")
}
foreach ($g in ($colFixes | Group-Object Table)) {
  [void]$sb.AppendLine("-- $($g.Name) 列注释")
  foreach ($f in $g.Group) {
    [void]$sb.AppendLine("ALTER TABLE ``$($f.Table)`` MODIFY COLUMN ``$($f.Column)`` $($f.Definition);")
  }
  [void]$sb.AppendLine("")
}
[void]$sb.AppendLine("INSERT IGNORE INTO schema_migrations (version, description)")
[void]$sb.AppendLine("VALUES ('30-translate-comments', 'Translate English table/column comments into Simplified Chinese');")
[void]$sb.AppendLine("")
[void]$sb.AppendLine("SELECT '30-translate-comments applied' AS note;")
$fixPath = Join-Path $Root "docker\mysql\migrations\30-translate-comments-migration.sql"
[System.IO.File]::WriteAllText($fixPath, $sb.ToString(), $enc)

# ---------- 4. 回滚 ----------
$rb = New-Object System.Text.StringBuilder
[void]$rb.AppendLine("-- 30-translate-comments-rollback.sql")
[void]$rb.AppendLine("-- 回滚：把表/列注释恢复为翻译前的英文。")
[void]$rb.AppendLine("-- 由 scripts/gen_comment_translate.ps1 生成。")
[void]$rb.AppendLine("USE fastgame;")
[void]$rb.AppendLine("")
[void]$rb.AppendLine("SET NAMES utf8mb4;")
[void]$rb.AppendLine("")
if ($tableFixes.Count -gt 0) {
  [void]$rb.AppendLine("-- 表注释")
  foreach ($t in $tableFixes) {
    $esc = $t.Current -replace "'", "''"
    [void]$rb.AppendLine("ALTER TABLE ``$($t.Table)`` COMMENT = '$esc';")
  }
  [void]$rb.AppendLine("")
}
foreach ($g in ($colFixes | Group-Object Table)) {
  [void]$rb.AppendLine("-- $($g.Name) 列注释")
  foreach ($f in $g.Group) {
    [void]$rb.AppendLine("ALTER TABLE ``$($f.Table)`` MODIFY COLUMN ``$($f.Column)`` $($f.OldDefinition);")
  }
  [void]$rb.AppendLine("")
}
[void]$rb.AppendLine("DELETE FROM schema_migrations WHERE version = '30-translate-comments';")
$rbPath = Join-Path $Root "docker\mysql\migrations\30-translate-comments-rollback.sql"
[System.IO.File]::WriteAllText($rbPath, $rb.ToString(), $enc)

# ---------- 5. 合并清单（迁移文件 + 译文 = 目标状态），供文档与后续校验使用 ----------
$merged = [ordered]@{
  generated_from = 'docker/mysql/init/*.sql + translations_comments.json'
  table_comments  = $targetTables
  column_comments = $targetCols
}
[System.IO.File]::WriteAllText((Join-Path $Root "docker\mysql\migrations\comment_manifest_zh.json"),
  ($merged | ConvertTo-Json -Depth 5), $enc)

Write-Host ("生成完毕：{0} 个表注释 + {1} 个列注释需要中文化" -f $tableFixes.Count, $colFixes.Count)
Write-Host "  迁移: docker/mysql/migrations/30-translate-comments-migration.sql"
Write-Host "  回滚: docker/mysql/migrations/30-translate-comments-rollback.sql"
Write-Host "  清单: docker/mysql/migrations/comment_manifest_zh.json"
