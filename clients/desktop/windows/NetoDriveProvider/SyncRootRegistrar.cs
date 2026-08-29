using System.Runtime.InteropServices;
using Vanara.PInvoke;
using Windows.Security.Cryptography;
using Windows.Storage;
using Windows.Storage.Provider;
using Windows.Storage.Streams;
using static Vanara.PInvoke.CldApi;

namespace NetoDriveProvider;

/// <summary>
/// Registra a pasta de sync como sync root do Windows (Explorer nativo + pin/despejo).
/// </summary>
internal static class SyncRootRegistrar
{
    internal static void Register(AppConfig cfg)
    {
        Directory.CreateDirectory(cfg.LocalFolder);

        var folder = StorageFolder.GetFolderFromPathAsync(cfg.LocalFolder).AsTask().GetAwaiter().GetResult();
        var icon = Path.Combine(AppContext.BaseDirectory, "netodrive-sync.exe");
        if (!File.Exists(icon))
            icon = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "NetoDrive", "netodrive-sync.exe");

        var info = new StorageProviderSyncRootInfo
        {
            Id = Paths.SyncRootId,
            ProviderId = Paths.ProviderId,
            Path = folder,
            DisplayNameResource = "NetoDrive",
            IconResource = icon,
            AllowPinning = true,
            ShowSiblingsAsGroup = false,
            HydrationPolicy = StorageProviderHydrationPolicy.Full,
            PopulationPolicy = StorageProviderPopulationPolicy.Full,
            InSyncPolicy = StorageProviderInSyncPolicy.FileCreationTime | StorageProviderInSyncPolicy.FileLastWriteTime,
            Version = "1.0",
            Context = CryptographicBuffer.ConvertStringToBinary("NetoDrive", BinaryStringEncoding.Utf8),
        };

        StorageProviderSyncRootManager.Register(info);

        var reg = new CF_SYNC_REGISTRATION
        {
            StructSize = (uint)Marshal.SizeOf<CF_SYNC_REGISTRATION>(),
            ProviderName = "NetoDrive",
            ProviderVersion = "1.0",
            ProviderId = Paths.ProviderId,
        };

        var pol = new CF_SYNC_POLICIES
        {
            StructSize = (uint)Marshal.SizeOf<CF_SYNC_POLICIES>(),
            Hydration = new CF_HYDRATION_POLICY
            {
                Primary = CF_HYDRATION_POLICY_PRIMARY.CF_HYDRATION_POLICY_FULL,
                Modifier = CF_HYDRATION_POLICY_MODIFIER.CF_HYDRATION_POLICY_MODIFIER_AUTO_DEHYDRATION_ALLOWED,
            },
            Population = new CF_POPULATION_POLICY
            {
                Primary = CF_POPULATION_POLICY_PRIMARY.CF_POPULATION_POLICY_FULL,
                Modifier = CF_POPULATION_POLICY_MODIFIER.CF_POPULATION_POLICY_MODIFIER_NONE,
            },
            InSync = CF_INSYNC_POLICY.CF_INSYNC_POLICY_NONE,
            HardLink = CF_HARDLINK_POLICY.CF_HARDLINK_POLICY_NONE,
        };

        var hr = CfRegisterSyncRoot(cfg.LocalFolder, reg, pol, CF_REGISTER_FLAGS.CF_REGISTER_FLAG_NONE);
        if (hr.Failed)
            throw new InvalidOperationException($"CfRegisterSyncRoot falhou: {hr}");
    }

    internal static void Unregister(string localFolder)
    {
        try
        {
            StorageProviderSyncRootManager.Unregister(Paths.SyncRootId);
        }
        catch
        {
            // best effort
        }
        CfUnregisterSyncRoot(localFolder);
    }
}
