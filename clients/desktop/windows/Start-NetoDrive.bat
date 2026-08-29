@echo off
REM NetoDrive Sync for Windows - painel + sync + provider CFAPI (clique / manter no dispositivo)
setlocal
cd /d "%~dp0"

set CFG=%APPDATA%\NetoDrive\netodrive.json

if not exist "%CFG%" (
  echo Criando configuracao inicial...
  mkdir "%APPDATA%\NetoDrive" 2>nul
  "%~dp0netodrive-sync.exe" -init -config "%CFG%"
  echo.
  echo Edite o arquivo e ajuste server_url e local_folder:
  echo   %CFG%
  notepad "%CFG%"
)

REM Provider CFAPI: reinicia o provider; registra no Explorer so se local_folder mudou ou falta registro
taskkill /IM netodrive-sync.exe /F >nul 2>&1
timeout /t 1 /nobreak >nul

if exist "%~dp0netodrive-provider.exe" (
  taskkill /IM netodrive-provider.exe /F >nul 2>&1
  timeout /t 1 /nobreak >nul
  "%~dp0netodrive-provider.exe" -ensure-register -config "%CFG%"
  if not exist "%APPDATA%\NetoDrive\logs" mkdir "%APPDATA%\NetoDrive\logs" 2>nul
  start "" /B cmd /c ""%~dp0netodrive-provider.exe" -run -config "%CFG%" >> "%APPDATA%\NetoDrive\logs\provider.log" 2>&1"
)

echo NetoDrive sync engine:
"%~dp0netodrive-sync.exe" -version 2>nul
if errorlevel 1 echo   (binario antigo - rode Install-NetoDrive.ps1 com Go instalado)

"%~dp0netodrive-sync.exe" -ui -config "%CFG%"
