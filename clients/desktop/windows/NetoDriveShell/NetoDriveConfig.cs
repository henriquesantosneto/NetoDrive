using System;
using System.IO;
using System.Text.Json;
using System.Text.RegularExpressions;
using System.Windows.Forms;

namespace NetoDriveShell;

internal static class NetoDriveConfig
{
    private static readonly string ConfigPath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "NetoDrive",
        "netodrive.json");

    private static readonly string InstallDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "NetoDrive");

    private static readonly Regex LocalFolderPattern = new(
        @"""((?:local_folder|LocalFolder))""\s*:\s*""([^""]*)""",
        RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);

    internal static string SyncExe => Path.Combine(InstallDir, "netodrive-sync.exe");

    internal static bool TryGetLocalFolder(out string localFolder)
    {
        localFolder = "";
        if (!File.Exists(ConfigPath))
            return false;

        var raw = File.ReadAllText(ConfigPath);
        try
        {
            var prepared = PrepareJson(raw);
            using var doc = JsonDocument.Parse(prepared);
            if (doc.RootElement.TryGetProperty("local_folder", out var lf) ||
                doc.RootElement.TryGetProperty("LocalFolder", out lf))
            {
                localFolder = lf.GetString() ?? "";
            }
        }
        catch
        {
            localFolder = ExtractLocalFolderFromRaw(raw);
        }

        if (string.IsNullOrWhiteSpace(localFolder))
            return false;

        localFolder = ResolveLocalFolder(ConfigPath, localFolder.Trim());
        return true;
    }

    internal static bool IsUnderSyncRoot(string path, out string relative)
    {
        relative = "";
        if (!TryGetLocalFolder(out var root))
            return false;
        try
        {
            var full = Path.GetFullPath(path);
            root = Path.GetFullPath(root);
            if (!full.StartsWith(root, StringComparison.OrdinalIgnoreCase))
                return false;
            relative = PathUtil.GetRelativePath(root, full).Replace('\\', '/').Trim('/');
            return relative.Length > 0 && !relative.StartsWith("..", StringComparison.Ordinal);
        }
        catch
        {
            return false;
        }
    }

    internal static void RunSync(string args)
    {
        if (!File.Exists(SyncExe))
            throw new FileNotFoundException("netodrive-sync.exe nao encontrado", SyncExe);
        var psi = new System.Diagnostics.ProcessStartInfo(SyncExe, args)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            WorkingDirectory = InstallDir,
        };
        System.Diagnostics.Process.Start(psi);
    }

    private static string PrepareJson(string json) =>
        LocalFolderPattern.Replace(json, m =>
        {
            var key = m.Groups[1].Value;
            var path = m.Groups[2].Value.Replace(@"\\", @"\");
            var escaped = path.Replace(@"\", @"\\");
            return $@"""{key}"": ""{escaped}""";
        });

    private static string ExtractLocalFolderFromRaw(string json)
    {
        var m = LocalFolderPattern.Match(json);
        if (!m.Success)
            return "";
        return m.Groups[2].Value.Replace(@"\\", @"\");
    }

    private static string ResolveLocalFolder(string configPath, string folder)
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
