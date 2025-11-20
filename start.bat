@echo off
echo ========================================
echo   iperf3-Tracker Startup Script
echo ========================================
echo.

REM Get the directory where the script is located
cd /d "%~dp0"

echo [1/2] Starting Backend Server...
echo.
start "iperf3-Tracker Backend" cmd /k "cd backend && python -m uvicorn app.main:app --reload --host 0.0.0.0 --port 8000"

REM Wait a bit for backend to start
timeout /t 3 /nobreak >nul

echo [2/2] Starting Frontend Server...
echo.
start "iperf3-Tracker Frontend" cmd /k "cd frontend && npm start"

echo.
echo ========================================
echo   Both servers are starting...
echo ========================================
echo.
echo Backend:  http://localhost:8000
echo Frontend: http://localhost:3000
echo.
echo Press any key to exit this window...
echo (The servers will keep running in separate windows)
pause >nul
