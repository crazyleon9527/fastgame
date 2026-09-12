# FastGame autopilot — wake agent every 5 minutes
# Stop: kill the PowerShell process running this script
$ErrorActionPreference = "SilentlyContinue"
while ($true) {
    Start-Sleep -Seconds 300
    Write-Output 'AGENT_LOOP_TICK_fastgame {"prompt":"FastGame autopilot tick: read docs/AUTOPILOT.md, execute next pending phase, self-review, update AUTOPILOT.md, build/deploy if needed, commit if safe. Do not ask user."}'
}
