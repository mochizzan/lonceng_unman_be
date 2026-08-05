@echo off
setlocal

set APP_DIR=%~dp0
set BIN_DIR=%APP_DIR%bin
set SERVER_EXE=%BIN_DIR%\server.exe

echo [1/2] Building server...
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
go build -o "%SERVER_EXE%" "%APP_DIR%cmd\server"
if errorlevel 1 (
    echo [ERROR] Build failed
    exit /b 1
)
echo [OK] Build successful

echo [2/2] Starting server...
"%SERVER_EXE%"
