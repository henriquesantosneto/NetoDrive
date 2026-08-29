# Install NetoDrive Windows client (sync + desktop shortcut).
# No Go required if netodrive-sync.exe is present in this folder.
# Run in PowerShell:  .\Install-NetoDrive.ps1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Repo = Resolve-Path (Join-Path $Root "..\..\..")
$InstallDir = Join-Path $env:LOCALAPPDATA "NetoDrive"
$ConfigDir = Join-Path $env:APPDATA "NetoDrive"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

# Prefer bundled exe (no Go needed)
$candidates = @(
  (Join-Path $Root "netodrive-sync.exe"),
  (Join-Path $Repo "dist\netodrive-sync.exe"),
  (Join-Path $InstallDir "netodrive-sync.exe")
)

$exeSrc = $null
foreach ($c in $candidates) {
  if (Test-Path $c) {
    $exeSrc = $c
    break
  }
}

if (-not $exeSrc) {
  $go = Get-Command go -ErrorAction SilentlyContinue
  if ($go) {
    Write-Host "Compilando netodrive-sync.exe ..."
    $exeSrc = Join-Path $Root "netodrive-sync.exe"
    Push-Location (Join-Path $Repo "clients\desktop")
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -o $exeSrc .\cmd\netodrive-sync
    if ($LASTEXITCODE -ne 0) {
      Pop-Location
      throw "Falha ao compilar netodrive-sync.exe"
    }
    Pop-Location
  } else {
    throw @"
netodrive-sync.exe nao encontrado.

Opcoes:
1) git pull  (o exe deve vir em clients\desktop\windows\)
2) Baixe/copie netodrive-sync.exe para:
   $Root
"@
  }
}

Write-Host "Usando: $exeSrc"
Copy-Item $exeSrc (Join-Path $InstallDir "netodrive-sync.exe") -Force
Copy-Item (Join-Path $Root "Start-NetoDrive.bat") (Join-Path $InstallDir "Start-NetoDrive.bat") -Force
Copy-Item (Join-Path $Root "OpenPlaceholder.vbs") (Join-Path $InstallDir "OpenPlaceholder.vbs") -Force

$cfg = Join-Path $ConfigDir "netodrive.json"
if (-not (Test-Path $cfg)) {
  & (Join-Path $InstallDir "netodrive-sync.exe") -init -config $cfg
  Write-Host "Config criada em $cfg"
  Write-Host "Edite server_url e local_folder (padrao: Documents\NetoDrive)."
  notepad $cfg
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
