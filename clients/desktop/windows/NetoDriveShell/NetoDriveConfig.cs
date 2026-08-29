using System;
using System.IO;
using System.Linq;
using System.Text.Json;
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

    internal static string SyncExe => Path.Combine(InstallDir, "netodrive-sync.exe");

    internal static bool TryGetLocalFolder(out string localFolder)
    {
        localFolder = "";
        if (!File.Exists(ConfigPath))
            return false;
        try
        {
            using var doc = JsonDocument.Parse(File.ReadAllText(ConfigPath));
            if (doc.RootElement.TryGetProperty("local_folder", out var lf))
            {
                localFolder = lf.GetString() ?? "";
                if (!string.IsNullOrWhiteSpace(localFolder))
                {
                    localFolder = Path.GetFullPath(localFolder.Trim());
                    return true;
                }
            }
        }
        catch
        {
            // ignore malformed config
        }
        return false;
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
            relative = Path.GetRelativePath(root, full).Replace('\\', '/').Trim('/');
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
}
