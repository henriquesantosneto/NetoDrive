using System.Text.Json;
using System.Text.Json.Serialization;

namespace NetoDriveProvider;

internal sealed class ProviderCommandEntry
{
    [JsonPropertyName("op")]
    public string Op { get; set; } = "";

    [JsonPropertyName("rel")]
    public string Rel { get; set; } = "";
}

/// <summary>
/// Pin/hydrate/dehydrate requests from netodrive-sync (must run in the connected -run process).
/// </summary>
internal static class ProviderCommandQueue
{
    private static readonly object Gate = new();

    internal static string QueuePath(AppConfig cfg) => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "NetoDrive",
        "provider-commands",
        PlaceholderQueue.SyncRootDataId(cfg.LocalFolder),
        "pending.jsonl");

    internal static int ProcessPending(AppConfig cfg, int maxItems = 8)
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
            ProviderCommandEntry? entry;
            try
            {
                entry = JsonSerializer.Deserialize<ProviderCommandEntry>(line);
            }
            catch
            {
                continue;
            }
            if (entry == null || string.IsNullOrWhiteSpace(entry.Op) || string.IsNullOrWhiteSpace(entry.Rel))
                continue;
            try
            {
                Execute(cfg, entry.Op, entry.Rel);
                processed++;
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"provider cmd {entry.Op} {entry.Rel}: {ex.Message}");
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

    private static void Execute(AppConfig cfg, string op, string rel)
    {
        switch (op.Trim().ToLowerInvariant())
        {
            case "pin":
                PlaceholderManager.Pin(cfg, rel);
                return;
            case "hydrate":
                PlaceholderManager.Hydrate(cfg, rel);
                return;
            case "dehydrate":
                PlaceholderManager.Dehydrate(cfg, rel);
                return;
            default:
                throw new InvalidOperationException($"comando desconhecido: {op}");
        }
    }

    private static List<string> ReadAndClear(string path)
    {
        var lines = new List<string>();
        try
        {
            using var fs = new FileStream(path, FileMode.Open, FileAccess.ReadWrite, FileShare.None);
            using (var reader = new StreamReader(fs, leaveOpen: true))
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
            // sync may be appending
        }
        return lines;
    }

    private static void AppendLines(string path, IEnumerable<string> lines)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        using var fs = new FileStream(path, FileMode.OpenOrCreate, FileAccess.Write, FileShare.Read);
        fs.Seek(0, SeekOrigin.End);
        using var writer = new StreamWriter(fs);
        foreach (var line in lines)
            writer.WriteLine(line);
    }
}
