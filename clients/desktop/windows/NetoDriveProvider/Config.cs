using System.Security.Principal;
using System.Text.Json;

namespace NetoDriveProvider;

internal sealed class AppConfig
{
    public string ServerURL { get; set; } = "";
    public string Token { get; set; } = "";
    public string DeviceID { get; set; } = "";
    public string LocalFolder { get; set; } = "";
    public bool OnDemand { get; set; } = true;

    public static AppConfig Load(string path)
    {
        var json = File.ReadAllText(path);
        var cfg = JsonSerializer.Deserialize<AppConfig>(json, new JsonSerializerOptions
        {
            PropertyNameCaseInsensitive = true,
        }) ?? new AppConfig();
        if (string.IsNullOrWhiteSpace(cfg.LocalFolder))
        {
            cfg.LocalFolder = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
                "NetoDrive");
        }
        cfg.LocalFolder = Path.GetFullPath(cfg.LocalFolder);
        return cfg;
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
