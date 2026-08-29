namespace NetoDriveProvider;

internal static class RemotePathUtil
{
    /// <summary>Matches Go syncer escapePath: encode each path segment, keep slashes.</summary>
    internal static string EscapeRemotePath(string rel)
    {
        rel = rel.Replace('\\', '/').Trim('/');
        if (rel.Length == 0)
            return "";
        var parts = rel.Split('/', StringSplitOptions.RemoveEmptyEntries);
        return string.Join("/", parts.Select(Uri.EscapeDataString));
    }
}
