using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using Vanara.InteropServices;
using static Vanara.PInvoke.CldApi;

namespace NetoDriveProvider;

/// <summary>
/// Registro CFAPI de baixo nivel (CfRegisterSyncRoot). Necessario antes de CfConnectSyncRoot e do Explorer.
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

    internal static void Register(string localFolder, string syncRootId)
    {
        var identity = Encoding.UTF8.GetBytes(syncRootId);
        using var identityMem = new SafeCoTaskMemHandle(identity);

        var reg = new CF_SYNC_REGISTRATION
        {
            StructSize = (uint)Marshal.SizeOf<CF_SYNC_REGISTRATION>(),
            ProviderName = Paths.ProviderName,
            ProviderVersion = "1.0.0",
            SyncRootIdentity = identityMem.DangerousGetHandle(),
            SyncRootIdentityLength = (uint)identity.Length,
            FileIdentityLength = 4096,
            ProviderId = Paths.ProviderId,
        };

        var pol = new CF_SYNC_POLICIES
        {
            StructSize = (uint)Marshal.SizeOf<CF_SYNC_POLICIES>(),
            Hydration = new CF_HYDRATION_POLICY
            {
                Primary = CF_HYDRATION_POLICY_PRIMARY.CF_HYDRATION_POLICY_FULL,
                Modifier = CF_HYDRATION_POLICY_MODIFIER.CF_HYDRATION_POLICY_MODIFIER_NONE,
            },
            Population = new CF_POPULATION_POLICY
            {
                Primary = CF_POPULATION_POLICY_PRIMARY.CF_POPULATION_POLICY_PARTIAL,
                Modifier = CF_POPULATION_POLICY_MODIFIER.CF_POPULATION_POLICY_MODIFIER_NONE,
            },
            InSync = CF_INSYNC_POLICY.CF_INSYNC_POLICY_NONE,
            HardLink = CF_HARDLINK_POLICY.CF_HARDLINK_POLICY_NONE,
            PlaceholderManagement = CF_PLACEHOLDER_MANAGEMENT_POLICY.CF_PLACEHOLDER_MANAGEMENT_POLICY_DEFAULT,
        };

        var hr = CfRegisterSyncRoot(localFolder, reg, pol, CF_REGISTER_FLAGS.CF_REGISTER_FLAG_NONE);
        if (hr.Failed)
        {
            hr = CfRegisterSyncRoot(localFolder, reg, pol, CF_REGISTER_FLAGS.CF_REGISTER_FLAG_UPDATE);
        }
        if (hr.Failed)
        {
            throw new InvalidOperationException(
                $"CfRegisterSyncRoot falhou em {localFolder}: {HrMessage(hr)}");
        }
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
