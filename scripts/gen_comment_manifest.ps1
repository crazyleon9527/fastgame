# gen_comment_manifest.ps1 — 从迁移文件提取权威的表/列注释，输出 JSON 清单
#
# 为什么需要这个：线上库里部分注释在建表时被错误 charset 毁掉了
#   - 中文被替换成 ASCII 问号（HEX 3F3F3F...），原始文字已丢失
#   - 双重编码乱码（HEX C3A5C2AD...），Windows-1252 不可逆映射
# 而 docker/mysql/init/*.sql 里保留着完整中文，是权威定义。
#
# 用法:
#   powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_manifest.ps1
# 输出:
#   docker/mysql/migrations/comment_manifest.json
#
# 迁移文件按文件名升序处理，后者覆盖前者（与 ALTER TABLE ... MODIFY 的语义一致）。

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

$utf8 = [System.Text.Encoding]::UTF8

$tableComments = [ordered]@{}
$columnComments = [ordered]@{}

function Add-TableComment([string]$t, [string]$c) {
  if (-not $t -or -not $c) { return }
  $c = $c.Trim()
  if ($c -eq '') { return }
  $tableComments[$t] = $c
}
function Add-ColumnComment([string]$t, [string]$col, [string]$c) {
  if (-not $t -or -not $col -or -not $c) { return }
  $c = $c.Trim()
  if ($c -eq '') { return }
  $columnComments["$t.$col"] = $c
}

function Import-SqlFile([string]$path) {
  $lines = [System.IO.File]::ReadAllLines($path, $utf8)
  $curTable = $null
  foreach ($line in $lines) {
    # 切换当前表
    $m = [regex]::Match($line, '(?i)(?:CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?|ALTER\s+TABLE)\s+`?([a-z_][a-z0-9_]*)`?')
    if ($m.Success) { $curTable = $m.Groups[1].Value.ToLower(); continue }
    if (-not $curTable) { continue }

    # 表定义结束行：) ENGINE=... COMMENT='表注释'
    if ($line -match '^\s*\)\s*(ENGINE|COMMENT|[A-Z])') {
      $tc = [regex]::Match($line, "(?i)COMMENT\s*=?\s*'([^']*)'")
      if ($tc.Success) { Add-TableComment $curTable $tc.Groups[1].Value }
      $curTable = $null
      continue
    }

    # 列注释：同行的 COMMENT '...'
    $cc = [regex]::Match($line, "(?i)COMMENT\s+'([^']*)'")
    if ($cc.Success) {
      $col = [regex]::Match($line, '^\s*`?([a-z_][a-z0-9_]*)`?\s+[a-zA-Z]')
      if ($col.Success) { Add-ColumnComment $curTable $col.Groups[1].Value.ToLower() $cc.Groups[1].Value }
    }
  }
}

foreach ($f in (Get-ChildItem docker\mysql\init\*.sql | Sort-Object Name)) { Import-SqlFile $f.FullName }

# 质检：清单里不应出现问号或疑似乱码
$suspicious = @()
foreach ($k in $tableComments.Keys) {
  if ($tableComments[$k] -match '\?\?\?|[\u00c0-\u00ff]{2,}') { $suspicious += "table:$k = $($tableComments[$k])" }
}
foreach ($k in $columnComments.Keys) {
  if ($columnComments[$k] -match '\?\?\?|[\u00c0-\u00ff]{2,}') { $suspicious += "column:$k = $($columnComments[$k])" }
}

$manifest = [ordered]@{
  generated_from = 'docker/mysql/init/*.sql'
  table_comments = $tableComments
  column_comments = $columnComments
}

$out = Join-Path $Root "docker\mysql\migrations\comment_manifest.json"
$json = $manifest | ConvertTo-Json -Depth 5
[System.IO.File]::WriteAllText($out, $json, (New-Object System.Text.UTF8Encoding($false)))

Write-Host ("提取完毕：{0} 个表注释，{1} 个列注释 -> {2}" -f $tableComments.Count, $columnComments.Count, $out)
if ($suspicious.Count -gt 0) {
  Write-Host "警告：清单中发现可疑内容（不应入库）：" -ForegroundColor Yellow
  $suspicious | Select-Object -First 10 | ForEach-Object { Write-Host "  $_" -ForegroundColor Yellow }
  exit 1
}
Write-Host "质检通过：清单中无问号或乱码。" -ForegroundColor Green
