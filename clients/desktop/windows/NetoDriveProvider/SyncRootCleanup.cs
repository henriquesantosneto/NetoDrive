using Microsoft.Win32;
using Windows.Storage.Provider;

namespace NetoDriveProvider;

internal static class SyncRootCleanup
{
    internal static void ValidatePath(string localFolder)
    {
        localFolder = Normalize(localFolder);
        foreach (var env in new[] { "OneDrive", "OneDriveCommercial", "OneDriveConsumer" })
        {
            var od = Environment.GetEnvironmentVariable(env);
            if (string.IsNullOrWhiteSpace(od))
                continue;
            od = Normalize(od);
            if (localFolder.Equals(od, StringComparison.OrdinalIgnoreCase) ||
                localFolder.StartsWith(od + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
            {
                var suggested = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), "NetoDrive");
                throw new InvalidOperationException(
                    "A pasta de sync nao pode ficar dentro do OneDrive.\n" +
                    $"  Atual: {localFolder}\n" +
                    $"  OneDrive: {od}\n" +
                    $"  Use no netodrive.json: \"local_folder\": \"{suggested}\"");
            }
        }
    }

    internal static void DeepUnregister(string localFolder)
    {
        localFolder = Normalize(localFolder);
        var installDir = Normalize(SyncRootStatus.InstallDir);

        try
        {
            foreach (var root in StorageProviderSyncRootManager.GetCurrentSyncRoots())
            {
                try
                {
                    var id = root.Id;
                    var path = root.Path?.Path;
                    if (string.IsNullOrEmpty(id))
                        continue;
                    var remove = id.StartsWith(Paths.ProviderName + "!", StringComparison.OrdinalIgnoreCase);
                    if (!remove && !string.IsNullOrEmpty(path))
                    {
                        var norm = Normalize(path);
                        remove = norm.Equals(localFolder, StringComparison.OrdinalIgnoreCase) ||
                                 norm.Equals(installDir, StringComparison.OrdinalIgnoreCase);
                    }
                    if (remove)
                        StorageProviderSyncRootManager.Unregister(id);
                }
                catch
                {
                    // continue
                }
            }
        }
        catch
        {
            // ignore
        }

        try
        {
            StorageProviderSyncRootManager.Unregister(Paths.SyncRootId);
        }
        catch
        {
            // ignore
        }

        PurgeRegistry(localFolder, installDir);
        Thread.Sleep(500);
    }

    private static void PurgeRegistry(string localFolder, string installDir)
    {
        const string basePath = @"Software\Microsoft\Windows\CurrentVersion\Explorer\SyncRootManager";
        try
        {
            using var mgr = Registry.CurrentUser.OpenSubKey(basePath, writable: true);
            if (mgr == null)
                return;

            foreach (var name in mgr.GetSubKeyNames())
            {
                var remove = name.StartsWith(Paths.ProviderName + "!", StringComparison.OrdinalIgnoreCase);
                if (!remove)
                {
                    try
                    {
                        using var sk = mgr.OpenSubKey(name);
                        var userRoot = sk?.GetValue("UserSyncRootPath") as string;
                        if (string.IsNullOrEmpty(userRoot))
                            continue;
                        var norm = Normalize(userRoot);
                        remove = norm.Equals(localFolder, StringComparison.OrdinalIgnoreCase) ||
                                 norm.Equals(installDir, StringComparison.OrdinalIgnoreCase);
                    }
                    catch
                    {
                        // ignore
                    }
                }
                if (remove)
                {
                    try
                    {
                        mgr.DeleteSubKeyTree(name, false);
                    }
                    catch
                    {
                        // ignore
                    }
                }
            }
        }
        catch
        {
            // ignore
        }
    }

    private static string Normalize(string path)
    {
        try
        {
            return Path.GetFullPath(path).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        }
        catch
        {
            return path.TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        }
    }
}
