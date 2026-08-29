using System.Net.Http.Headers;
using System.Runtime.InteropServices;
using System.Text;
using Vanara.PInvoke;
using static Vanara.PInvoke.CldApi;

namespace NetoDriveProvider;

/// <summary>
/// Conecta ao sync root e atende FETCH_DATA (clique/abrir) e pin (manter no dispositivo).
/// </summary>
internal sealed class ProviderHost : IDisposable
{
    private readonly AppConfig _cfg;
    private readonly HttpClient _http = new() { Timeout = TimeSpan.FromMinutes(30) };
    private CF_CONNECTION_KEY _connection;
    private GCHandle _callbackTableHandle;
    private bool _connected;

    internal ProviderHost(AppConfig cfg) => _cfg = cfg;

    internal void Connect()
    {
        var callbacks = new CF_CALLBACK_REGISTRATION[]
        {
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_FETCH_DATA,
                Callback = Marshal.GetFunctionPointerForDelegate<CF_CALLBACK>(OnFetchData),
            },
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_NOTIFY_UPDATE_PINNING,
                Callback = Marshal.GetFunctionPointerForDelegate<CF_CALLBACK>(OnPinning),
            },
            new() { Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_NONE, Callback = IntPtr.Zero },
        };

        _callbackTableHandle = GCHandle.Alloc(callbacks);

        var hr = CfConnectSyncRoot(
            _cfg.LocalFolder,
            callbacks,
            IntPtr.Zero,
            CF_CONNECT_FLAGS.CF_CONNECT_FLAG_REQUIRE_FULL_FILE_PATH,
            out _connection);
        if (hr.Failed)
            throw new InvalidOperationException($"CfConnectSyncRoot: {hr}");
        _connected = true;
    }

    private void OnFetchData(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            var path = info.VolumeDosName + info.NormalizedPath;
            var rel = Path.GetRelativePath(_cfg.LocalFolder, path).Replace('\\', '/');
            FetchAndTransfer(info, parameters, rel);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"FETCH_DATA erro: {ex.Message}");
        }
    }

    private void OnPinning(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            var path = info.VolumeDosName + info.NormalizedPath;
            if (!path.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
                return;
            var rel = Path.GetRelativePath(_cfg.LocalFolder, path).Replace('\\', '/');
            var pin = parameters.UpdatePinning.PinState;
            var cfgPath = Paths.ConfigPath;
            var syncExe = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "NetoDrive",
                "netodrive-sync.exe");
            if (!File.Exists(syncExe)) return;

            var arg = pin == CF_PIN_STATE.CF_PIN_STATE_PINNED
                ? $"-pin \"{rel}\" -config \"{cfgPath}\""
                : $"-unpin \"{rel}\" -config \"{cfgPath}\"";
            System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(syncExe, arg)
            {
                CreateNoWindow = true,
                UseShellExecute = false,
            });
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"PIN erro: {ex.Message}");
        }
    }

    private void FetchAndTransfer(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters, string rel)
    {
        var range = parameters.FetchData.RequiredFileOffset;
        var length = parameters.FetchData.RequiredLength;
        var offset = range;
        var need = length;

        using var req = new HttpRequestMessage(HttpMethod.Get,
            $"{_cfg.ServerURL.TrimEnd('/')}/api/sync/download/{Uri.EscapeDataString(rel)}");
        req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _cfg.Token);
        if (offset > 0 || need > 0)
            req.Headers.Range = new RangeHeaderValue(offset, offset + need - 1);

        using var res = _http.Send(req);
        res.EnsureSuccessStatusCode();
        using var stream = res.Content.ReadAsStream();

        var buffer = new byte[Math.Min(need, 1024 * 1024)];
        long done = 0;
        while (done < need)
        {
            var toRead = (int)Math.Min(buffer.Length, need - done);
            var read = stream.Read(buffer, 0, toRead);
            if (read <= 0) break;

            var opParams = new CF_OPERATION_PARAMETERS
            {
                ParamSize = (uint)Marshal.SizeOf<CF_OPERATION_PARAMETERS>(),
                Type = CF_OPERATION_TYPE.CF_OPERATION_TYPE_TRANSFER_DATA,
                TransferData = new CF_OPERATION_PARAMETERS._TransferData
                {
                    CompletionStatus = HRESULT.S_OK,
                    Buffer = buffer.AsSpan(0, read).ToArray(),
                    Offset = offset + done,
                    Length = (uint)read,
                },
            };

            var opInfo = new CF_OPERATION_INFO
            {
                StructSize = (uint)Marshal.SizeOf<CF_OPERATION_INFO>(),
                Type = CF_OPERATION_TYPE.CF_OPERATION_TYPE_TRANSFER_DATA,
                ConnectionKey = info.ConnectionKey,
                TransferKey = info.TransferKey,
                RequestKey = info.RequestKey,
            };

            CfExecute(opInfo, ref opParams);
            done += read;
        }
    }

    public void Dispose()
    {
        if (_connected)
        {
            CfDisconnectSyncRoot(_connection);
            _connected = false;
        }
        if (_callbackTableHandle.IsAllocated)
            _callbackTableHandle.Free();
        _http.Dispose();
    }
}
