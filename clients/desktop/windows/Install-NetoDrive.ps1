# Instala o cliente NetoDrive no Windows (sync + atalho).
# Execute no PowerShell:  .\Install-NetoDrive.ps1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Repo = Resolve-Path (Join-Path $Root "..\..\..")
$InstallDir = Join-Path $env:LOCALAPPDATA "NetoDrive"
$ConfigDir = Join-Path $env:APPDATA "NetoDrive"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

$exeSrc = Join-Path $Repo "dist\netodrive-sync.exe"
if (-not (Test-Path $exeSrc)) {
  Write-Host "Compilando netodrive-sync.exe ..."
  Push-Location (Join-Path $Repo "clients\desktop")
  $env:GOOS = "windows"; $env:GOARCH = "amd64"
  go build -o $exeSrc .\cmd\netodrive-sync
  Pop-Location
}

Copy-Item $exeSrc (Join-Path $InstallDir "netodrive-sync.exe") -Force
Copy-Item (Join-Path $Root "Start-NetoDrive.bat") (Join-Path $InstallDir "Start-NetoDrive.bat") -Force

$cfg = Join-Path $ConfigDir "netodrive.json"
if (-not (Test-Path $cfg)) {
  & (Join-Path $InstallDir "netodrive-sync.exe") -init -config $cfg
  Write-Host "Config criada em $cfg — ajuste server_url para o IP do servidor."
}

$desktop = [Environment]::GetFolderPath("Desktop")
$shortcut = Join-Path $desktop "NetoDrive.lnk"
$w = New-Object -ComObject WScript.Shell
$s = $w.CreateShortcut($shortcut)
$s.TargetPath = Join-Path $InstallDir "Start-NetoDrive.bat"
$s.WorkingDirectory = $InstallDir
$s.Description = "NetoDrive Sync"
$s.Save()

Write-Host "Instalado em $InstallDir"
Write-Host "Atalho: $shortcut"
Write-Host "Inicie pelo atalho NetoDrive na area de trabalho."
