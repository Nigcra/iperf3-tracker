@echo off
setlocal

:: ------------------------------------------------------------
:: Start-Script  –  iperf3-Tracker
:: Aufruf:  start.cmd [neu]
::
::   (kein Argument)  startet _release\iperf3-tracker.exe und baut sie vorher,
::                    falls sie noch nicht existiert
::   neu              baut die Binary vor dem Start neu (nach Code-Änderungen)
::
:: Die Oberfläche öffnet sich im Browser unter http://localhost:8000.
:: Beenden mit Strg+C oder durch Schließen des Konsolenfensters.
:: ------------------------------------------------------------

cd /d "%~dp0"

if /I "%~1"=="neu" goto build
if exist "_release\iperf3-tracker.exe" goto run

:build
call "%~dp0build.cmd" release
if errorlevel 1 exit /b 1

:run
:: Läuft bereits eine Instanz auf Port 8000? Dann nur den Browser öffnen.
powershell -NoProfile -Command "if (Get-NetTCPConnection -State Listen -LocalPort 8000 -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
if not errorlevel 1 (
    echo [INFO ] iperf3-Tracker laeuft bereits - oeffne nur den Browser.
    start "" http://localhost:8000
    exit /b 0
)

:: Browser leicht verzögert öffnen, damit der Dienst schon lauscht.
start "" /min powershell -NoProfile -WindowStyle Hidden -Command "Start-Sleep -Seconds 2; Start-Process 'http://localhost:8000'"

echo [INFO ] Starte iperf3-Tracker - Oberflaeche unter http://localhost:8000
echo [INFO ] Beenden mit Strg+C
title iperf3-Tracker
cd /d "%~dp0_release"
iperf3-tracker.exe

endlocal