#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
mkdir -p build/windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/windows/flamingodbd.exe ./cmd/flamingodbd
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-H windowsgui' -o build/windows/FlamingoDB.exe ./cmd/flamingo-launcher
"${MAKENSIS:-makensis}" packaging/windows/flamingodb.nsi
