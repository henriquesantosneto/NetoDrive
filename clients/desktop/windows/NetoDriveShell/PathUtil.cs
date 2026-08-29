using System;
using System.IO;

namespace NetoDriveShell;

internal static class PathUtil
{
    internal static string GetRelativePath(string root, string full)
    {
        root = Path.GetFullPath(root);
        full = Path.GetFullPath(full);
        if (!full.StartsWith(root, StringComparison.OrdinalIgnoreCase))
            return full;

        var rootUri = new Uri(root.TrimEnd(Path.DirectorySeparatorChar) + Path.DirectorySeparatorChar);
        var fullUri = new Uri(full);
        var rel = Uri.UnescapeDataString(rootUri.MakeRelativeUri(fullUri).ToString());
        return rel.Replace('/', Path.DirectorySeparatorChar);
    }
}
