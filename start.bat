@echo off
setlocal enabledelayedexpansion

chcp 65001 >nul

:: TMD Server Windows startup script
:: Listens to tmd.exe ERRORLEVEL:
:: 1. Normal shutdown (ERRORLEVEL 0) -> Exit script
:: 2. Crash (Other) -> Wait 5 seconds and start again

set BIN_PATH=.\tmd.exe

if not exist "%BIN_PATH%" (
	echo Error: Executable not found at %BIN_PATH%
	echo Please build the project first using: go build -o tmd.exe .\main.go
	pause
	exit /b 1
)

echo Starting TMD Server...

:loop
"%BIN_PATH%" -server %*
set EXIT_CODE=%ERRORLEVEL%

if %EXIT_CODE% EQU 0 (
    echo Server shut down gracefully. Stopping script.
    goto :end
)

echo Server crashed or stopped unexpectedly ^(Exit Code %EXIT_CODE%^). Starting again in 5 seconds...
timeout /t 5 >nul
goto :loop

:end
endlocal
