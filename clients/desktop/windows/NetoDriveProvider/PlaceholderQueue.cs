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

    internal static void ProcessPending(AppConfig cfg)
    {
        var path = QueuePath(cfg);
        if (!File.Exists(path))
            return;

        List<string> lines;
        lock (Gate)
        {
            lines = ReadAndClear(path);
        }
        if (lines.Count == 0)
            return;

        var retry = new List<string>();
        foreach (var line in lines)
        {
            if (string.IsNullOrWhiteSpace(line))
                continue;
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
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"placeholder queue {entry.Rel}: {ex.Message}");
                retry.Add(line);
            }
        }

        if (retry.Count == 0)
            return;
        lock (Gate)
        {
            AppendLines(path, retry);
        }
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
