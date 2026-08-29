@echo off
REM Sobe o servidor NetoDrive neste Windows (porta 8080)
setlocal
cd /d "%~dp0"

set "NETODRIVE_ADDR=0.0.0.0:8080"
set "NETODRIVE_DATA=%LOCALAPPDATA%\NetoDrive\server-data"
set "NETODRIVE_DB=%LOCALAPPDATA%\NetoDrive\server-data\netodrive.db"
set "NETODRIVE_ADMIN_USER=admin"
set "NETODRIVE_ADMIN_PASS=admin123"
set "NETODRIVE_JWT_SECRET=netodrive-windows-dev-secret"

if not exist "%NETODRIVE_DATA%" mkdir "%NETODRIVE_DATA%"

echo.
echo NetoDrive Server
echo   URL:      http://127.0.0.1:8080
echo   Login:    admin / admin123
echo   Dados:    %NETODRIVE_DATA%
echo.
echo Deixe esta janela aberta. Ctrl+C para parar.
echo.

if not exist "%~dp0netodrive-server.exe" (
  echo ERRO: netodrive-server.exe nao encontrado nesta pasta.
  echo Rode: git pull
  pause
  exit /b 1
)

"%~dp0netodrive-server.exe"
pause
