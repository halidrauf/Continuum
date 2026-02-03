@echo off
setlocal

echo ==========================================
echo      Continuum Benchmark Suite
echo ==========================================

REM Check if Docker is running (simple check)
docker ps >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker is not running! Please start Docker Desktop.
    exit /b 1
)

REM Build the Benchmark Runner
echo [INFO] Building Benchmark Runner...
cd tests\benchmark
go build -o benchmark.exe runner.go
if %errorlevel% neq 0 (
    echo [ERROR] Failed to build runner.
    cd ..\..
    exit /b 1
)
cd ..\..

:menu
echo.
echo Select Benchmark Suite:
echo 1. CPU Stress Test (Matrix Multiplication)
echo 2. Network I/O Test (API Fetch)
echo 3. Mixed Load Test (Ported from test.sql)
echo 4. Security Probe (Ported from test_security.sql)
echo 5. Exit
echo.
set /p choice="Enter choice (1-5): "

if "%choice%"=="1" (
    echo [INFO] Running CPU Stress Test...
    tests\benchmark\benchmark.exe -suite=cpu
    goto menu
)
if "%choice%"=="2" (
    echo [INFO] Running Network I/O Test...
    tests\benchmark\benchmark.exe -suite=network
    goto menu
)
if "%choice%"=="3" (
    echo [INFO] Running Mixed Load Test...
    tests\benchmark\benchmark.exe -suite=mixed
    goto menu
)
if "%choice%"=="4" (
    echo [INFO] Running Security Probe...
    tests\benchmark\benchmark.exe -suite=security
    goto menu
)
if "%choice%"=="5" (
    echo Exiting...
    exit /b 0
)

echo Invalid choice.
goto menu
