@echo off
setlocal

if "%TMD_DEV%"=="1" (
    echo [DEV MODE] Running from source with live frontend reload...
    echo [DEV MODE] Edit files in internal/api/web/web1/ or web2/ and refresh browser.
    go run . -server %*
    exit /b %errorlevel%
)

set "TMD_EXE=%~dp0tmd-Windows-amd64.exe"
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd.exe"
)
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd"
)

if not exist "%TMD_EXE%" (
    echo tmd executable not found beside this script.
    echo Building from source with dev mode...
    set "TMD_EXE=%~dp0tmd-test.exe"
    go build -ldflags "-X github.com/unkmonster/tmd/internal/api.Version=test" -o tmd-test.exe .
    if %errorlevel% neq 0 (
        echo Build failed.
        exit /b %errorlevel%
    )
    set "TMD_DEV=1"
)

"%TMD_EXE%" -server %*
exit /b %errorlevel%
