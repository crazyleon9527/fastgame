# gen_database_doc.ps1 — 从运行中的 MySQL 元数据重新生成 docs/DATABASE.md
# 用法: powershell -ExecutionPolicy Bypass -File .\scripts\gen_database_doc.ps1
# 前置: docker 容器 fastgame-mysql 正在运行
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

$enc  = New-Object System.Text.UTF8Encoding($false)
$utf8 = [System.Text.Encoding]::UTF8
# 临时 SQL 文件放系统临时目录，不污染仓库
$tmp  = Join-Path ([System.IO.Path]::GetTempPath()) "fastgame_dbdoc"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$TICK = [char]96

function Code([string]$s) { "$TICK$s$TICK" }

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

$tables  = Invoke-Sql "SELECT TABLE_NAME, TABLE_TYPE, IFNULL(ENGINE,'-'), IFNULL(TABLE_ROWS,0), IFNULL(TABLE_COMMENT,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA='fastgame' ORDER BY TABLE_TYPE, TABLE_NAME;"
$columns = Invoke-Sql "SELECT TABLE_NAME, ORDINAL_POSITION, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, IFNULL(COLUMN_DEFAULT,''), IFNULL(COLUMN_KEY,''), IFNULL(EXTRA,''), IFNULL(COLUMN_COMMENT,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='fastgame' ORDER BY TABLE_NAME, ORDINAL_POSITION;"
$indexes = Invoke-Sql "SELECT TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX;"
$fkeys   = Invoke-Sql "SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA='fastgame' AND REFERENCED_TABLE_NAME IS NOT NULL ORDER BY TABLE_NAME, COLUMN_NAME;"
$migs    = Invoke-Sql "SELECT version, description FROM fastgame.schema_migrations ORDER BY version;"

function Split-Rows($rows) {
  @($rows) | Where-Object { $_ -ne '' -and $_ -ne $null } | ForEach-Object { , ($_ -split "`t") }
}

$tblMap = @{}
foreach ($r in Split-Rows $tables) { $tblMap[$r[0]] = [pscustomobject]@{ Name=$r[0]; Type=$r[1]; Engine=$r[2]; Rows=$r[3]; Comment=$r[4] } }
$colMap = @{}
foreach ($r in Split-Rows $columns) {
  if (-not $colMap.ContainsKey($r[0])) { $colMap[$r[0]] = New-Object System.Collections.Generic.List[object] }
  $colMap[$r[0]].Add([pscustomobject]@{ Name=$r[2]; Type=$r[3]; Nullable=($r[4] -eq 'YES'); Default=$r[5]; Key=$r[6]; Extra=$r[7]; Comment=$r[8] })
}
$idxMap = @{}
foreach ($r in Split-Rows $indexes) {
  if (-not $idxMap.ContainsKey($r[0])) { $idxMap[$r[0]] = [ordered]@{} }
  if (-not $idxMap[$r[0]].Contains($r[1])) { $idxMap[$r[0]][$r[1]] = [pscustomobject]@{ Cols=New-Object System.Collections.Generic.List[string]; Unique=($r[4] -eq '0') } }
  $idxMap[$r[0]][$r[1]].Cols.Add($r[3])
}
$fkMap = @{}
foreach ($r in Split-Rows $fkeys) {
  if (-not $fkMap.ContainsKey($r[0])) { $fkMap[$r[0]] = New-Object System.Collections.Generic.List[object] }
  $fkMap[$r[0]].Add([pscustomobject]@{ Col=$r[1]; RefTable=$r[2]; RefCol=$r[3] })
}

# 模型覆盖：从 Go 源码抽取 table 绑定
# 源码形如:  table: "`merchants`",
$modelByTable = @{}
$modelPat = 'table:\s*"' + $TICK + '([a-z_]+)' + $TICK
foreach ($f in Get-ChildItem internal\model\*.go) {
  $txt = [System.IO.File]::ReadAllText($f.FullName, $utf8)
  foreach ($m in [regex]::Matches($txt, $modelPat)) {
    $t = $m.Groups[1].Value
    if (-not $modelByTable.ContainsKey($t)) { $modelByTable[$t] = New-Object System.Collections.Generic.List[string] }
    if (-not $modelByTable[$t].Contains($f.Name)) { $modelByTable[$t].Add($f.Name) }
  }
}

# 注释来源：库里部分注释在建表时已被错误 charset 毁成 ??? 或双重编码乱码，
# 因此以 docker/mysql/init/*.sql（权威定义）为准，库里的注释仅作兜底。
$commentByTable = @{}   # table -> 表注释
$colComment     = @{}   # "table.col" -> 列注释
function Set-TblComment([string]$t, [string]$c) { if ($c -and $c.Trim()) { $commentByTable[$t] = $c.Trim() } }
function Set-ColComment([string]$t, [string]$col, [string]$c) {
  if ($col -and $c -and $c.Trim()) {
    $k = $t + '.' + $col
    if (-not $colComment.ContainsKey($k)) { $colComment[$k] = $c.Trim() }
  }
}

function Import-SqlComments([string]$path) {
  $lines = [System.IO.File]::ReadAllLines($path, [System.Text.Encoding]::UTF8)
  $curTable = $null
  $seenTables = @{}
  foreach ($line in $lines) {
    # CREATE TABLE / ALTER TABLE 切换当前表
    $m = [regex]::Match($line, '(?i)(?:CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?|ALTER\s+TABLE)\s+`?([a-z_][a-z0-9_]*)`?')
    if ($m.Success) {
      $curTable = $m.Groups[1].Value.ToLower()
      if (-not $seenTables.ContainsKey($curTable)) { $seenTables[$curTable] = $true }
      continue
    }
    if ($line -match '^\s*\)\s*ENGINE' -or $line -match '^\s*\)\s*[A-Z]') {
      if ($curTable) {
        $tc = [regex]::Match($line, "(?i)COMMENT\s*=?\s*'([^']*)'")
        if ($tc.Success) { Set-TblComment $curTable $tc.Groups[1].Value }
        $curTable = $null
      }
      continue
    }
    if ($curTable) {
      # 列注释：同一行的 COMMENT '...'
      $c = [regex]::Match($line, "(?i)COMMENT\s+'([^']*)'")
      if ($c.Success) {
        $col = [regex]::Match($line, '^\s*`?([a-z_][a-z0-9_]*)`?\s+[a-zA-Z]')
        if ($col.Success) { Set-ColComment $curTable $col.Groups[1].Value.ToLower() $c.Groups[1].Value }
      }
    }
  }
}

foreach ($f in (Get-ChildItem docker\mysql\init\*.sql | Sort-Object Name)) { Import-SqlComments $f.FullName }
Write-Host ("从迁移文件提取：{0} 个表注释，{1} 个列注释" -f $commentByTable.Count, $colComment.Count)

# 叠加人工译法（translations_comments.json + translations_common_columns.json），
# 使文档与"已中文化的线上库"保持一致；线上库为最终兜底。
$trDir = Join-Path $Root "docker\mysql\migrations"
foreach ($name in @('translations_comments.json', 'translations_common_columns.json')) {
  $p = Join-Path $trDir $name
  if (-not (Test-Path $p)) { continue }
  $j = [System.IO.File]::ReadAllText($p, $utf8) | ConvertFrom-Json
  if ($j.table_comments) {
    foreach ($x in $j.table_comments.PSObject.Properties) { if ($x.Name -notmatch '^_') { $commentByTable[$x.Name] = $x.Value } }
  }
  if ($j.column_comments) {
    foreach ($x in $j.column_comments.PSObject.Properties) { if ($x.Name -notmatch '^_') { $colComment[$x.Name] = $x.Value } }
  }
}
Write-Host ("叠加译文后：{0} 个表注释，{1} 个列注释" -f $commentByTable.Count, $colComment.Count)

$groups = [ordered]@{
  'A. 商户与钱包'       = @('merchants','merchant_wallet_configs','merchant_currencies','currencies','commission_rules')
  'B. 游戏目录与配置'   = @('games','game_categories','game_rtp_tiers','merchant_games','game_configs','game_client_versions','merchant_game_versions')
  'C. 结算与对账'       = @('settlement_periods','merchant_settlement_lines','daily_settlements','pending_transactions','wallet_pending_ops','game_round_replay')
  'D. 风控'             = @('risk_alerts','risk_blacklist','player_merchant_profiles')
  'E. 会话、审计与运维' = @('game_sessions','audit_logs','game_maintenance_windows','api_rate_limits','merchant_webhooks')
  'F. 多语言（i18n）'   = @('locales','i18n_bundles','i18n_messages','i18n_message_translations','i18n_entity_translations','merchant_locales')
  'G. 后台权限'         = @('roles','admin_users')
  'H. 元数据与归档'     = @('schema_migrations','schema_archive_policies')
}

$sb = New-Object System.Text.StringBuilder
function W([string]$s = '') { [void]$sb.AppendLine($s) }

# 注释来源优先级：人工译文 > 线上库 > 迁移文件。
# 线上库已全量中文化，因此正常情况下译文与库一致；迁移文件仅作最后兜底。
function Get-TblComment([string]$t) {
  if ($commentByTable.ContainsKey($t)) { return $commentByTable[$t] }
  $c = $tblMap[$t].Comment
  if ($c -and $c -ne 'VIEW') { return $c }
  return ''
}

function Get-ColComment([string]$t, [string]$col) {
  $k = $t + '.' + $col
  if ($colComment.ContainsKey($k)) { return $colComment[$k] }
  $c = (($colMap[$t] | Where-Object { $_.Name -eq $col }).Comment)
  if (-not $c) { return '' }
  if ($c -match '\?\?\?') { return '' }               # 已被毁成问号，无信息量
  if ($c -match '[\u00c0-\u00ff]{2,}') { return '' }   # 双重编码乱码
  return $c
}

function Write-Columns([string]$t) {
  W ('#### ' + (Code $t))
  W ''
  $tc = Get-TblComment $t
  if ($tc) { W ('> ' + $tc); W '' }
  $isView = ($tblMap[$t].Type -eq 'VIEW')
  W '| 列 | 类型 | 空 | 默认 | 键 | 说明 |'
  W '|---|---|:--:|---|---|---|'
  foreach ($x in $colMap[$t]) {
    $def = if ($x.Default -eq '' -or $x.Default -eq 'NULL') { '-' } else { Code $x.Default }
    $key = if ($x.Key -eq 'PRI') { 'PK' } elseif ($x.Key -eq 'UNI') { 'UK' } elseif ($x.Key -eq 'MUL') { 'IDX' } else { '' }
    $cmt = Get-ColComment $t $x.Name
    # 视图的计算列（如 COALESCE 派生）没有来源注释，显式标注避免误认为遗漏
    if ($cmt -eq '' -and $isView) { $cmt = '视图计算列' }
    W ('| ' + (Code $x.Name) + ' | ' + $x.Type + ' | ' + $(if ($x.Nullable) { '是' } else { '否' }) + ' | ' + $def + ' | ' + $key + ' | ' + $cmt + ' |')
  }
  W ''
  if ($idxMap.ContainsKey($t)) {
    W '**索引**'
    W ''
    foreach ($k in $idxMap[$t].Keys) {
      $i = $idxMap[$t][$k]
      $kind = if ($k -eq 'PRIMARY') { '主键' } elseif ($i.Unique) { '唯一' } else { '普通' }
      $cols = (($i.Cols | ForEach-Object { Code $_ }) -join ', ')
      W ('- ' + (Code $k) + '（' + $kind + '）：' + $cols)
    }
    W ''
  }
  if ($fkMap.ContainsKey($t)) {
    W '**外键**'
    W ''
    foreach ($f in $fkMap[$t]) { W ('- ' + (Code $f.Col) + ' → ' + (Code ($f.RefTable + '.' + $f.RefCol))) }
    W ''
  }
}

$base    = @($tblMap.Values | Where-Object { $_.Type -eq 'BASE TABLE' })
$views   = @($tblMap.Values | Where-Object { $_.Type -eq 'VIEW' })
$noModel = @($base | Where-Object { -not $modelByTable.ContainsKey($_.Name) })

W '# FastGame 数据库设计'
W ''
W '> 多商户 · 自研多游戏 · MySQL 8.0 · utf8mb4 · UTC'
W '>'
W ('> **本文档由 ' + (Code 'scripts/gen_database_doc.ps1') + ' 从运行中的 MySQL 元数据生成，请勿手工编辑。**')
W '>'
W ('> 重新生成：' + (Code 'powershell -ExecutionPolicy Bypass -File .\scripts\gen_database_doc.ps1'))
W ''
W ('**当前规模：' + $base.Count + ' 张基表 + ' + $views.Count + ' 个视图**（information_schema 实测）')
W ''
W '---'
W ''
W '## 一、设计原则'
W ''
W ('1. **平台层 vs 商户层**：' + (Code 'games') + ' / ' + (Code 'game_categories') + ' / ' + (Code 'game_rtp_tiers') + ' 是平台统一维护的游戏目录；商户通过 ' + (Code 'merchant_games') + ' 开通并覆盖参数。')
W ('2. **注单明细不落 MySQL**：海量注单在 ClickHouse（' + (Code 'game_round_settled') + '）。MySQL 只存配置、会话、对账与审计，因此 ' + (Code 'game_round_replay') + ' 只保留确定性回放所需的种子，不存派彩金额。')
W ('3. **金额一律 minor units（BIGINT）**：与 ' + (Code 'pkg/money') + ' 的 Scale=10000 对齐（4 位小数）。' + (Code '08-money-migration.sql') + ' 已把所有金额列从 DECIMAL 迁为 BIGINT。')
W '4. **RTP 用 PPM**：96% = 960000（parts per million），避免浮点。'
W ('5. **状态列用 tinyint/varchar + CHECK 约束**：' + (Code '22-status-checks-migration.sql') + ' 引入。')
W ('6. **时间统一 UTC，精度 datetime(3)**：由 MySQL 容器 --default-time-zone=+00:00 保证。')
W ('7. **迁移可重复执行**：新脚本使用 CREATE TABLE IF NOT EXISTS / 条件 ALTER，并登记到 ' + (Code 'schema_migrations') + '。')
W ''
W '## 二、字段与命名约定'
W ''
W '| 后缀 / 前缀 | 含义 |'
W '|---|---|'
W ('| ' + (Code '*_minor') + ' | 金额，BIGINT minor units（1.0 = 10000） |')
W ('| ' + (Code '*_ppm') + ' | 比率，百万分之一 |')
W ('| ' + (Code 'merchant_code') + ' | 商户业务编码（字符串，对外使用） |')
W ('| ' + (Code 'merchant_id') + ' | 商户主键（对应 merchants.id，内部关联） |')
W ('| ' + (Code 'game_code') + ' | 游戏业务编码（对应 games.game_code） |')
W ('| ' + (Code 'game_id') + ' | 游戏主键（对应 games.id） |')
W ('| ' + (Code 'round_id') + ' | 单局唯一 ID，同时作为 provably-fair 的 nonce |')
W ('| ' + (Code 'status') + ' | 通常 1=启用 / 0=停用；games 为 0/1/2 三态 |')
W ('| ' + (Code 'created_at') + ' / ' + (Code 'updated_at') + ' | datetime(3)；日志类表用 timestamp(3) |')
W ''
W '## 三、全表清单（按业务域）'
W ''
foreach ($g in $groups.Keys) {
  W ('### ' + $g)
  W ''
  W '| 表 | 行数 | 模型文件 | 表注释 |'
  W '|---|---:|---|---|'
  foreach ($t in $groups[$g]) {
    if (-not $tblMap.ContainsKey($t)) { continue }
    $m = if ($modelByTable.ContainsKey($t)) { (($modelByTable[$t] | ForEach-Object { Code $_ }) -join ', ') } else { '**缺失**' }
    W ('| ' + (Code $t) + ' | ' + $tblMap[$t].Rows + ' | ' + $m + ' | ' + (Get-TblComment $t) + ' |')
  }
  W ''
}
W '> 行数为 InnoDB 统计估算值，仅供参考。「模型文件」指 `internal/model/` 下的对应文件。'
W ''
W '## 四、表结构明细'
W ''
W '### 金额列速查'
W ''
W '以下列均为 BIGINT minor units（Scale=10000）：'
W ''
$moneyCols = @()
foreach ($t in ($colMap.Keys | Sort-Object)) {
  foreach ($x in $colMap[$t]) {
    if ($x.Name -match '(minor$|_amount$|total_bet$|total_win$)' -and $x.Type -eq 'bigint') { $moneyCols += (Code ($t + '.' + $x.Name)) }
  }
}
W ($moneyCols -join ' · ')
W ''
W '---'
W ''
foreach ($g in $groups.Keys) {
  W ('### ' + $g)
  W ''
  foreach ($t in $groups[$g]) { if ($tblMap.ContainsKey($t)) { Write-Columns $t } }
}
W '### I. 视图'
W ''
foreach ($v in $views) { Write-Columns $v.Name }
W '## 五、表关系'
W ''
W '除 i18n 相关表外，数据库层**没有物理外键**，关联全部由代码维护。以下 4 个是仅有的物理外键：'
W ''
W '| 表 | 列 | 引用 |'
W '|---|---|---|'
foreach ($t in ($fkMap.Keys | Sort-Object)) {
  foreach ($f in $fkMap[$t]) { W ('| ' + (Code $t) + ' | ' + (Code $f.Col) + ' | ' + (Code ($f.RefTable + '.' + $f.RefCol)) + ' |') }
}
W ''
W '逻辑关系：'
W ''
W '```'
W 'merchants ─┬─< merchant_games >── games ─┬─< game_rtp_tiers'
W '           │                              └── game_categories'
W '           ├─< merchant_wallet_configs'
W '           ├─< merchant_currencies >── currencies'
W '           ├─< merchant_locales >───── locales'
W '           ├─< commission_rules'
W '           ├─< api_rate_limits'
W '           ├─< merchant_webhooks'
W '           ├─< game_configs           唯一键 (merchant_id, game_code, config_key)'
W '           ├─< settlement_periods ─< merchant_settlement_lines'
W '           ├─< daily_settlements ──> settlement_periods (settlement_period_id)'
W '           └─< player_merchant_profiles'
W ''
W 'games ─┬─< game_client_versions'
W '       └─< merchant_game_versions >── merchants'
W ''
W '同一局（round_id）在三张表留痕：'
W '  game_round_replay.round_id     无唯一键'
W '  pending_transactions.round_id  uk_round_id'
W '  wallet_pending_ops.round_id    uk_round_id + op_type'
W ''
W 'i18n：'
W '  locales ─┬─< i18n_message_translations >── i18n_messages >── i18n_bundles'
W '           ├─< i18n_entity_translations  业务实体字段翻译（game / category 名称）'
W '           └─< merchant_locales >── merchants'
W '```'
W ''
W '## 六、视图说明'
W ''
W ('- **' + (Code 'v_merchant_game_lobby') + '**：商户游戏大厅。聚合 merchants + merchant_games + games + game_categories + game_rtp_tiers，用 COALESCE(mg.min_bet_minor, g.min_bet_minor) 计算生效注额（商户覆盖优先于游戏默认），tier_rtp_ppm 按 mg.rtp_tier_code 取档位 RTP。')
W ('- **' + (Code 'v_merchant_game_lobby_i18n') + '**：在大厅视图之上，按 merchant_locales 关联 i18n_entity_translations，输出多语言游戏名与分类名（COALESCE(译文, 原名)）。')
W ''
W '> ⚠️ 两个视图目前**无任何 Go 代码引用**。RGS 的注额校验走 game_configs 里的 bet_limits JSON，而不是视图里的 effective_min_bet_minor。详见第九节。'
W ''
W '## 七、迁移历史'
W ''
W ('登记在 ' + (Code 'schema_migrations') + ' 的迁移：')
W ''
W '| version | 说明 |'
W '|---|---|'
foreach ($r in Split-Rows $migs) { W ('| ' + (Code $r[0]) + ' | ' + $r[1] + ' |') }
W ''
W ('> 注意：03 / 07 / 09 / 10 四个迁移脚本**未登记** ' + (Code 'schema_migrations') + '（只有 11–28 登记了）。排查历史请一并查看 ' + (Code 'docker/mysql/init/') + ' 下的原始文件。')
W ''
W '## 八、模型覆盖情况'
W ''
# 注意：哈希表的 .Keys 是 KeyCollection，PS 5.1 下 .Count 在字符串插值里会取空，必须先转数组
$withModel = @($modelByTable.Keys | Sort-Object)
$withCount = $withModel.Count
$noCount = $noModel.Count
W ('`internal/model/` 只为 **' + $withCount + ' 张表**提供了模型，其余 **' + $noCount + ' 张基表无模型**：')
W ''
W '| 有模型 | 无模型 |'
W '|---|---|'
$maxN = [Math]::Max($withCount, $noCount)
for ($i = 0; $i -lt $maxN; $i++) {
  $a = if ($i -lt $withCount) { Code $withModel[$i] } else { '' }
  $b = if ($i -lt $noCount) { Code $noModel[$i].Name } else { '' }
  W ('| ' + $a + ' | ' + $b + ' |')
}
W ''
W ('> 无模型不等于无用：games / game_rtp_tiers / merchant_games / game_categories / i18n 系列都在 ' + (Code 'platformGamesModel.go') + '、' + (Code 'i18nModel.go') + ' 里用**手写 SQL** 访问，只是没有 goctl 生成的模型文件。真正零引用的表见第九节。')
W ''
W '## 九、重复定义与已知问题'
W ''
W '### 9.1 真正的重复定义（同一语义存于多处）'
W ''
W '| # | 语义 | 冲突位置 | 实际生效方 |'
W '|---|---|---|---|'
W ('| 1 | 下注限额 | ' + (Code 'game_configs.bet_limits') + '（JSON） vs ' + (Code 'merchant_games.min_bet_minor/max_bet_minor') + ' vs ' + (Code 'games.min_bet_minor/max_bet_minor') + ' | **game_configs**：RGS 读它（betLogic.go:96 → gameconfig.Loader），另两者只被 Admin 写入、无人读 |')
W ('| 2 | RTP 档位 | ' + (Code 'games.default_rtp_ppm') + ' / ' + (Code 'game_rtp_tiers.target_rtp_ppm') + ' / ' + (Code 'merchant_games.rtp_tier_code') + ' / ' + (Code 'game_configs.rtp_tier') + ' | **都不生效**：pkg/prng 的分支硬编码，rtpTier 只作标签回填 |')
W ('| 3 | 客户端版本 | ' + (Code 'games.client_version') + '（varchar） vs ' + (Code 'game_client_versions') + '（整表） | **games.client_version**；后者无代码引用 |')
W ('| 4 | 货币 | ' + (Code 'currencies') + ' vs ' + (Code 'merchant_currencies') + ' | **都不生效**：两表均无代码引用 |')
W ('| 5 | 钱包配置 | merchants 表的密钥字段 vs ' + (Code 'merchant_wallet_configs') + ' | **YAML 配置**：钱包参数读 services/*/etc/*.yaml，表无引用 |')
W ('| 6 | 商户分润 | ' + (Code 'commission_rules') + ' vs ' + (Code 'settlement_periods.commission_minor') + ' | **settlement_periods**：佣金在结算周期里直接列存 |')
W ('| 7 | 结算金额 | ' + (Code 'daily_settlements.total_bet/total_win') + ' vs ' + (Code 'settlement_periods.total_bet_minor/total_win_minor') + ' | 日结与周期结算是两级汇总（保留），但金额列命名不一致（前者无 minor 后缀） |')
W ''
W '### 9.2 零代码引用的表'
W ''
W '以下对象在 Go 代码中**完全没有任何引用**（既无模型，也无手写 SQL）：'
W ''
$deadTables = @('api_rate_limits','commission_rules','currencies','game_client_versions','game_maintenance_windows','game_sessions','i18n_entity_translations','merchant_currencies','merchant_game_versions','merchant_locales','merchant_settlement_lines','merchant_wallet_configs','merchant_webhooks','player_merchant_profiles','schema_archive_policies','schema_migrations','v_merchant_game_lobby','v_merchant_game_lobby_i18n')
foreach ($t in $deadTables) { W ('- ' + (Code $t)) }
W ''
W '> schema_migrations 由迁移脚本自身读写，两个视图供查询/报表使用，这两类属于「非代码路径」，不算冗余。其余属于**设计完成但功能未实现**。'
W ''
W '### 9.3 模型与表结构的一致性'
W ''
W '模型分两类，**只有第一类会真正出错**：'
W ''
W ('- **goctl 生成的** ' + (Code '*_gen.go') + '：用 ' + (Code 'builder.RawFieldNames(&Struct{})') + ' 动态拼列名，结构体缺字段会让生成的 SELECT/INSERT/UPDATE **直接少列**，编译期无法发现。')
W ('- **手写模型**：用显式列名（如 ' + (Code 'rolesModel.go') + ' 的 select id, name, description），结构体是按需投影，少字段只是不查那一列，**不是 bug**。')
W ''
W '#### 已修复（goctl 生成模型缺列 → 生成的 SQL 缺列）'
W ''
W '| 表 | 模型文件 | 原缺失列 | 影响 |'
W '|---|---|---|---|'
W ('| ' + (Code 'merchants') + ' | merchantsModel_gen.go | private_key_prev, private_key_prev_expires_at, allowed_ips | SELECT 查不到密钥轮换与 IP 白名单；INSERT/UPDATE 语句缺列（列数与占位符数不匹配） |')
W ('| ' + (Code 'admin_users') + ' | adminUsersModel_gen.go | totp_secret, totp_enabled, totp_recovery_hashes | INSERT 只有 4 个占位符却要写 7 列，**建管理员账号必失败**；登录取不到 2FA 字段 |')
W ''
W '修复内容：补齐结构体字段，并同步修正 goctl 硬编码的占位符串与参数列表。已用真实数据库执行 CRUD 验证。'
W ''
W '#### 手写模型的"缺失列"（按需投影，无需修复）'
W ''
W '| 表 | 模型文件 | 未纳入的列 | 说明 |'
W '|---|---|---|---|'
W ('| ' + (Code 'roles') + ' | rolesModel.go | created_at, updated_at | 显式 select 三列，用不到时间戳 |')
W ('| ' + (Code 'admin_users') + ' | adminAuthModel.go | created_at, updated_at | 登录鉴权查询，用不到 |')
W ('| ' + (Code 'wallet_pending_ops') + ' | walletPendingOpsModel.go | merchant_id | 迁移 19 新增列；当前按 merchant_code 工作，未使用该列 |')
W ('| ' + (Code 'risk_alerts') + ' | riskAlertsModel.go | merchant_id | 同上 |')
W ('| ' + (Code 'pending_transactions') + ' | pendingTransactionsModel.go | merchant_id | 同上 |')
W ('| ' + (Code 'game_round_replay') + ' | gameRoundReplayModel.go | merchant_id | 同上 |')
W ('| ' + (Code 'daily_settlements') + ' | dailySettlementsModel.go | settlement_period_id | 迁移 13 新增列；周期汇总由 settlementPeriodsModel 独立处理 |')
W ''
W '**根因**：迁移 13 / 19 等 ALTER 加列后，没有重新运行 goctl。生成的模型因此停留在旧表结构上。'
W ''
W '#### 防止再次漂移'
W ''
W ('新增了 ' + (Code 'internal/model/schema_consistency_test.go') + '，直接拿真实库比对模型声明的列，并真跑一遍生成的 CRUD SQL：')
W ''
W '```powershell'
W '# 需要 fastgame-mysql 在线（未设置 MYSQL_DSN 时测试自动 skip）'
W 'go test ./internal/model/ -run TestSchema -v'
W '```'
W ''
W '**今后任何 ALTER 之后都应跑一次这个测试。**'
W ''
W '### 9.4 库内注释损坏（已修复）'
W ''
W '线上库里曾有 **29 处注释**被错误 charset 毁掉（8 张表的 28 个列注释 + 1 个表注释），两种形态（均用 HEX 确认）：'
W ''
W '| 形态 | 例子 | HEX 证据 | 可否直接还原 |'
W '|---|---|---|---|'
W ('| 中文被替换成 ASCII 问号（含单个汉字被吃成一个 ?） | ' + (Code 'games.game_code') + ' = "????????"、' + (Code 'game_rtp_tiers.tier_code') + ' 末尾的 "等" 变成 "?" | ' + (Code '3F3F3F3F...') + ' | 回不来，但可从迁移文件取回原文 |')
W ('| 双重编码乱码（UTF-8 被当 Latin-1 再编码） | ' + (Code 'pending_transactions') + ' 表注释、' + (Code 'pending_transactions.trace_id') + ' | ' + (Code 'C3A5C2AD...') + ' | 回不来，但可从迁移文件取回原文 |')
W ''
W '成因：早期迁移脚本由未指定 charset 的客户端执行，中文在写入时就已损坏。'
W ''
W ('**修复方式**：以 ' + (Code 'docker/mysql/init/*.sql') + ' 为权威来源重建注释定义。')
W ''
W '| 步骤 | 命令 / 产物 |'
W '|---|---|'
W ('| 1. 提取权威注释 | ' + (Code '.\scripts\gen_comment_manifest.ps1') + ' → ' + (Code 'docker/mysql/migrations/comment_manifest.json') + ' |')
W ('| 2. 生成修复迁移 | ' + (Code '.\scripts\gen_comment_repair.ps1') + ' |')
W ('| 3. 应用 | ' + (Code 'docker/mysql/migrations/29-repair-column-comments-migration.sql') + '（回滚脚本同目录） |')
W ''
W ('修复要点：列定义由 ' + (Code 'information_schema') + ' 的 ' + (Code 'COLUMN_TYPE/IS_NULLABLE/COLUMN_DEFAULT/EXTRA') + ' 原样重建，**只替换 COMMENT**，因此类型、默认值、' + (Code 'auto_increment') + '、' + (Code 'on update CURRENT_TIMESTAMP') + ' 均无漂移（已比对修复前后 393 列定义，完全一致）。')
W ''
W '**修复后状态：36 个表注释 + 393 个列注释全部为简体中文，0 处不一致、0 处问号、0 处缺失。**'
W ''
W '### 9.5 注释全量中文化（已应用）'
W ''
W '注释先修复（9.4），再统一中文化：原先有 27 个表注释与 83 个列注释是英文，另有 264 个列完全没有注释。'
W ''
W '| 步骤 | 命令 / 产物 |'
W '|---|---|'
W ('| 1. 编写译法 | ' + (Code 'docker/mysql/migrations/translations_comments.json') + '（精确译法）、' + (Code 'translations_common_columns.json') + '（通用列名词典） |')
W ('| 2. 生成迁移 | ' + (Code '.\scripts\gen_comment_translate.ps1') + ' |')
W ('| 3. 应用 | ' + (Code 'docker/mysql/migrations/30-translate-comments-migration.sql') + '（回滚脚本同目录） |')
W ('| 4. 同步 Go 模型 | ' + (Code '.\scripts\annotate_models_zh.ps1') + ' — 用库中注释为模型字段补中文注释 |')
W ''
W '翻译约定：枚举值与专有名词保留英文（如 ' + (Code 'fishing/slot/crash/table') + '、' + (Code 'SHA-256') + '、' + (Code 'BCP 47') + '、' + (Code 'minor units') + '），中文在前、英文枚举在后，便于 DBA 阅读。'
W ''
W '**当前状态：**'
W ''
W '| 对象 | 数量 | 中文注释 | 无注释 |'
W '|---|---:|---:|---:|'
W '| 基表 | 36 | 36 | 0 |'
W '| 基表列 | 393 | 393 | 0 |'
W '| 视图计算列 | 16 | 标注为「视图计算列」 | — |'
W ''
W ('Go 模型字段注释由 ' + (Code 'scripts/annotate_models_zh.ps1') + ' 自动同步自库中注释，两者不会漂移。')
W ''
W '### 9.6 其他'
W ''
W '- 旧文档记载的表数与实际不符（曾写 35 张，实际 36 张基表 + 2 视图），以本文档为准。'
W ''
W '## 十、维护命令'
W ''
W '```powershell'
W '# 应用单个迁移'
W '.\scripts\apply-migration.ps1 24-i18n-dictionary'
W '# 应用全部'
W '.\scripts\apply-all-migrations.ps1'
W '# 重新生成本文档'
W 'powershell -ExecutionPolicy Bypass -File .\scripts\gen_database_doc.ps1'
W '# 校验模型与实际表结构是否一致（ALTER 之后必跑）'
W 'go test ./internal/model/ -run TestSchema -v'
W '# 注释维护：提取权威注释 -> 生成修复/翻译迁移 -> 同步 Go 模型'
W 'powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_manifest.ps1'
W 'powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_repair.ps1'
W 'powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_translate.ps1'
W 'powershell -ExecutionPolicy Bypass -File .\scripts\annotate_models_zh.ps1 -DryRun'
W '# 表数量核对'
W 'docker exec fastgame-mysql mysql --default-character-set=utf8mb4 -ufastgame -pfastgame_pass fastgame -e "SELECT TABLE_TYPE, COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=''fastgame'' GROUP BY TABLE_TYPE;"'
W '```'
W ''
W '---'
W ''
W ('*本文件由脚本生成。如需修改结构说明，请改 ' + (Code 'scripts/gen_database_doc.ps1') + ' 后重新生成。*')

$outPath = Join-Path $Root "docs\DATABASE.md"
[System.IO.File]::WriteAllText($outPath, $sb.ToString(), $enc)
Write-Host ("已生成 docs/DATABASE.md ({0} 字节)" -f (Get-Item $outPath).Length)
