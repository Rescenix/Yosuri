@echo off
setlocal enabledelayedexpansion
REM ============================================================
REM  Rescene 服务注册脚本（需管理员权限运行）
REM  1) ResceneHarness  -> python watchdog.py   (AppDirectory=harness)
REM  2) ResceneBackend  -> server.exe           (AppDirectory=main-backend)
REM  均设为 SERVICE_AUTO_START，stdout/stderr 落到各自 logs 目录。
REM ============================================================

set HARNESS_DIR=C:\Pro2026\re0\harness
set GO_DIR=C:\Pro2026\re0\main-backend

REM ---- 前置检查：nssm ----
where nssm >nul 2>&1
if errorlevel 1 (
    echo [ERROR] 未找到 nssm。请先安装（管理员 PowerShell）:
    echo     winget install NSSM.NSSM
    echo 或从 https://nssm.cc/download 下载后把 nssm.exe 放入 PATH。
    exit /b 1
)

REM ---- 前置检查：管理员权限 ----
net session >nul 2>&1
if errorlevel 1 (
    echo [ERROR] 需要管理员权限。右键"以管理员身份运行"本脚本。
    exit /b 1
)

REM ---- 取真实 python 路径（跳过 WindowsApps 商店 stub） ----
set PYTHON_EXE=
for /f "delims=" %%i in ('where python') do (
    echo %%i | find /i "WindowsApps" >nul
    if errorlevel 1 (
        if not defined PYTHON_EXE set PYTHON_EXE=%%i
    )
)
if not defined PYTHON_EXE (
    echo [ERROR] where python 未找到可用的 python.exe
    exit /b 1
)
echo [INFO] python = !PYTHON_EXE!

REM ============ 服务 1: ResceneHarness ============
nssm stop ResceneHarness >nul 2>&1
nssm remove ResceneHarness confirm >nul 2>&1

nssm install ResceneHarness "!PYTHON_EXE!" "%HARNESS_DIR%\watchdog.py"
nssm set ResceneHarness AppDirectory "%HARNESS_DIR%"
nssm set ResceneHarness Start SERVICE_AUTO_START
nssm set ResceneHarness AppStdout "%HARNESS_DIR%\logs\service_stdout.log"
nssm set ResceneHarness AppStderr "%HARNESS_DIR%\logs\service_stderr.log"
nssm set ResceneHarness AppRotateFiles 1
nssm set ResceneHarness AppRotateBytes 10485760
nssm set ResceneHarness Description "Rescene Harness (FastAPI :8001, watchdog-supervised)"
echo [OK] ResceneHarness 已注册

REM ============ 服务 2: ResceneBackend (Go) ============
if not exist "%GO_DIR%\server.exe" (
    echo [WARN] %GO_DIR%\server.exe 不存在，先构建:
    echo     cd %GO_DIR% ^&^& go build -o server.exe .\cmd\server
    echo 跳过 ResceneBackend 注册。
    goto :start_services
)
nssm stop ResceneBackend >nul 2>&1
nssm remove ResceneBackend confirm >nul 2>&1

nssm install ResceneBackend "%GO_DIR%\server.exe"
nssm set ResceneBackend AppDirectory "%GO_DIR%"
nssm set ResceneBackend Start SERVICE_AUTO_START
nssm set ResceneBackend AppStdout "%GO_DIR%\logs\service_stdout.log"
nssm set ResceneBackend AppStderr "%GO_DIR%\logs\service_stderr.log"
nssm set ResceneBackend AppRotateFiles 1
nssm set ResceneBackend AppRotateBytes 10485760
nssm set ResceneBackend Description "Rescene Go Backend (:8080)"
echo [OK] ResceneBackend 已注册

:start_services
nssm start ResceneHarness
echo [INFO] ResceneHarness 已启动。验证: curl http://localhost:8001/health
if exist "%GO_DIR%\server.exe" (
    nssm start ResceneBackend
    echo [INFO] ResceneBackend 已启动。验证: curl http://localhost:8080/
)
endlocal
