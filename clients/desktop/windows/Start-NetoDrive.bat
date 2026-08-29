@echo off
REM NetoDrive Sync para Windows — painel estilo OneDrive + sync em segundo plano
setlocal
cd /d "%~dp0"

if not exist "%APPDATA%\NetoDrive\netodrive.json" (
  echo Criando configuracao inicial...
  mkdir "%APPDATA%\NetoDrive" 2>nul
  "%~dp0netodrive-sync.exe" -init -config "%APPDATA%\NetoDrive\netodrive.json"
  echo.
  echo Edite o arquivo e ajuste server_url para o IP do seu servidor Linux:
  echo   %APPDATA%\NetoDrive\netodrive.json
  notepad "%APPDATA%\NetoDrive\netodrive.json"
)

"%~dp0netodrive-sync.exe" -ui -config "%APPDATA%\NetoDrive\netodrive.json"
