using System.Text.Json;

namespace NetoDriveProvider;

/// <summary>
/// Reads placeholder metadata from AppData (written by netodrive-sync) for CFAPI directory population.
/// </summary>
internal static class PlaceholderCatalog
{
    internal static string MetaStoreRoot(AppConfig cfg) => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "NetoDrive",
        "placeholder-meta",
        PlaceholderQueue.SyncRootDataId(cfg.LocalFolder));

    internal static IEnumerable<PlaceholderQueueEntry> AllKnown(AppConfig cfg)
    {
        var pendingDeletes = LocalChangesQueue.ReadAll(cfg);
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var entry in ReadMetaStore(cfg))
        {
            if (pendingDeletes.Contains(entry.Rel))
                continue;
            if (seen.Add(entry.Rel))
                yield return entry;
        }
        foreach (var entry in PlaceholderQueue.PeekPending(cfg))
        {
            if (pendingDeletes.Contains(entry.Rel))
                continue;
            if (seen.Add(entry.Rel))
                yield return entry;
        }
    }

    internal static List<PlaceholderQueueEntry> DirectChildren(AppConfig cfg, string dirRel)
    {
        dirRel = dirRel.Replace('\\', '/').Trim('/');
        if (dirRel == ".")
            dirRel = "";

        var result = new List<PlaceholderQueueEntry>();
        foreach (var entry in AllKnown(cfg))
        {
            var rel = entry.Rel.Replace('\\', '/').Trim('/');
            if (string.IsNullOrEmpty(rel))
                continue;

            if (dirRel.Length == 0)
            {
                if (!rel.Contains('/'))
                    result.Add(entry);
                continue;
            }

            var prefix = dirRel + "/";
            if (!rel.StartsWith(prefix, StringComparison.OrdinalIgnoreCase))
                continue;
            var rest = rel[prefix.Length..];
            if (!rest.Contains('/'))
                result.Add(entry);
        }
        return result;
    }

    internal static void RemoveMeta(AppConfig cfg, string rel)
    {
        rel = rel.Replace('\\', '/').Trim('/');
        if (string.IsNullOrEmpty(rel))
            return;
        var key = MetaKeyFromRel(rel);
        var path = Path.Combine(MetaStoreRoot(cfg), key + ".json");
        try
        {
            if (File.Exists(path))
                File.Delete(path);
        }
        catch (IOException ex)
        {
            Console.Error.WriteLine($"remove meta {rel}: {ex.Message}");
        }
    }

    private static string MetaKeyFromRel(string rel)
    {
        if (rel.Length == 0)
            return "_root";
        return rel.Replace('\\', '/').Replace("/", "__");
    }

    private static IEnumerable<PlaceholderQueueEntry> ReadMetaStore(AppConfig cfg)
    {
        var root = MetaStoreRoot(cfg);
        if (!Directory.Exists(root))
            yield break;

        foreach (var file in Directory.EnumerateFiles(root, "*.json"))
        {
            PlaceholderQueueEntry? entry;
            try
            {
                var json = File.ReadAllText(file);
                using var doc = JsonDocument.Parse(json);
                var rootEl = doc.RootElement;
                var rel = MetaRelFromKey(Path.GetFileNameWithoutExtension(file));
                var hash = rootEl.TryGetProperty("hash", out var h) ? h.GetString() ?? "" : "";
                var size = rootEl.TryGetProperty("size", out var s) && s.TryGetInt64(out var n) ? n : 0L;
                if (string.IsNullOrWhiteSpace(rel) || hash.Length == 0)
                    continue;
                entry = new PlaceholderQueueEntry { Rel = rel, Hash = hash, Size = size };
            }
            catch
            {
                continue;
            }
            yield return entry;
        }
    }

    private static string MetaRelFromKey(string key)
    {
        if (key == "_root")
            return "";
        return key.Replace("__", "/");
    }
}
