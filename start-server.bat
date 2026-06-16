@echo off
setlocal

set "TMD_EXE=%~dp0tmd.exe"
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd-Windows-amd64.exe"
)
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd"
)

if not exist "%TMD_EXE%" (
    echo tmd executable not found beside this script.
    echo Building from source...
    go build -ldflags "-s -w -X github.com/unkmonster/tmd/internal/api.Version=test" -o tmd.exe .
    if %errorlevel% neq 0 (
        echo Build failed.
        exit /b %errorlevel%
    )
    set "TMD_EXE=%~dp0tmd.exe"
)

"%TMD_EXE%" -server %*
exit /b %errorlevel%
