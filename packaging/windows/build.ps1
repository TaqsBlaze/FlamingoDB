$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '../..')
New-Item -ItemType Directory -Force build/windows | Out-Null
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -o build/windows/flamingodbd.exe ./cmd/flamingodbd
if ($LASTEXITCODE -ne 0) { throw 'Server build failed' }
go build -ldflags='-H windowsgui' -o build/windows/FlamingoDB.exe ./cmd/flamingo-launcher
if ($LASTEXITCODE -ne 0) { throw 'Launcher build failed' }
makensis packaging/windows/flamingodb.nsi
if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
