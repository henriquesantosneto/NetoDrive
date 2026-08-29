using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;
using Vanara.PInvoke;
using static Vanara.PInvoke.CldApi;

namespace NetoDriveProvider;

internal static class PlaceholderIdentity
{
    internal static byte[] Encode(string rel, string hash, long size)
    {
        var json = JsonSerializer.Serialize(new { rel, hash, size });
        return Encoding.UTF8.GetBytes(json);
    }

    internal static bool TryDecode(ReadOnlySpan<byte> data, out string rel, out string hash, out long size)
    {
        rel = "";
        hash = "";
        size = 0;
        try
        {
            var json = Encoding.UTF8.GetString(data);
            using var doc = JsonDocument.Parse(json);
            rel = doc.RootElement.GetProperty("rel").GetString() ?? "";
            hash = doc.RootElement.GetProperty("hash").GetString() ?? "";
            size = doc.RootElement.GetProperty("size").GetInt64();
            return rel.Length > 0;
        }
        catch
        {
            return false;
        }
    }
}

internal static class PlaceholderManager
{
    internal static void Create(AppConfig cfg, string rel, string hash, long size)
    {
        rel = rel.Replace('\\', '/').Trim('/');
        var baseDir = cfg.LocalFolder;
        var full = Path.Combine(baseDir, rel.Replace('/', Path.DirectorySeparatorChar));
        Directory.CreateDirectory(Path.GetDirectoryName(full)!);

        var identity = PlaceholderIdentity.Encode(rel, hash, size);
        var info = new CF_PLACEHOLDER_CREATE_INFO
        {
            RelativeFileName = Path.GetFileName(full),
            FsMetadata = new CF_FS_METADATA
            {
                FileSize = size,
                BasicInfo = new Vanara.PInvoke.Kernel32.FILE_BASIC_INFO
                {
                    FileAttributes = (uint)FileAttributes.Archive,
                },
            },
            FileIdentity = identity,
            FileIdentityLength = (uint)identity.Length,
            Flags = CF_PLACEHOLDER_CREATE_FLAGS.CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC,
        };

        var parent = Path.GetDirectoryName(full)!;
        var processed = 0u;
        var hr = CfCreatePlaceholders(parent, new[] { info }, 1,
            CF_CREATE_FLAGS.CF_CREATE_FLAG_NONE, ref processed);
        if (hr.Failed)
            throw new InvalidOperationException($"CfCreatePlaceholders {rel}: {hr}");

        // Remove legacy .lnk / magic placeholder if present
        var lnk = full + ".lnk";
        if (File.Exists(lnk)) File.Delete(lnk);
        if (File.Exists(full) && !IsCloudPlaceholder(full))
        {
            // magic placeholder — replace
            File.Delete(full);
            hr = CfCreatePlaceholders(parent, new[] { info }, 1,
                CF_CREATE_FLAGS.CF_CREATE_FLAG_NONE, ref processed);
            if (hr.Failed)
                throw new InvalidOperationException($"CfCreatePlaceholders retry {rel}: {hr}");
        }
    }

    internal static bool IsCloudPlaceholder(string path)
    {
        var hr = CfGetPlaceholderStateFromPath(path, out var state);
        return hr.Succeeded && state.HasFlag(CF_PLACEHOLDER_STATE.CF_PLACEHOLDER_STATE_PLACEHOLDER);
    }
}
