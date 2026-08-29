using Windows.Security.Cryptography;
using Windows.Storage;
using Windows.Storage.Provider;
using Windows.Storage.Streams;

namespace NetoDriveProvider;

/// <summary>
/// Registra a pasta de sync no Windows (Explorer nativo via WinRT; CFAPI no -run).
/// </summary>
internal static class SyncRootRegistrar
{
    internal static void Register(AppConfig cfg, string cfgPath)
    {
        SyncRootCleanup.ValidatePath(cfg.LocalFolder);
        CfSyncRoot.StopProviderProcesses();
        SyncRootCleanup.DeepUnregister(cfg.LocalFolder);
        Thread.Sleep(1500);

        Directory.CreateDirectory(cfg.LocalFolder);

        var syncRootId = Paths.SyncRootId;
        Console.WriteLine($"Registrando sync root: {syncRootId}");
        var rawJson = JsonConfigReader.ReadLocalFolderRawFromFile(cfgPath);
        if (!string.IsNullOrWhiteSpace(rawJson))
            Console.WriteLine($"local_folder (JSON): {rawJson}");
        Console.WriteLine($"local_folder (Explorer): {cfg.LocalFolder}");

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

        var onDemand = cfg.OnDemand;
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
            PopulationPolicy = onDemand
                ? StorageProviderPopulationPolicy.Full
                : StorageProviderPopulationPolicy.AlwaysFull,
            InSyncPolicy = StorageProviderInSyncPolicy.FileCreationTime | StorageProviderInSyncPolicy.DirectoryCreationTime,
            Version = "1.0.0",
            RecycleBinUri = new Uri("https://netodrive.local/recyclebin"),
            Context = CryptographicBuffer.ConvertStringToBinary(syncRootId, BinaryStringEncoding.Utf8),
        };

        Console.WriteLine("Registrando no Explorer (WinRT)...");
        try
        {
            StorageProviderSyncRootManager.Register(info);
        }
        catch (Exception ex)
        {
            var hr = ex.HResult;
            throw new InvalidOperationException(
                $"StorageProviderSyncRootManager.Register falhou para {cfg.LocalFolder}.\n" +
                $"  Erro: {ex.Message}\n" +
                (hr != 0 ? $"  HRESULT: 0x{hr & 0xFFFFFFFF:X8}\n" : "") +
                "  Feche todas as janelas do Explorer, encerre netodrive-provider.exe,\n" +
                "  depois: netodrive-provider.exe -unregister -config \"%APPDATA%\\NetoDrive\\netodrive.json\"\n" +
                "          netodrive-provider.exe -register -config \"%APPDATA%\\NetoDrive\\netodrive.json\"",
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
