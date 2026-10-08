@echo off
setlocal

:: ------------------------------------------------------------
:: Build-Script  –  iperf3-Tracker (Go)
:: Aufruf:  build.cmd [clean] [tidy] [release]
::
::   (kein Argument)  Standard-Build (Debug, aktuelles OS)
::   clean            Binärdateien löschen, dann bauen
::   tidy             go mod tidy vorher ausführen
::   release          Optimierter Build ohne Debugsymbole
::   clean tidy release können kombiniert werden
:: ------------------------------------------------------------

set MODULE=iperf3-tracker
set CMD_PATH=.\cmd\iperf3-tracker
set OUTDIR=_release
set OUT=%OUTDIR%\iperf3-tracker.exe
set GOOS=windows
set GOARCH=amd64

:: Argumente auswerten
set DO_CLEAN=0
set DO_TIDY=0
set DO_RELEASE=0
for %%A in (%*) do (
    if /I "%%A"=="clean"   set DO_CLEAN=1
    if /I "%%A"=="tidy"    set DO_TIDY=1
    if /I "%%A"=="release" set DO_RELEASE=1
)

:: ---------- Voraussetzungen ----------
where go >nul 2>&1
if errorlevel 1 if exist "C:\tools\go1.27.1\bin\go.exe" set "PATH=%PATH%;C:\tools\go1.27.1\bin"
if errorlevel 1 if exist "C:\Program Files\Go\bin\go.exe" set "PATH=%PATH%;C:\Program Files\Go\bin"
if errorlevel 1 if exist "C:\go\bin\go.exe"               set "PATH=%PATH%;C:\go\bin"
where go >nul 2>&1
if errorlevel 1 (
    echo [FEHLER] Go ist nicht im PATH. Bitte Go installieren: https://go.dev/dl/
    exit /b 1
)
for /f "tokens=3" %%V in ('go version') do set GO_VER=%%V
echo [INFO ] Go-Version: %GO_VER%

:: ---------- Aufräumen ----------
if %DO_CLEAN%==1 (
    echo [INFO ] Aufräumen ...
    if exist "%OUT%" del /q "%OUT%"
)

:: ---------- go mod tidy ----------
if %DO_TIDY%==1 (
    echo [INFO ] go mod tidy ...
    go mod tidy
    if errorlevel 1 ( echo [FEHLER] go mod tidy fehlgeschlagen. & exit /b 1 )
)

:: ---------- Build-Flags ----------
:: Build-Datum ermitteln und in das Binary injizieren (internal/version.BuildDate).
for /f "delims=" %%i in ('powershell -NoProfile -Command "Get-Date -Format \"yyyy-MM-dd HH:mm\""') do set "BUILD_DATE=%%i"
echo [INFO ] Build-Datum: %BUILD_DATE%
set "VERSION_FLAG=-X '%MODULE%/internal/version.BuildDate=%BUILD_DATE%'"

set BUILD_FLAGS=

if %DO_RELEASE%==1 (
    echo [INFO ] Modus: Release (keine Debugsymbole^)
    set BUILD_FLAGS=-trimpath -ldflags "-s -w %VERSION_FLAG%"
) else (
    echo [INFO ] Modus: Debug
    set BUILD_FLAGS=-ldflags "%VERSION_FLAG%"
)

:: ---------- Kompilieren ----------
if not exist "%OUTDIR%" mkdir "%OUTDIR%"
echo [INFO ] Baue %OUT% ...
go build %BUILD_FLAGS% -o "%OUT%" %CMD_PATH%
if errorlevel 1 (
    echo [FEHLER] Build fehlgeschlagen.
    exit /b 1
)

:: ---------- Ergebnis ----------
for %%F in ("%OUT%") do set SIZE=%%~zF
set /a SIZE_MB=%SIZE% / 1048576
echo [OK   ] %OUT% erfolgreich erstellt  (%SIZE_MB% MB, Build %BUILD_DATE%)
echo [HINWEIS] Beim ersten Start wird config.yaml im Arbeitsverzeichnis angelegt (oder -config ^<pfad^>).

endlocal
