@echo off
setlocal

set "TMD_EXE=%~dp0tmd.exe"
if not exist "%TMD_EXE%" (
    set "TMD_EXE=%~dp0tmd"
)

if not exist "%TMD_EXE%" (
    echo tmd executable not found beside this script.
    echo Expected one of:
    echo   %~dp0tmd.exe
    echo   %~dp0tmd
    exit /b 1
)

"%TMD_EXE%" -server %*
exit /b %errorlevel%
