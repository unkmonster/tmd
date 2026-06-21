@echo off
setlocal

set "TMD_EXE=%~dp0tmd-Windows-amd64.exe"
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd.exe"
)
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd-test.exe"
)
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd"
)

if not exist "%TMD_EXE%" (
    echo tmd executable not found beside this script.
    echo Building from source...
    go build -ldflags "-X github.com/unkmonster/tmd/internal/api.Version=test" -o tmd-test.exe .
    if %errorlevel% neq 0 (
        echo Build failed.
        exit /b %errorlevel%
    )
    set "TMD_EXE=%~dp0tmd-test.exe"
)

"%TMD_EXE%" -server %*
exit /b %errorlevel%
