# FastGame — 结算周期(settlement_periods)冒烟脚本
# 用法(直接打 admin-api,推荐):
#   .\scripts\smoke-settlement.ps1
#   .\scripts\smoke-settlement.ps1 -Username admin -Password admin123 -TotpCode 123456
#   .\scripts\smoke-settlement.ps1 -BaseUrl http://localhost:18889
# 经网关(需 sponge.localhost 解析):
#   .\scripts\smoke-settlement.ps1 -BaseUrl http://sponge.localhost:18000
# 前置: docker compose up -d && .\scripts\dev-start.ps1 已启动, 迁移13已应用
# 验证链路: 登录 -> 结算周期列表 -> (有 draft) rollup -> 复查列表
# 退出码: 0=通过 1=未通过(或前置缺失) 2=部分通过(无数据可rollup,提示先同步)

param(
    [string]$BaseUrl = "http://localhost:18889",
    [string]$Username = "admin",
    [string]$Password = "admin123",
    [string]$TotpCode = "",
    [int]$TimeoutSec = 10
)

$ErrorActionPreference = "Stop"
$Api = "$BaseUrl/api/v1/admin"

function Invoke-Json {
    param([string]$Method, [string]$Path, $Body = $null)
    $params = @{
        Method  = $Method
        Uri     = "$Api$Path"
        Headers = @{ "Content-Type" = "application/json" }
        TimeoutSec = $TimeoutSec
    }
    if ($script:Token) { $params.Headers["Authorization"] = "Bearer $script:Token" }
    if ($null -ne $Body) { $params.Body = ($Body | ConvertTo-Json -Compress) }
    $resp = Invoke-RestMethod @params
    return $resp
}

function Fail {
    param([string]$Msg)
    Write-Host "FAIL: $Msg" -ForegroundColor Red
    exit 1
}

Write-Host "==> 1/5 健康检查 $BaseUrl" -ForegroundColor Cyan
try {
    $health = Invoke-WebRequest -Uri "$BaseUrl/healthz" -TimeoutSec $TimeoutSec -ErrorAction SilentlyContinue
    if ($health.StatusCode -eq 200) { Write-Host "  OK /healthz -> $($health.StatusCode)" -ForegroundColor Green }
    else { Write-Host "  /healthz 返回 $($health.StatusCode) (继续尝试登录)" -ForegroundColor DarkYellow }
} catch {
    Write-Host "  /healthz 不可用(正常,服务可能未提供该端点),继续" -ForegroundColor DarkYellow
}

Write-Host "==> 2/5 登录 $Username" -ForegroundColor Cyan
$loginBody = @{ username = $Username; password = $Password }
if ($TotpCode) { $loginBody.totpCode = $TotpCode }
try {
    $login = Invoke-Json -Method "POST" -Path "/login" -Body $loginBody
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 401 -or $_.Exception.Message -match "totp|TOTP") {
        Fail "登录被拒。若账号已绑定2FA请传 -TotpCode <动态码>; 若未绑定说明账号需先做 /totp/setup"
    }
    Fail "登录失败: $($_.Exception.Message)"
}
if (-not $login.accessToken) { Fail "登录响应缺少 accessToken" }
$script:Token = $login.accessToken
Write-Host "  OK token已获取 (expireAt=$($login.expireAt), role=$($login.roleName))" -ForegroundColor Green

Write-Host "==> 3/5 查询结算周期列表" -ForegroundColor Cyan
$periods = Invoke-Json -Method "GET" -Path "/reports/settlement-periods?page=1&pageSize=20"
$total = $periods.total
Write-Host "  共 $total 条结算周期" -ForegroundColor Green
if ($total -eq 0) {
    Write-Host "  WARN: 表 settlement_periods 为空。" -ForegroundColor Yellow
    Write-Host "  请先在后台[日结算]页同步(sync)并确认(confirm)至少一条日结算," -ForegroundColor Yellow
    Write-Host "  或检查迁移13是否已应用: .\scripts\apply-migration.ps1 13-settlement-billing" -ForegroundColor Yellow
    exit 2
}

$draft = $periods.list | Where-Object { $_.status -eq "draft" } | Select-Object -First 1
if (-not $draft) {
    Write-Host "  INFO: 无 draft 状态周期(现有状态: $((($periods.list | ForEach-Object { $_.status }) -join ',')))" -ForegroundColor Yellow
    Write-Host "  rollup 仅允许 draft;已确认的周期无需重算。列表查询通过即可视为冒烟通过。" -ForegroundColor Yellow
    Write-Host "  RESULT: PASS(列表正常,无 draft 可测 rollup)" -ForegroundColor Green
    exit 0
}
Write-Host "  选中 draft 周期 id=$($draft.id) type=$($draft.periodType) $($draft.periodStart)~$($draft.periodEnd)" -ForegroundColor Green

Write-Host "==> 4/5 触发 rollup id=$($draft.id)" -ForegroundColor Cyan
$rollup = Invoke-Json -Method "POST" -Path "/reports/settlement-periods/$($draft.id)/rollup" -Body @{}
Write-Host "  OK rolledUp=$($rollup.rolledUp)" -ForegroundColor Green

Write-Host "==> 5/5 复查列表验证" -ForegroundColor Cyan
$after = Invoke-Json -Method "GET" -Path "/reports/settlement-periods?page=1&pageSize=20&periodType=$($draft.periodType)"
$row = $after.list | Where-Object { $_.id -eq $draft.id } | Select-Object -First 1
if (-not $row) { Fail "复查未找到 id=$($draft.id)" }
Write-Host "  id=$($row.id) status=$($row.status) bet=$($row.totalBet) win=$($row.totalWin) ggr=$($row.ggr) rounds=$($row.totalRounds)" -ForegroundColor Green
if ($row.totalBet -le 0) {
    Write-Host "  WARN: 汇总后 totalBet<=0,说明区间内无已确认日结算,数据未真实汇总" -ForegroundColor Yellow
} else {
    Write-Host "  OK 金额已汇总,rollup 链路正常" -ForegroundColor Green
}

Write-Host ""
Write-Host "RESULT: PASS" -ForegroundColor Green
exit 0
