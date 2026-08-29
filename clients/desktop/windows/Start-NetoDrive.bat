@echo off
REM NetoDrive Sync for Windows - OneDrive-style panel + background sync
setlocal
cd /d "%~dp0"

if not exist "%APPDATA%\NetoDrive\netodrive.json" (
  echo Criando configuracao inicial...
  mkdir "%APPDATA%\NetoDrive" 2>nul
  "%~dp0netodrive-sync.exe" -init -config "%APPDATA%\NetoDrive\netodrive.json"
  echo.
  echo Edite o arquivo e ajuste server_url e local_folder:
  echo   server_url  = IP do servidor (ex: http://192.168.1.10:8080)
  echo   local_folder = pasta sincronizada (padrao: Documents\NetoDrive)
  echo   %APPDATA%\NetoDrive\netodrive.json
  notepad "%APPDATA%\NetoDrive\netodrive.json"
)

"%~dp0netodrive-sync.exe" -ui -config "%APPDATA%\NetoDrive\netodrive.json"
