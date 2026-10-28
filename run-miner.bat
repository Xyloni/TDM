@echo off
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
    echo Go is not installed or is not available in PATH.
    echo Install Go, then open a new terminal and try again.
    pause
    exit /b 1
)

if not exist ".\data-test" mkdir ".\data-test"

echo Building GrubDrops for Windows...
go build -o ".\grubdrops.exe" ".\cmd\miner"
if errorlevel 1 (
    echo.
    echo Build failed. The miner was not started.
    echo If grubdrops.exe is already running, close it and run this file again.
    pause
    exit /b 1
)

rem Use the existing local test database and its adjacent master.key.
set "GRUB_DB_PATH=%CD%\data-test\miner.db"
set "GRUB_MASTER_KEY="

echo.
echo Starting GrubDrops with database: %GRUB_DB_PATH%
echo Leave this window open while the miner is running.
echo Open http://localhost:8080 in your browser.
echo.
".\grubdrops.exe"

echo.
echo GrubDrops has stopped.
pause
