# Install NetoDrive Windows client (sync + desktop shortcut).
# No Go required if netodrive-sync.exe is present in this folder.
# Run in PowerShell:  .\Install-NetoDrive.ps1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Repo = Resolve-Path (Join-Path $Root "..\..\..")
$InstallDir = Join-Path $env:LOCALAPPDATA "NetoDrive"
$ConfigDir = Join-Path $env:APPDATA "NetoDrive"
$SyncExe = Join-Path $InstallDir "netodrive-sync.exe"

function Get-NetoDriveLocalFolder {
  param([string]$CfgPath)
  if (Test-Path $SyncExe) {
    $prevEA = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
      $resolved = & $SyncExe -print-local-folder -config $CfgPath 2>$null | Select-Object -First 1
      if ($LASTEXITCODE -eq 0 -and $resolved) {
        return "$resolved".Trim()
      }
    } catch {
      # exe antigo sem -print-local-folder; usa JSON abaixo
    } finally {
      $ErrorActionPreference = $prevEA
    }
  }
  if (-not (Test-Path $CfgPath)) {
    return (Join-Path $env:USERPROFILE "NetoDrive")
  }
  try {
    $j = Get-Content $CfgPath -Raw | ConvertFrom-Json
    $folder = if ($j.local_folder) { $j.local_folder } else { $j.LocalFolder }
    if (-not $folder) { return (Join-Path $env:USERPROFILE "NetoDrive") }
    $folder = "$folder".Trim()
    if ($folder -eq "~") { return $env:USERPROFILE }
    if ($folder -match '^~[\\/]') { return (Join-Path $env:USERPROFILE $folder.Substring(2)) }
    if (-not [System.IO.Path]::IsPathRooted($folder)) {
      $folder = Join-Path (Split-Path $CfgPath -Parent) $folder
    }
    return [System.IO.Path]::GetFullPath($folder)
  } catch {
    return (Join-Path $env:USERPROFILE "NetoDrive")
  }
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

# Prefer compilar do fonte quando Go estiver disponivel (exe embarcado pode estar desatualizado)
$go = Get-Command go -ErrorAction SilentlyContinue
$exeSrc = $null

if ($go) {
  Write-Host "Compilando netodrive-sync.exe (fonte atual) ..."
  $built = Join-Path $InstallDir "netodrive-sync.exe"
  Push-Location (Join-Path $Repo "clients\desktop")
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  go build -o $built .\cmd\netodrive-sync
  if ($LASTEXITCODE -ne 0) {
    Pop-Location
    throw "Falha ao compilar netodrive-sync.exe"
  }
  Pop-Location
  $exeSrc = $built
} else {
  $candidates = @(
    (Join-Path $Root "netodrive-sync.exe"),
    (Join-Path $Repo "dist\netodrive-sync.exe"),
    (Join-Path $InstallDir "netodrive-sync.exe")
  )
  foreach ($c in $candidates) {
    if (Test-Path $c) {
      $exeSrc = $c
      break
    }
  }
  if (-not $exeSrc) {
    throw @"
netodrive-sync.exe nao encontrado.

Opcoes:
1) Instale Go e rode Install-NetoDrive.ps1 de novo (compila automaticamente)
2) Copie netodrive-sync.exe para:
   $Root
"@
  }
}

Write-Host "Usando: $exeSrc"
if ($exeSrc -ne (Join-Path $InstallDir "netodrive-sync.exe")) {
  Copy-Item $exeSrc (Join-Path $InstallDir "netodrive-sync.exe") -Force
}
Copy-Item (Join-Path $Root "Start-NetoDrive.bat") (Join-Path $InstallDir "Start-NetoDrive.bat") -Force
Copy-Item (Join-Path $Root "OpenPlaceholder.vbs") (Join-Path $InstallDir "OpenPlaceholder.vbs") -Force

$cfg = Join-Path $ConfigDir "netodrive.json"
if (-not (Test-Path $cfg)) {
  & (Join-Path $InstallDir "netodrive-sync.exe") -init -config $cfg
  Write-Host "Config criada em $cfg"
  Write-Host "Edite server_url e local_folder (padrao: $env:USERPROFILE\NetoDrive, fora do OneDrive)."
  notepad $cfg
}

# Build/install CFAPI provider + menu de contexto (requer .NET 8 SDK no Windows)
$dotnet = Get-Command dotnet -ErrorAction SilentlyContinue
if ($dotnet) {
  Write-Host "Compilando integracao nativa do Explorer (CFAPI + menu)..."
  $providerDir = Join-Path $Root "NetoDriveProvider"
  $shellDir = Join-Path $Root "NetoDriveShell"
  if (Test-Path $providerDir) {
    Push-Location $providerDir
    dotnet publish -c Release -r win-x64 --self-contained false -o $InstallDir
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "Falha ao compilar netodrive-provider" }
    Pop-Location
    $providerExe = Join-Path $InstallDir "netodrive-provider.exe"
    if (Test-Path $providerExe) {
      & $providerExe -unregister -config $cfg 2>$null
      $regOut = & $providerExe -register -config $cfg 2>&1
      $regOut | ForEach-Object { Write-Host $_ }
      if ($LASTEXITCODE -ne 0) {
        Write-Host ""
        Write-Host "AVISO: registro CFAPI falhou." -ForegroundColor Yellow
        Write-Host "  Causa comum: pasta dentro do OneDrive (Documents)." -ForegroundColor Yellow
        $suggested = Join-Path $env:USERPROFILE "NetoDrive"
        Write-Host "  Edite $cfg e defina:" -ForegroundColor Yellow
        Write-Host "    `"local_folder`": `"$suggested`"" -ForegroundColor Yellow
        Write-Host "  Depois rode Install-NetoDrive.ps1 novamente." -ForegroundColor Yellow
      } else {
        & $providerExe -status -config $cfg
      }
    }
  }
  if (Test-Path $shellDir) {
    Push-Location $shellDir
    dotnet publish -f net48 -c Release -r win-x64 --self-contained false -o (Join-Path $InstallDir "shell")
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "Falha ao compilar NetoDriveShell" }
    Pop-Location
    $shellDll = Join-Path $InstallDir "shell\NetoDriveShell.dll"
    if (Test-Path $shellDll) {
      $regasm = Join-Path ${env:WINDIR} "Microsoft.NET\Framework64\v4.0.30319\RegAsm.exe"
      if (Test-Path $regasm) {
        & $regasm /codebase $shellDll | Out-Null
        Write-Host "Menu de contexto NetoDrive registrado."
      } else {
        Write-Host "RegAsm nao encontrado - menu de contexto nao registrado."
      }
    }
  }
} else {
  Write-Host "AVISO: .NET SDK nao encontrado. Integracao nativa (clique/manter no dispositivo) nao instalada."
  Write-Host "Instale .NET 8 SDK e rode Install-NetoDrive.ps1 novamente."
}

$localFolder = Get-NetoDriveLocalFolder -CfgPath $cfg
New-Item -ItemType Directory -Force -Path $localFolder | Out-Null

$desktop = [Environment]::GetFolderPath("Desktop")
$w = New-Object -ComObject WScript.Shell

# Atalho na area de trabalho abre a PASTA DE SYNC (local_folder), nao a pasta do programa
$folderShortcut = Join-Path $desktop "NetoDrive.lnk"
$s = $w.CreateShortcut($folderShortcut)
$s.TargetPath = Join-Path $env:WINDIR "explorer.exe"
$s.Arguments = "`"$localFolder`""
$s.IconLocation = "$(Join-Path $InstallDir 'netodrive-sync.exe'),0"
$s.Description = "Pasta NetoDrive (local_folder)"
$s.Save()

# Atalho separado para subir sync + painel
$appShortcut = Join-Path $desktop "NetoDrive Sync.lnk"
$a = $w.CreateShortcut($appShortcut)
$a.TargetPath = Join-Path $InstallDir "Start-NetoDrive.bat"
$a.WorkingDirectory = $InstallDir
$a.IconLocation = "$(Join-Path $InstallDir 'netodrive-sync.exe'),0"
$a.Description = "NetoDrive Sync (painel + provider)"
$a.Save()

Write-Host "Instalado em $InstallDir"
Write-Host "Pasta de sync (local_folder): $localFolder"
Write-Host "Atalho pasta: $folderShortcut"
Write-Host "Atalho app:   $appShortcut"
Write-Host "Inicie pelo atalho NetoDrive Sync ou NetoDrive (pasta)."
