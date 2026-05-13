@echo off
cd /d "%~dp0"
echo Building ds2api.exe (background daemon mode)...
go build -ldflags="-H=windowsgui" -o ds2api.exe ./cmd/ds2api
if %ERRORLEVEL% EQU 0 (
    echo Built: ds2api.exe
    echo.
    echo To start:  ds2api.exe
    echo Logs:      termal.log (same folder)
    echo To stop:   taskkill /f /im ds2api.exe
) else (
    echo Build failed.
    pause
)
