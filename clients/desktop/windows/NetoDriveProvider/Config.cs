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
        var prepared = JsonConfigReader.PrepareJson(json);
        var repaired = !string.Equals(json, prepared, StringComparison.Ordinal);

        AppConfig cfg;
        try
        {
            cfg = JsonSerializer.Deserialize<AppConfig>(prepared, JsonOptions) ?? new AppConfig();
        }
        catch (JsonException ex)
        {
            cfg = new AppConfig();
            var raw = JsonConfigReader.ExtractLocalFolderRegex(json);
            if (string.IsNullOrWhiteSpace(raw))
                throw new InvalidOperationException(
                    $"JSON invalido em {path}: {ex.Message}\n" +
                    "Corrija local_folder com barras duplas, ex.:\n" +
                    "  \"local_folder\": \"C:\\\\Users\\\\henri\\\\NetoDrive\"\n" +
                    "ou use barras normais: \"C:/Users/henri/NetoDrive\"",
                    ex);
            cfg.LocalFolder = raw;
            repaired = true;
        }

        if (string.IsNullOrWhiteSpace(cfg.LocalFolder))
        {
            var raw = JsonConfigReader.ReadLocalFolderRaw(json);
            if (!string.IsNullOrWhiteSpace(raw))
                cfg.LocalFolder = raw;
        }

        cfg.LocalFolder = LocalFolderResolver.Resolve(path, cfg.LocalFolder);

        if (repaired)
        {
            Console.Error.WriteLine(
                $"AVISO: local_folder no JSON usa barras simples invalidas em {path}. " +
                "Use barras duplas ou normais, ex.: \"local_folder\": \"C:/Users/henri/NetoDrive\"");
        }

        return cfg;
    }

    private static void TryPersistPreparedJson(string path, string original, string prepared)
    {
        // Intentionally do not rewrite user config on load/register.
        _ = path;
        _ = original;
        _ = prepared;
    }

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true,
        ReadCommentHandling = JsonCommentHandling.Skip,
        AllowTrailingCommas = true,
    };
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
