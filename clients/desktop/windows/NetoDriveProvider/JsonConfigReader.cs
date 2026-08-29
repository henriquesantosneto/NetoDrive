using System.Text.RegularExpressions;

namespace NetoDriveProvider;

internal static class JsonConfigReader
{
    private static readonly Regex LocalFolderPattern = new(
        @"""((?:local_folder|LocalFolder|localFolder))""\s*:\s*""([^""]*)""",
        RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);

    /// <summary>
    /// Corrige caminhos Windows com barra simples (invalidos em JSON): C:\Users -> C:\\Users
    /// </summary>
    internal static string PrepareJson(string json) =>
        LocalFolderPattern.Replace(json, FixLocalFolderMatch);

    internal static string? ReadLocalFolderRaw(string json)
    {
        var prepared = PrepareJson(json);
        try
        {
            using var doc = System.Text.Json.JsonDocument.Parse(prepared);
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
        return ExtractLocalFolderRegex(json);
    }

    internal static string? ReadLocalFolderRawFromFile(string cfgPath)
    {
        if (!File.Exists(cfgPath))
            return null;
        return ReadLocalFolderRaw(File.ReadAllText(cfgPath));
    }

    internal static string? ExtractLocalFolderRegex(string json)
    {
        var m = LocalFolderPattern.Match(json);
        if (!m.Success)
            return null;
        return UnescapeJsonPath(m.Groups[2].Value);
    }

    private static string FixLocalFolderMatch(Match m)
    {
        var key = m.Groups[1].Value;
        var path = UnescapeJsonPath(m.Groups[2].Value);
        return $@"""{key}"": ""{EscapeJsonPath(path)}""";
    }

    internal static string EscapeJsonPath(string path) =>
        path.Replace(@"\", @"\\", StringComparison.Ordinal);

    internal static string UnescapeJsonPath(string path) =>
        path.Replace(@"\\", @"\", StringComparison.Ordinal);
}
