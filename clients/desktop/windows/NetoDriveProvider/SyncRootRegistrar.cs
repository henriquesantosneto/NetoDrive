using Windows.Security.Cryptography;
using Windows.Storage;
using Windows.Storage.Provider;
using Windows.Storage.Streams;

namespace NetoDriveProvider;

/// <summary>
/// Registra a pasta de sync no Windows (Explorer nativo + CFAPI via WinRT).
/// </summary>
internal static class SyncRootRegistrar
{
    internal static void Register(AppConfig cfg)
    {
        Directory.CreateDirectory(cfg.LocalFolder);
        Unregister(cfg.LocalFolder);

        var folder = StorageFolder.GetFolderFromPathAsync(cfg.LocalFolder).AsTask().GetAwaiter().GetResult();
        var iconExe = Path.Combine(AppContext.BaseDirectory, "netodrive-sync.exe");
        if (!File.Exists(iconExe))
        {
            iconExe = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "NetoDrive",
                "netodrive-sync.exe");
        }
        var iconResource = File.Exists(iconExe)
            ? $"{iconExe},0"
            : "%SystemRoot%\\system32\\imageres.dll,1043";

        var syncRootId = Paths.SyncRootId;
        var info = new StorageProviderSyncRootInfo
        {
            Id = syncRootId,
            ProviderId = Paths.ProviderId,
            Path = folder,
            DisplayNameResource = Paths.ProviderName,
            IconResource = iconResource,
            AllowPinning = true,
            ShowSiblingsAsGroup = false,
            HardlinkPolicy = StorageProviderHardlinkPolicy.None,
            HydrationPolicy = StorageProviderHydrationPolicy.Full,
            HydrationPolicyModifier = StorageProviderHydrationPolicyModifier.None,
            PopulationPolicy = StorageProviderPopulationPolicy.AlwaysFull,
            InSyncPolicy = StorageProviderInSyncPolicy.FileCreationTime | StorageProviderInSyncPolicy.DirectoryCreationTime,
            Version = "1.0.0",
            RecycleBinUri = new Uri("https://netodrive.local/recyclebin"),
            Context = CryptographicBuffer.ConvertStringToBinary(syncRootId, BinaryStringEncoding.Utf8),
        };

        StorageProviderSyncRootManager.Register(info);
        Thread.Sleep(1000);
    }

    internal static void Unregister(string localFolder)
    {
        _ = localFolder;
        try
        {
            StorageProviderSyncRootManager.Unregister(Paths.SyncRootId);
        }
        catch
        {
            // best effort
        }
    }
}
