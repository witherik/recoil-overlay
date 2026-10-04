param([switch]$Dev)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Push-Location $projectRoot
try {
    $portableGo = Join-Path $projectRoot '.tools\go1.23\go\bin'
    if (Test-Path (Join-Path $portableGo 'go.exe')) {
        $env:Path = "$portableGo;$env:Path"
        $env:GOPATH = Join-Path $projectRoot '.tools\gopath'
        $env:GOCACHE = Join-Path $projectRoot '.tools\gocache123'
    }
    $wailsCommand = Join-Path $projectRoot '.tools\gopath\bin\wails.exe'
    if (-not (Test-Path $wailsCommand)) { $wailsCommand = 'wails' }
    Push-Location frontend
    try {
        npm.cmd ci
        if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency installation failed.' }
        npm.cmd run build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    } finally { Pop-Location }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
    if ($Dev) { & $wailsCommand dev } else { & $wailsCommand build -platform windows/amd64 }
    if ($LASTEXITCODE -ne 0) { throw 'Wails build failed.' }
} finally { Pop-Location }
