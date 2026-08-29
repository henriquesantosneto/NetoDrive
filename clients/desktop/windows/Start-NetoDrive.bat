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

REM Provider CFAPI: clique abre / menu nativo "Manter neste dispositivo"
if exist "%~dp0netodrive-provider.exe" (
  start "" /B "%~dp0netodrive-provider.exe" -run -config "%CFG%"
)

"%~dp0netodrive-sync.exe" -ui -config "%CFG%"
