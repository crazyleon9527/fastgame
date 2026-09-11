# Build vue-pure-admin and deploy to web/admin for nginx
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$VueDir = Join-Path $Root "web\admin-vue"
$OutDir = Join-Path $Root "web\admin"

Set-Location $VueDir
if (-not (Test-Path node_modules)) {
    Write-Host "Installing dependencies..." -ForegroundColor Cyan
    npm install
}
Write-Host "Building admin UI..." -ForegroundColor Cyan
npm run build

$Dist = Join-Path $VueDir "dist"
if (-not (Test-Path $Dist)) {
    throw "Build failed: dist not found"
}

Write-Host "Deploying to web/admin ..." -ForegroundColor Cyan
Get-ChildItem $OutDir -Exclude @("README.md", "uploads") | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
if (-not (Test-Path (Join-Path $OutDir "uploads"))) {
    New-Item -ItemType Directory -Path (Join-Path $OutDir "uploads") -Force | Out-Null
}
Copy-Item "$Dist\*" $OutDir -Recurse -Force

Write-Host "Done. Open http://localhost:18000/admin/" -ForegroundColor Green
