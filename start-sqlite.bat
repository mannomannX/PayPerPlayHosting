@echo off
echo ========================================
echo  PayPerPlay - SQLite Setup
echo ========================================
echo.

echo [1/2] Using SQLite (no Docker needed)...
if not exist .env (
    echo Creating .env from .env.example...
    copy .env.example .env >nul
)

echo.
echo [2/2] Installing Go dependencies...
go mod tidy

echo.
echo ========================================
echo  Setup Complete!
echo ========================================
echo.
echo Using SQLite database: ./payperplay.db
echo No Docker containers needed!
echo.
echo Now start the backend with:
echo   go run ./cmd/api/main.go
echo.
pause
