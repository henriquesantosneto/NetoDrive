# Integracao nativa Windows (Explorer)

## Comportamento estilo OneDrive

Com **netodrive-provider** (CFAPI) instalado:

- Arquivos aparecem como **placeholders na nuvem** (icone de nuvem)
- **Duplo clique** baixa e abre automaticamente
- **Botao direito → Manter neste dispositivo** (menu nativo do Windows quando o provider esta ativo)
- **Botao direito → Liberar espaco** volta o arquivo para placeholder

Sem o provider, o cliente usa atalhos `.lnk` + `OpenPlaceholder.vbs` (fallback).

## Instalacao

Requisitos no Windows:

- .NET 8 SDK (provider CFAPI)
- .NET Framework 4.8 (menu SharpShell; ja vem no Windows 10/11)

```powershell
cd clients\desktop\windows
.\Install-NetoDrive.ps1
```

O instalador compila `NetoDriveProvider` e `NetoDriveShell`, registra o sync root e o menu de contexto.

Inicie pelo atalho **NetoDrive** na area de trabalho (sobe provider + sync).

## Menu de contexto extra

Mesmo com CFAPI, o menu **NetoDrive:** (Baixar / Manter / Liberar) aparece em arquivos dentro da pasta de sync.

## Desinstalar sync root

```powershell
& "$env:LOCALAPPDATA\NetoDrive\netodrive-provider.exe" -unregister -config "$env:APPDATA\NetoDrive\netodrive.json"
```
