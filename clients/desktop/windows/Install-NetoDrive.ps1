# Install NetoDrive Windows client (sync + desktop shortcut).
# No Go required if netodrive-sync.exe is present in this folder.
# Run in PowerShell:  .\Install-NetoDrive.ps1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Repo = Resolve-Path (Join-Path $Root "..\..\..")
$InstallDir = Join-Path $env:LOCALAPPDATA "NetoDrive"
$ConfigDir = Join-Path $env:APPDATA "NetoDrive"
$SyncExe = Join-Path $InstallDir "netodrive-sync.exe"

function Get-RegAsmPath {
  $regasm = Join-Path ${env:WINDIR} "Microsoft.NET\Framework64\v4.0.30319\RegAsm.exe"
  if (Test-Path $regasm) { return $regasm }
  return $null
}

function Unregister-NetoDriveShell {
  param([string]$ShellDll)
  if (-not (Test-Path -LiteralPath $ShellDll)) { return }
  $regasm = Get-RegAsmPath
  if (-not $regasm) { return }
  Write-Host "Desregistrando menu de contexto anterior..."
  $prevEA = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  & $regasm /u /codebase $ShellDll 2>$null | Out-Null
  $ErrorActionPreference = $prevEA
}

function Restart-ExplorerForShellUpdate {
  Write-Host "Reiniciando Explorer para liberar NetoDriveShell.dll..."
  Stop-Process -Name explorer -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 2
  Start-Process explorer.exe
  Start-Sleep -Seconds 2
}

function Install-NetoDriveShellBuild {
  param(
    [string]$ShellDir,
    [string]$InstallDir
  )
  $shellOut = Join-Path $InstallDir "shell"
  $shellStaging = Join-Path $env:TEMP ("NetoDriveShell-publish-{0}" -f [Guid]::NewGuid().ToString('N'))
  $existingDll = Join-Path $shellOut "NetoDriveShell.dll"

  Unregister-NetoDriveShell -ShellDll $existingDll

  Push-Location $ShellDir
  try {
    dotnet publish -f net48 -c Release -r win-x64 --self-contained false -o $shellStaging
    if ($LASTEXITCODE -ne 0) { throw "Falha ao compilar NetoDriveShell" }
  } finally {
    Pop-Location
  }

  New-Item -ItemType Directory -Force -Path $shellOut | Out-Null

  $maxRetries = 5
  for ($i = 1; $i -le $maxRetries; $i++) {
    try {
      Copy-Item -Path (Join-Path $shellStaging "*") -Destination $shellOut -Recurse -Force -ErrorAction Stop
      break
    } catch {
      if ($i -eq 1) {
        Restart-ExplorerForShellUpdate
      }
      if ($i -eq $maxRetries) {
        Remove-Item $shellStaging -Recurse -Force -ErrorAction SilentlyContinue
        throw "Nao foi possivel atualizar NetoDriveShell.dll (arquivo em uso). Feche o Explorer e rode Install-NetoDrive.ps1 de novo."
      }
      Write-Host "Aguardando liberacao de NetoDriveShell.dll (tentativa $i/$maxRetries)..."
      Start-Sleep -Seconds 2
    }
  }

  Remove-Item $shellStaging -Recurse -Force -ErrorAction SilentlyContinue

  $shellDll = Join-Path $shellOut "NetoDriveShell.dll"
  if (-not (Test-Path $shellDll)) { return }

  $regasm = Get-RegAsmPath
  if ($regasm) {
    & $regasm /codebase $shellDll | Out-Null
    Write-Host "Menu de contexto NetoDrive registrado."
  } else {
    Write-Host "RegAsm nao encontrado - menu de contexto nao registrado."
  }
}

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

Write-Host "Encerrando netodrive-sync em execucao (libera exe e CFAPI)..."
Get-Process netodrive-sync -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

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
  $embedded = Join-Path $Root "netodrive-sync.exe"
  $dist = Join-Path $Repo "dist\netodrive-sync.exe"
  if (Test-Path $embedded) {
    $exeSrc = $embedded
  } elseif (Test-Path $dist) {
    $exeSrc = $dist
  } else {
    throw @"
netodrive-sync.exe nao encontrado no repositorio.

Opcoes:
1) git pull origin main
2) Instale Go (https://go.dev/dl/) e rode Install-NetoDrive.ps1 de novo
"@
  }
  Write-Host "AVISO: Go nao instalado - usando netodrive-sync.exe embarcado." -ForegroundColor Yellow
  $srcVer = & $exeSrc -version 2>&1 | Select-Object -First 1
  if ($srcVer) {
    Write-Host "  Binario embarcado: $srcVer"
  }
}

Write-Host "Usando: $exeSrc"
Copy-Item $exeSrc (Join-Path $InstallDir "netodrive-sync.exe") -Force
$verOut = & (Join-Path $InstallDir "netodrive-sync.exe") -version 2>&1 | Select-Object -First 1
if ($verOut) {
  Write-Host "Versao instalada: $verOut"
  if ($verOut -match 'fast-path-cfapi-v(\d+)' -and [int]$Matches[1] -lt 16) {
    Write-Host ""
    Write-Host "ERRO: binario antigo ($verOut). Rode:" -ForegroundColor Red
    Write-Host "  cd $Repo" -ForegroundColor Yellow
    Write-Host "  git pull origin main" -ForegroundColor Yellow
    Write-Host "  .\clients\desktop\windows\Install-NetoDrive.ps1" -ForegroundColor Yellow
    throw "Atualize o repositorio para obter fast-path-cfapi-v16 ou superior."
  }
} else {
  Write-Host "AVISO: exe sem flag -version (binario antigo). Instale Go e rode Install de novo." -ForegroundColor Yellow
}
if ($go -and $exeSrc -ne (Join-Path $Root "netodrive-sync.exe")) {
  Copy-Item $exeSrc (Join-Path $Root "netodrive-sync.exe") -Force
}
Copy-Item (Join-Path $Root "Start-NetoDrive.bat") (Join-Path $InstallDir "Start-NetoDrive.bat") -Force
Copy-Item (Join-Path $Root "OpenPlaceholder.vbs") (Join-Path $InstallDir "OpenPlaceholder.vbs") -Force

$cfg = Join-Path $ConfigDir "netodrive.json"
if (-not (Test-Path $cfg)) {
  & (Join-Path $InstallDir "netodrive-sync.exe") -init -config $cfg
  Write-Host "Config criada em $cfg"
  Write-Host "Edite server_url e local_folder (padrao: $env:USERPROFILE\NetoDrive, fora do OneDrive)."
  notepad $cfg
} else {
  Write-Host "Config existente preservada: $cfg"
}

# CFAPI: evitar Stat repetido no sync root - marcar clone git no JSON
try {
  if (-not (Test-Path -LiteralPath $cfg)) { throw 'config missing' }
  $cfgText = Get-Content -Raw -LiteralPath $cfg
  $cfgObj = $cfgText | ConvertFrom-Json
  $lf = $null
  if ($cfgObj.local_folder) { $lf = $cfgObj.local_folder }
  elseif ($cfgObj.LocalFolder) { $lf = $cfgObj.LocalFolder }
  if ($lf -and (Test-Path -LiteralPath (Join-Path $lf '.git'))) {
    $hasRepoFlag = $false
    foreach ($p in $cfgObj.PSObject.Properties) {
      if ($p.Name -eq 'is_repo_root') { $hasRepoFlag = $true; break }
    }
    if (-not $hasRepoFlag) {
      $cfgObj | Add-Member -NotePropertyName is_repo_root -NotePropertyValue $true -Force
      $cfgObj | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $cfg -Encoding UTF8
      Write-Host 'Config: is_repo_root=true (local_folder e clone git; necessario com CFAPI).'
    }
  }
}
catch {
  Write-Host "AVISO: nao foi possivel atualizar is_repo_root no config: $_" -ForegroundColor Yellow
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
      Get-Process netodrive-provider -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
      Start-Sleep -Seconds 2
      $prevEA = $ErrorActionPreference
      $ErrorActionPreference = 'Continue'
      & $providerExe -unregister -config $cfg 2>$null | Out-Null
      $regOut = & $providerExe -register -config $cfg 2>&1
      $regExit = $LASTEXITCODE
      $ErrorActionPreference = $prevEA
      $regOut | ForEach-Object { Write-Host $_ }
      if ($regExit -ne 0) {
        Write-Host ""
        Write-Host "AVISO: registro CFAPI falhou (codigo $regExit)." -ForegroundColor Yellow
        if ($regOut) {
          Write-Host "  Detalhe:" -ForegroundColor Yellow
          $regOut | ForEach-Object { Write-Host "    $_" -ForegroundColor Yellow }
        }
        Write-Host "  Feche o Explorer, encerre netodrive-provider e tente:" -ForegroundColor Yellow
        Write-Host ('  {0} -unregister -config "{1}"' -f $providerExe, $cfg) -ForegroundColor Yellow
        Write-Host ('  {0} -register -config "{1}"' -f $providerExe, $cfg) -ForegroundColor Yellow
        Write-Host "  Causa comum: local_folder com barra simples no JSON." -ForegroundColor Yellow
        Write-Host "  Use barras duplas ou normais:" -ForegroundColor Yellow
        Write-Host '    "local_folder": "C:\\Users\\henri\\NetoDrive"' -ForegroundColor Yellow
        Write-Host '    "local_folder": "C:/Users/henri/NetoDrive"' -ForegroundColor Yellow
        Write-Host "  Depois rode Install-NetoDrive.ps1 novamente." -ForegroundColor Yellow
      } else {
        & $providerExe -status -config $cfg
      }
    }
  }
  if (Test-Path $shellDir) {
    Install-NetoDriveShellBuild -ShellDir $shellDir -InstallDir $InstallDir
  }
} else {
  Write-Host "AVISO: .NET SDK nao encontrado. Integracao nativa (clique/manter no dispositivo) nao instalada."
  Write-Host "Instale .NET 8 SDK e rode Install-NetoDrive.ps1 novamente."
}

function Pin-LocalFolderQuickAccess {
  param([string]$Folder)
  if (-not (Test-Path -LiteralPath $Folder)) { return }
  try {
    $shell = New-Object -ComObject Shell.Application
    $dir = $shell.Namespace($Folder)
    if ($dir) {
      $dir.Self.InvokeVerb("pintohome") | Out-Null
      Write-Host "Acesso rapido do Explorer fixado em: $Folder"
    }
  } catch {
    Write-Host "AVISO: nao foi possivel fixar Acesso rapido automaticamente." -ForegroundColor Yellow
  }
}

$localFolder = Get-NetoDriveLocalFolder -CfgPath $cfg
New-Item -ItemType Directory -Force -Path $localFolder | Out-Null
Pin-LocalFolderQuickAccess -Folder $localFolder

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
Write-Host 'Inicie pelo atalho NetoDrive Sync ou NetoDrive (pasta).'
