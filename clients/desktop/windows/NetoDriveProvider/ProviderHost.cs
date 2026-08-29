using System.Net.Http.Headers;
using System.Runtime.InteropServices;
using Vanara.PInvoke;
using static Vanara.PInvoke.CldApi;

namespace NetoDriveProvider;

/// <summary>
/// Conecta ao sync root e atende FETCH_DATA (clique/abrir no Explorer).
/// Pin/despejo nativo vem do menu do Windows quando AllowPinning=true; o menu SharpShell e netodrive-sync -pin/-unpin tambem funcionam.
/// </summary>
internal sealed class ProviderHost : IDisposable
{
    private static ProviderHost? _active;
    private static readonly CF_CALLBACK FetchDataCb = OnFetchData;

    private readonly AppConfig _cfg;
    private readonly HttpClient _http = new() { Timeout = TimeSpan.FromMinutes(30) };
    private CF_CONNECTION_KEY _connection;
    private GCHandle _callbackTableHandle;
    private bool _connected;

    internal ProviderHost(AppConfig cfg) => _cfg = cfg;

    internal void Connect()
    {
        _active = this;

        var callbacks = new CF_CALLBACK_REGISTRATION[]
        {
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_FETCH_DATA,
                Callback = FetchDataCb,
            },
            CF_CALLBACK_REGISTRATION.CF_CALLBACK_REGISTRATION_END,
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

    private static void OnFetchData(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            _active?.HandleFetchData(info, parameters);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"FETCH_DATA erro: {ex.Message}");
        }
    }

    private void HandleFetchData(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        var fetch = parameters.FetchData;
        var path = info.VolumeDosName + info.NormalizedPath;
        if (!path.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
            return;

        var rel = Path.GetRelativePath(_cfg.LocalFolder, path).Replace('\\', '/');
        var offset = fetch.RequiredFileOffset;
        var need = fetch.RequiredLength;

        using var req = new HttpRequestMessage(HttpMethod.Get,
            $"{_cfg.ServerURL.TrimEnd('/')}/api/sync/download/{Uri.EscapeDataString(rel)}");
        req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _cfg.Token);
        if (need > 0)
            req.Headers.Range = new RangeHeaderValue(offset, offset + need - 1);

        using var res = _http.Send(req);
        res.EnsureSuccessStatusCode();
        using var stream = res.Content.ReadAsStream();

        var buffer = new byte[Math.Min(Math.Max(need, 1), 1024 * 1024)];
        long done = 0;
        while (done < need)
        {
            var toRead = (int)Math.Min(buffer.Length, need - done);
            var read = stream.Read(buffer, 0, toRead);
            if (read <= 0)
                break;

            var transfer = new CF_OPERATION_PARAMETERS.TRANSFERDATA
            {
                CompletionStatus = HRESULT.S_OK,
                Buffer = buffer.AsSpan(0, read).ToArray(),
                Offset = offset + done,
                Length = (uint)read,
            };

            var opParams = CF_OPERATION_PARAMETERS.Create(transfer);
            var opInfo = new CF_OPERATION_INFO
            {
                StructSize = (uint)Marshal.SizeOf<CF_OPERATION_INFO>(),
                Type = CF_OPERATION_TYPE.CF_OPERATION_TYPE_TRANSFER_DATA,
                ConnectionKey = info.ConnectionKey,
                TransferKey = info.TransferKey,
                RequestKey = info.RequestKey,
            };

            CfExecute(opInfo, ref opParams).ThrowIfFailed();
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
        if (ReferenceEquals(_active, this))
            _active = null;
    }
}
