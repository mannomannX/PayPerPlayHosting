@echo off
echo ========================================
echo  PayPerPlay - PostgreSQL Setup
echo ========================================
echo.

REM Check if Docker is running
docker ps >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker is not running!
    echo Please start Docker Desktop first.
    pause
    exit /b 1
)

echo [1/4] Starting PostgreSQL container...
docker-compose up -d

echo.
echo [2/4] Waiting for PostgreSQL to be ready...
timeout /t 5 /nobreak >nul

echo.
echo [3/4] Copying PostgreSQL config to .env...
copy /Y .env.postgres .env >nul

echo.
echo [4/4] Installing Go dependencies...
go mod tidy

echo.
echo ========================================
echo  Setup Complete!
echo ========================================
echo.
echo PostgreSQL is running on: localhost:5432
echo Adminer (DB UI) is on: http://localhost:8080
echo.
echo Now start the backend with:
echo   go run ./cmd/api/main.go
echo.
pause
