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
                    if (!string.IsNullOrEmpty(path) &&
                        Normalize(path).Equals(localFolder, StringComparison.OrdinalIgnoreCase))
                    {
                        StorageProviderSyncRootManager.Unregister(id);
                    }
                    else if (id.StartsWith(Paths.ProviderName + "!", StringComparison.OrdinalIgnoreCase))
                    {
                        StorageProviderSyncRootManager.Unregister(id);
                    }
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

        PurgeRegistry(localFolder);
        Thread.Sleep(500);
    }

    private static void PurgeRegistry(string localFolder)
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
                        if (!string.IsNullOrEmpty(userRoot) &&
                            Normalize(userRoot).Equals(localFolder, StringComparison.OrdinalIgnoreCase))
                            remove = true;
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
