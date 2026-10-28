@echo off
rem Builds DSP Ledger for Windows and Linux (64-bit). Needs Go 1.21+.
cd /d "%~dp0"
if not exist dist mkdir dist
set CGO_ENABLED=0
set GOARCH=amd64
set GOOS=windows
go build -trimpath -ldflags "-s -w" -o dist\dsp-ledger-windows-x64.exe . || exit /b 1
set GOOS=linux
go build -trimpath -ldflags "-s -w" -o dist\dsp-ledger-linux-x64 . || exit /b 1
echo Built into .\dist
