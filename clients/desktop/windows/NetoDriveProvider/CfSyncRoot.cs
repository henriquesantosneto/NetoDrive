using System.Diagnostics;
using Vanara.InteropServices;
using Vanara.PInvoke;
using static Vanara.PInvoke.CldApi;

namespace NetoDriveProvider;

/// <summary>
/// Limpeza CFAPI (CfUnregisterSyncRoot) e controle de processos do provider.
/// </summary>
internal static class CfSyncRoot
{
    internal static bool IsRegistered(string localFolder)
    {
        using var mem = SafeHGlobalHandle.CreateFromStructure<CF_SYNC_ROOT_BASIC_INFO>();
        return CfGetSyncRootInfoByPath(
            localFolder,
            CF_SYNC_ROOT_INFO_CLASS.CF_SYNC_ROOT_INFO_BASIC,
            mem,
            mem.Size,
            out _).Succeeded;
    }

    internal static void Unregister(string localFolder)
    {
        var hr = CfUnregisterSyncRoot(localFolder);
        if (hr.Succeeded)
            return;

        var code = (uint)hr;
        // ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT
        if (code is 0x80070186 or 0x80070190)
            return;

        Console.Error.WriteLine($"Aviso: CfUnregisterSyncRoot({localFolder}): {HrMessage(hr)}");
    }

    internal static void StopProviderProcesses()
    {
        foreach (var proc in Process.GetProcessesByName("netodrive-provider"))
        {
            try
            {
                if (proc.HasExited)
                    continue;
                proc.Kill(entireProcessTree: true);
                proc.WaitForExit(5000);
            }
            catch
            {
                // ignore
            }
            finally
            {
                proc.Dispose();
            }
        }
        Thread.Sleep(500);
    }

    internal static string HrMessage(HRESULT hr)
    {
        var code = (uint)hr;
        var hint = code switch
        {
            0x800701DE => " sync root em estado inconsistente (provider aberto ou registro antigo)",
            0x800701B7 or 0x800700B7 => " pasta ja registrada",
            0x80070186 or 0x80070190 => " pasta nao era sync root",
            0x80070178 => " pasta nao e cloud file (registre CFAPI antes do Explorer)",
            _ => "",
        };
        return $"0x{code:X8}{hint}";
    }
}
