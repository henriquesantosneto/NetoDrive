using System.ComponentModel;
using System.Runtime.InteropServices;
using Vanara.PInvoke;
using static Vanara.PInvoke.CldApi;
using static Vanara.PInvoke.Kernel32;
using Win32FileAccess = Vanara.PInvoke.Kernel32.FileAccess;

namespace NetoDriveProvider;

/// <summary>
/// CFAPI pin/hydrate/dehydrate using file handles (Vanara requires HFILE, not paths).
/// </summary>
internal static class CfPlaceholderOps
{
    private const long Eof = -1;

    internal static void SetPinState(string fullPath, CF_PIN_STATE state)
    {
        using var h = OpenReadHandle(fullPath);
        CfSetPinState(h, state, CF_SET_PIN_FLAGS.CF_SET_PIN_FLAG_NONE, null).ThrowIfFailed();
    }

    internal static void Hydrate(string fullPath)
    {
        using var h = OpenReadHandle(fullPath);
        CfSetPinState(h, CF_PIN_STATE.CF_PIN_STATE_PINNED, CF_SET_PIN_FLAGS.CF_SET_PIN_FLAG_NONE, null).ThrowIfFailed();
        CfHydratePlaceholder(h, 0, Eof, CF_HYDRATE_FLAGS.CF_HYDRATE_FLAG_NONE, null).ThrowIfFailed();
    }

    internal static void Dehydrate(string fullPath)
    {
        CfOpenFileWithOplock(fullPath, CF_OPEN_FILE_FLAGS.CF_OPEN_FILE_FLAG_EXCLUSIVE, out var protectedHandle).ThrowIfFailed();
        using (protectedHandle)
        {
            CfReferenceProtectedHandle(protectedHandle);
            try
            {
                var h = CfGetWin32HandleFromProtectedHandle(protectedHandle);
                CfSetPinState(h, CF_PIN_STATE.CF_PIN_STATE_UNPINNED, CF_SET_PIN_FLAGS.CF_SET_PIN_FLAG_NONE, null).ThrowIfFailed();
                CfDehydratePlaceholder(h, 0, Eof, CF_HYDRATE_FLAGS.CF_HYDRATE_FLAG_NONE, null).ThrowIfFailed();
            }
            finally
            {
                CfReleaseProtectedHandle(protectedHandle);
            }
        }
    }

    private static SafeHFILE OpenReadHandle(string fullPath)
    {
        var h = CreateFile(
            fullPath,
            (uint)(Win32FileAccess.FILE_READ_DATA | Win32FileAccess.FILE_READ_ATTRIBUTES),
            FileShare.ReadWrite,
            null,
            FileMode.Open,
            FileFlagsAndAttributes.FILE_ATTRIBUTE_NORMAL,
            HFILE.NULL);
        if (h.IsInvalid)
            throw new Win32Exception(Marshal.GetLastWin32Error(), $"CreateFile({fullPath})");
        return h;
    }
}
