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

REM Provider CFAPI: encerra instancia anterior, re-registra e sobe o provider
if exist "%~dp0netodrive-provider.exe" (
  taskkill /IM netodrive-provider.exe /F >nul 2>&1
  timeout /t 2 /nobreak >nul
  "%~dp0netodrive-provider.exe" -unregister -config "%CFG%"
  "%~dp0netodrive-provider.exe" -register -config "%CFG%"
  "%~dp0netodrive-provider.exe" -status -config "%CFG%"
  start "" /B "%~dp0netodrive-provider.exe" -run -config "%CFG%"
)

"%~dp0netodrive-sync.exe" -ui -config "%CFG%"
