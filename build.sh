#!/usr/bin/env sh
# Builds DSP Ledger for Windows and Linux (64-bit). Needs Go 1.21+.
set -e
cd "$(dirname "$0")"
mkdir -p dist
export CGO_ENABLED=0
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/dsp-ledger-windows-x64.exe .
GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/dsp-ledger-linux-x64 .
GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/dsp-ledger-linux-arm64 .
echo "Built into ./dist"
