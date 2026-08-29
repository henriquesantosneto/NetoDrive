using Windows.Security.Cryptography;
using Windows.Storage;
using Windows.Storage.Provider;
using Windows.Storage.Streams;

namespace NetoDriveProvider;

/// <summary>
/// Registra a pasta de sync no Windows (CFAPI + Explorer nativo).
/// </summary>
internal static class SyncRootRegistrar
{
    internal static void Register(AppConfig cfg, string cfgPath)
    {
        SyncRootCleanup.ValidatePath(cfg.LocalFolder);
        CfSyncRoot.StopProviderProcesses();
        SyncRootCleanup.DeepUnregister(cfg.LocalFolder);

        Directory.CreateDirectory(cfg.LocalFolder);

        var syncRootId = Paths.SyncRootId;
        Console.WriteLine($"Registrando sync root: {syncRootId}");
        var rawJson = JsonConfigReader.ReadLocalFolderRawFromFile(cfgPath);
        if (!string.IsNullOrWhiteSpace(rawJson))
            Console.WriteLine($"local_folder (JSON): {rawJson}");
        Console.WriteLine($"local_folder (Explorer): {cfg.LocalFolder}");

        CfSyncRoot.Register(cfg.LocalFolder, syncRootId);

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

        try
        {
            StorageProviderSyncRootManager.Register(info);
        }
        catch (Exception ex)
        {
            throw new InvalidOperationException(
                $"StorageProviderSyncRootManager.Register falhou para {cfg.LocalFolder}: {ex.Message}\n" +
                "Feche todas as janelas do Explorer e encerre netodrive-provider.exe, depois rode -unregister e -register.",
                ex);
        }

        Thread.Sleep(1000);
        SyncRootStatus.ConfirmRegistration(cfg);
    }

    internal static void Unregister(string localFolder)
    {
        SyncRootCleanup.DeepUnregister(localFolder);
    }
}
