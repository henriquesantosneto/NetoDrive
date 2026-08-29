using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace NetoDriveProvider;

internal sealed class PlaceholderQueueEntry
{
    [JsonPropertyName("rel")]
    public string Rel { get; set; } = "";

    [JsonPropertyName("hash")]
    public string Hash { get; set; } = "";

    [JsonPropertyName("size")]
    public long Size { get; set; }
}

/// <summary>
/// Placeholders are enqueued by netodrive-sync and created here in the single -run CFAPI process
/// (avoids deadlocks from a second provider subprocess touching the sync root).
/// </summary>
internal static class PlaceholderQueue
{
    private static readonly object Gate = new();

    internal static string SyncRootDataId(string localFolder)
    {
        var normalized = Path.GetFullPath(localFolder).TrimEnd('\\', '/').ToLowerInvariant();
        var hash = SHA256.HashData(Encoding.UTF8.GetBytes(normalized));
        return Convert.ToHexString(hash.AsSpan(0, 8)).ToLowerInvariant();
    }

    internal static string QueuePath(AppConfig cfg) => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "NetoDrive",
        "placeholder-queue",
        SyncRootDataId(cfg.LocalFolder),
        "pending.jsonl");

    internal static IReadOnlyList<PlaceholderQueueEntry> PeekPending(AppConfig cfg)
    {
        var path = QueuePath(cfg);
        if (!File.Exists(path))
            return Array.Empty<PlaceholderQueueEntry>();

        var entries = new List<PlaceholderQueueEntry>();
        try
        {
            foreach (var line in File.ReadAllLines(path))
            {
                if (string.IsNullOrWhiteSpace(line))
                    continue;
                try
                {
                    var entry = JsonSerializer.Deserialize<PlaceholderQueueEntry>(line);
                    if (entry != null && !string.IsNullOrWhiteSpace(entry.Rel))
                        entries.Add(entry);
                }
                catch
                {
                    // skip bad line
                }
            }
        }
        catch (IOException)
        {
            // sync may be writing
        }
        return entries;
    }

    internal static int ProcessPending(AppConfig cfg, int maxItems = 20)
    {
        var path = QueuePath(cfg);
        if (!File.Exists(path))
            return 0;

        List<string> lines;
        lock (Gate)
        {
            lines = ReadAndClear(path);
        }
        if (lines.Count == 0)
            return 0;

        var retry = new List<string>();
        var processed = 0;
        foreach (var line in lines)
        {
            if (string.IsNullOrWhiteSpace(line))
                continue;
            if (processed >= maxItems)
            {
                retry.Add(line);
                continue;
            }
            PlaceholderQueueEntry? entry;
            try
            {
                entry = JsonSerializer.Deserialize<PlaceholderQueueEntry>(line);
            }
            catch
            {
                continue;
            }
            if (entry == null || string.IsNullOrWhiteSpace(entry.Rel))
                continue;
            try
            {
                PlaceholderManager.Create(cfg, entry.Rel, entry.Hash, entry.Size);
                processed++;
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"placeholder queue {entry.Rel}: {ex.Message}");
                retry.Add(line);
            }
        }

        if (retry.Count > 0)
        {
            lock (Gate)
            {
                AppendLines(path, retry);
            }
        }
        return processed;
    }

    internal static void RemoveRel(AppConfig cfg, string rel)
    {
        rel = rel.Replace('\\', '/').Trim('/');
        if (string.IsNullOrEmpty(rel))
            return;

        var path = QueuePath(cfg);
        List<string> lines;
        lock (Gate)
        {
            lines = ReadAllLines(path);
        }
        if (lines.Count == 0)
            return;

        var kept = new List<string>();
        var removed = false;
        foreach (var line in lines)
        {
            if (string.IsNullOrWhiteSpace(line))
                continue;
            try
            {
                var entry = JsonSerializer.Deserialize<PlaceholderQueueEntry>(line);
                if (entry != null &&
                    string.Equals(entry.Rel.Replace('\\', '/').Trim('/'), rel, StringComparison.OrdinalIgnoreCase))
                {
                    removed = true;
                    continue;
                }
            }
            catch
            {
                // keep malformed lines
            }
            kept.Add(line);
        }
        if (!removed)
            return;

        lock (Gate)
        {
            Rewrite(path, kept);
        }
    }

    internal static void RenameRel(AppConfig cfg, string fromRel, string toRel)
    {
        fromRel = fromRel.Replace('\\', '/').Trim('/');
        toRel = toRel.Replace('\\', '/').Trim('/');
        if (string.IsNullOrEmpty(fromRel) || string.IsNullOrEmpty(toRel) ||
            string.Equals(fromRel, toRel, StringComparison.OrdinalIgnoreCase))
        {
            return;
        }

        var path = QueuePath(cfg);
        List<string> lines;
        lock (Gate)
        {
            lines = ReadAllLines(path);
        }
        if (lines.Count == 0)
            return;

        var changed = false;
        for (var i = 0; i < lines.Count; i++)
        {
            try
            {
                var entry = JsonSerializer.Deserialize<PlaceholderQueueEntry>(lines[i]);
                if (entry == null || string.IsNullOrWhiteSpace(entry.Rel))
                    continue;
                if (!string.Equals(entry.Rel.Replace('\\', '/').Trim('/'), fromRel, StringComparison.OrdinalIgnoreCase))
                    continue;
                entry.Rel = toRel;
                lines[i] = JsonSerializer.Serialize(entry);
                changed = true;
            }
            catch
            {
                // keep line
            }
        }
        if (!changed)
            return;

        lock (Gate)
        {
            Rewrite(path, lines);
        }
    }

    private static List<string> ReadAllLines(string path)
    {
        if (!File.Exists(path))
            return new List<string>();
        try
        {
            return File.ReadAllLines(path)
                .Where(line => !string.IsNullOrWhiteSpace(line))
                .ToList();
        }
        catch (IOException)
        {
            return new List<string>();
        }
    }

    private static void Rewrite(string path, List<string> lines)
    {
        if (lines.Count == 0)
        {
            if (File.Exists(path))
                File.Delete(path);
            return;
        }
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        File.WriteAllLines(path, lines);
    }

    private static List<string> ReadAndClear(string path)
    {
        var lines = new List<string>();
        try
        {
            using var fs = new FileStream(path, FileMode.Open, FileAccess.ReadWrite, FileShare.None);
            using (var reader = new StreamReader(fs, Encoding.UTF8, detectEncodingFromByteOrderMarks: true, leaveOpen: true))
            {
                string? line;
                while ((line = reader.ReadLine()) != null)
                {
                    if (!string.IsNullOrWhiteSpace(line))
                        lines.Add(line);
                }
            }
            fs.SetLength(0);
        }
        catch (IOException)
        {
            // Go sync may be appending; try again on next tick.
        }
        return lines;
    }

    private static void AppendLines(string path, IEnumerable<string> lines)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        using var fs = new FileStream(path, FileMode.OpenOrCreate, FileAccess.Write, FileShare.Read);
        fs.Seek(0, SeekOrigin.End);
        using var writer = new StreamWriter(fs, Encoding.UTF8);
        foreach (var line in lines)
            writer.WriteLine(line);
    }
}
