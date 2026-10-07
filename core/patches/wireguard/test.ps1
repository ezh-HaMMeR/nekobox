param([string]$GoCommand = 'go', [string]$CMakeCommand = 'cmake')
$ErrorActionPreference = 'Stop'
$taskRoot = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
$taskBuild = Join-Path $taskRoot 'build/wireguard-tests'
$taskServer = Join-Path $taskRoot 'core/server'
New-Item -ItemType Directory -Force $taskBuild | Out-Null
Push-Location $taskServer
try {
    & $GoCommand mod download github.com/sagernet/wireguard-go
    if ($LASTEXITCODE) { throw 'WireGuard download failed' }
    $taskModule = & $GoCommand list '-mod=readonly' -m -f '{{.Dir}}' github.com/sagernet/wireguard-go
    if ($LASTEXITCODE -or !$taskModule) { throw 'WireGuard module not found' }
    $taskCopy = Join-Path $taskBuild 'wireguard-source'
    New-Item -ItemType Directory -Force $taskCopy | Out-Null
    Copy-Item -Path (Join-Path $taskModule '*') -Destination $taskCopy -Recurse -Force
    $taskModFile = Join-Path $taskBuild 'test.mod'
    Copy-Item -LiteralPath (Join-Path $taskServer 'go.mod') -Destination $taskModFile -Force
    Copy-Item -LiteralPath (Join-Path $taskServer 'go.sum') -Destination (Join-Path $taskBuild 'test.sum') -Force
    & $GoCommand mod edit "-modfile=$taskModFile" "-replace=github.com/sagernet/wireguard-go=$taskCopy"
    if ($LASTEXITCODE) { throw 'Build-local replacement failed' }
    $taskOverlay = Join-Path $taskBuild 'overlay'
    & $CMakeCommand "-DGO_WIREGUARD_DIR=$taskCopy" "-DGO_OVERLAY_DIR=$taskOverlay" '-DGO_OVERLAY_TESTS=ON' -P (Join-Path $PSScriptRoot 'overlay.cmake')
    if ($LASTEXITCODE) { throw 'Overlay generation failed' }
    & $GoCommand test '-mod=readonly' "-modfile=$taskModFile" "-overlay=$taskOverlay/overlay.json" github.com/sagernet/wireguard-go/conn -run 'TestIPv4|TestIPv6Other|TestDualStack' -v -timeout 45s
    if ($LASTEXITCODE) { throw 'WireGuard regression tests failed' }
} finally {
    Pop-Location
}
