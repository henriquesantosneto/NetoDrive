using System.Text;
using System.Text.Json;
using Vanara.InteropServices;
using Vanara.PInvoke;
using static Vanara.PInvoke.CldApi;
using static Vanara.PInvoke.Kernel32;

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
        var full = Path.Combine(cfg.LocalFolder, rel.Replace('/', Path.DirectorySeparatorChar));
        var parent = Path.GetDirectoryName(full)!;
        Directory.CreateDirectory(parent);

        // Remove legacy placeholder (.lnk, magic file, or plain file).
        var lnk = full + ".lnk";
        if (File.Exists(lnk))
            File.Delete(lnk);
        if (File.Exists(full))
            File.Delete(full);

        var identity = PlaceholderIdentity.Encode(rel, hash, size);
        using var idMem = new SafeCoTaskMemHandle(identity);

        var info = new CF_PLACEHOLDER_CREATE_INFO
        {
            RelativeFileName = Path.GetFileName(full),
            FsMetadata = new CF_FS_METADATA
            {
                FileSize = size,
                BasicInfo = new FILE_BASIC_INFO
                {
                    FileAttributes = FileFlagsAndAttributes.FILE_ATTRIBUTE_ARCHIVE,
                },
            },
            FileIdentity = idMem.DangerousGetHandle(),
            FileIdentityLength = (uint)identity.Length,
            Flags = CF_PLACEHOLDER_CREATE_FLAGS.CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC,
        };

        uint done = 0;
        var hr = CfCreatePlaceholders(parent, new[] { info }, 1, CF_CREATE_FLAGS.CF_CREATE_FLAG_NONE, out done);
        if (hr.Failed)
            throw new InvalidOperationException($"CfCreatePlaceholders {rel}: {hr}");
        if (info.Result.Failed)
            throw new InvalidOperationException($"CfCreatePlaceholders {rel}: {info.Result}");
    }
}
