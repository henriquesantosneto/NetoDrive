using System.Security.Principal;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace NetoDriveProvider;

internal sealed class AppConfig
{
    [JsonPropertyName("server_url")]
    public string ServerURL { get; set; } = "";

    [JsonPropertyName("token")]
    public string Token { get; set; } = "";

    [JsonPropertyName("device_id")]
    public string DeviceID { get; set; } = "";

    [JsonPropertyName("local_folder")]
    public string LocalFolder { get; set; } = "";

    [JsonPropertyName("on_demand")]
    public bool OnDemand { get; set; } = true;

    public static AppConfig Load(string path)
    {
        var json = File.ReadAllText(path);
        var cfg = JsonSerializer.Deserialize<AppConfig>(json, JsonOptions) ?? new AppConfig();

        // Fallback: snake_case no JSON as vezes nao mapeia sem JsonPropertyName em builds antigos.
        if (string.IsNullOrWhiteSpace(cfg.LocalFolder))
        {
            var raw = JsonConfigReader.ReadLocalFolderRaw(json);
            if (!string.IsNullOrWhiteSpace(raw))
                cfg.LocalFolder = raw;
        }

        cfg.LocalFolder = LocalFolderResolver.Resolve(path, cfg.LocalFolder);
        return cfg;
    }

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true,
        ReadCommentHandling = JsonCommentHandling.Skip,
        AllowTrailingCommas = true,
    };
}

internal static class JsonConfigReader
{
    internal static string? ReadLocalFolderRaw(string json)
    {
        try
        {
            using var doc = JsonDocument.Parse(json);
            foreach (var name in new[] { "local_folder", "LocalFolder", "localFolder" })
            {
                if (!doc.RootElement.TryGetProperty(name, out var el))
                    continue;
                var s = el.GetString()?.Trim();
                if (!string.IsNullOrWhiteSpace(s))
                    return s;
            }
        }
        catch
        {
            // ignore
        }
        return null;
    }

    internal static string? ReadLocalFolderRawFromFile(string cfgPath)
    {
        if (!File.Exists(cfgPath))
            return null;
        return ReadLocalFolderRaw(File.ReadAllText(cfgPath));
    }
}

internal static class LocalFolderResolver
{
    internal static string Resolve(string configPath, string folder)
    {
        folder = (folder ?? "").Trim();
        if (string.IsNullOrWhiteSpace(folder))
        {
            return Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
                "NetoDrive");
        }
        if (folder == "~" || folder.StartsWith("~/", StringComparison.Ordinal) ||
            folder.StartsWith("~\\", StringComparison.Ordinal))
        {
            var home = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
            folder = folder.Length == 1 ? home : Path.Combine(home, folder.Substring(2));
        }
        if (Path.IsPathRooted(folder))
            return Path.GetFullPath(folder);
        var baseDir = Path.GetDirectoryName(configPath);
        if (string.IsNullOrEmpty(baseDir))
            baseDir = Environment.CurrentDirectory;
        return Path.GetFullPath(Path.Combine(baseDir, folder));
    }
}

internal static class Paths
{
    internal static readonly Guid ProviderId = new("8F3E2A1B-4C5D-9E6F-A1B2-C3D4E5F67890");
    internal const string ProviderName = "NetoDrive";

    internal static string ConfigPath => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "NetoDrive",
        "netodrive.json");

    /// Sync root id: Provider!UserSid!Account (formato exigido pelo Windows).
    internal static string SyncRootId
    {
        get
        {
            var sid = WindowsIdentity.GetCurrent().User?.Value ?? "local";
            var user = Environment.UserName;
            return $"{ProviderName}!{sid}!{user}";
        }
    }
}
