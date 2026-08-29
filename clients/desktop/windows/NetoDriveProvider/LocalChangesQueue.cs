namespace NetoDriveProvider;

/// <summary>
/// Local change queue shared with netodrive-sync (CFAPI cannot scan sync root for deletes).
/// </summary>
internal static class LocalChangesQueue
{
    internal static string PendingDeletesPath(AppConfig cfg) => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "NetoDrive",
        "local-changes",
        PlaceholderQueue.SyncRootDataId(cfg.LocalFolder),
        "pending-deletes.txt");

    internal static void EnqueueDelete(AppConfig cfg, string rel)
    {
        rel = rel.Replace('\\', '/').Trim('/');
        if (string.IsNullOrEmpty(rel))
            return;

        var path = PendingDeletesPath(cfg);
        var existing = ReadAll(cfg);
        if (existing.Contains(rel))
            return;

        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        File.AppendAllText(path, rel + Environment.NewLine);
        Console.WriteLine($"local delete enqueued: {rel}");
    }

    internal static HashSet<string> ReadAll(AppConfig cfg)
    {
        var set = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        var path = PendingDeletesPath(cfg);
        if (!File.Exists(path))
            return set;
        foreach (var line in File.ReadAllLines(path))
        {
            var rel = line.Replace('\\', '/').Trim('/');
            if (rel.Length > 0)
                set.Add(rel);
        }
        return set;
    }
}
