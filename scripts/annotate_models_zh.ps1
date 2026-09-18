# annotate_models_zh.ps1 — 用数据库的中文列注释为 Go 模型结构体字段补中文注释
#
# 数据来源：线上 MySQL 的 information_schema（表注释与列注释，均已中文化）
# 处理范围：internal/model/*.go 中带 db:"xxx" 标签的结构体字段
# 行为：
#   - 字段行末尾追加  // 中文注释（已存在 // 注释的跳过）
#   - 结构体声明上方插入中文文档注释（表注释）
#   - 幂等：重复运行不产生变化
#
# 用法:
#   powershell -ExecutionPolicy Bypass -File .\scripts\annotate_models_zh.ps1 -DryRun
#   powershell -ExecutionPolicy Bypass -File .\scripts\annotate_models_zh.ps1

param([switch]$DryRun)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

$enc  = New-Object System.Text.UTF8Encoding($false)
$tmp  = Join-Path ([System.IO.Path]::GetTempPath()) "fastgame_annot"
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

# ---------- 1. 取库里的中文注释 ----------
$colComment = @{}   # "table.col" -> 注释
$tblComment = @{}   # "table" -> 注释
foreach ($line in @(Invoke-Sql "SELECT TABLE_NAME, COLUMN_NAME, IFNULL(COLUMN_COMMENT,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='fastgame';")) {
  if ([string]::IsNullOrWhiteSpace($line)) { continue }
  $p = $line -split "`t"
  if ($p.Count -lt 3) { continue }
  if ($p[2] -ne '') { $colComment["$($p[0]).$($p[1])"] = $p[2] }
}
foreach ($line in @(Invoke-Sql "SELECT TABLE_NAME, IFNULL(TABLE_COMMENT,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA='fastgame' AND TABLE_TYPE='BASE TABLE';")) {
  if ([string]::IsNullOrWhiteSpace($line)) { continue }
  $p = $line -split "`t"
  if ($p.Count -ge 2 -and $p[1] -ne '') { $tblComment[$p[0]] = $p[1] }
}
Write-Host ("库里可用注释：{0} 个列，{1} 个表" -f $colComment.Count, $tblComment.Count)

# ---------- 2. 逐文件处理 ----------
$totalFields = 0
$totalStructs = 0
$changedFiles = @()

foreach ($f in (Get-ChildItem internal\model\*.go | Where-Object { $_.Name -notmatch '_test' })) {
  $lines = [System.IO.File]::ReadAllLines($f.FullName, [System.Text.Encoding]::UTF8)
  $out = New-Object System.Collections.Generic.List[string]

  # 先找出每个 struct 对应的表名：该 struct 的 db 标签列在哪个表里全部存在
  $structTable = @{}
  $curStruct = ''
  $structCols = @{}
  foreach ($line in $lines) {
    if ($line -match '^\s*(?:type\s+)?(\w+)\s+struct\s*\{') { $curStruct = $Matches[1]; $structCols[$curStruct] = New-Object System.Collections.Generic.List[string] }
    if ($curStruct -and $line -match 'db:"([a-z_]+)"') {
      $structCols[$curStruct].Add($Matches[1])
    }
  }
  $allTables = @($colComment.Keys | ForEach-Object { ($_ -split '\.')[0] } | Sort-Object -Unique)
  foreach ($s in $structCols.Keys) {
    $cols = @($structCols[$s])
    if ($cols.Count -eq 0) { continue }
    foreach ($t in $allTables) {
      $all = $true
      foreach ($c in $cols) { if (-not $colComment.ContainsKey("$t.$c")) { $all = $false; break } }
      if ($all) { $structTable[$s] = $t; break }
    }
  }

  $curStruct = ''
  $prevBlank = $false
  foreach ($line in $lines) {
    $newLine = $line

    # 结构体文档注释
    if ($line -match '^(\s*)(?:type\s+)?(\w+)\s+struct\s*\{') {
      $indent = $Matches[1]
      $sname  = $Matches[2]
      $curStruct = $sname
      if ($structTable.ContainsKey($sname)) {
        $tc = $tblComment[$structTable[$sname]]
        $docText = "// $sname 对应表 $($structTable[$sname])：$tc"
        # 若上一行已经是文档注释则跳过（幂等）
        $lastOut = if ($out.Count -gt 0) { $out[$out.Count - 1] } else { '' }
        if ($lastOut -notmatch [regex]::Escape($docText)) {
          $out.Add("$indent$docText") | Out-Null
          $totalStructs++
        }
      }
    }

    # 字段行注释
    if ($curStruct -and $line -match 'db:"([a-z_]+)"' -and $line -notmatch '//') {
      $col = $Matches[1]
      $tbl = if ($structTable.ContainsKey($curStruct)) { $structTable[$curStruct] } else { $null }
      if ($tbl -and $colComment.ContainsKey("$tbl.$col")) {
        $cmt = $colComment["$tbl.$col"]
        $newLine = $line.TrimEnd() + " // " + $cmt
        $totalFields++
      }
    }

    $out.Add($newLine) | Out-Null
  }

  $newText = ($out -join "`r`n") + "`r`n"
  $oldText = ([System.IO.File]::ReadAllText($f.FullName, [System.Text.Encoding]::UTF8))
  if ($newText -ne $oldText) {
    $changedFiles += $f.Name
    if (-not $DryRun) { [System.IO.File]::WriteAllText($f.FullName, $newText, $enc) }
  }
}

Write-Host ("`n{0}：字段注释 {1} 个，结构体注释 {2} 个，涉及文件 {3} 个" -f $(if ($DryRun) { '试运行' } else { '已写入' }), $totalFields, $totalStructs, $changedFiles.Count)
$changedFiles | ForEach-Object { Write-Host "  $_" }
if ($DryRun) { Write-Host "`n（试运行，未写入任何文件）" -ForegroundColor Yellow }
