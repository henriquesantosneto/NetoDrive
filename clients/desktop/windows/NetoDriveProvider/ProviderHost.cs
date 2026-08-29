using System.Net.Http.Headers;
using System.Runtime.InteropServices;
using System.Text.Json;
using Vanara.InteropServices;
using Vanara.PInvoke;
using static Vanara.PInvoke.CldApi;
using static Vanara.PInvoke.Kernel32;

namespace NetoDriveProvider;

/// <summary>
/// Conecta ao sync root e atende FETCH_PLACEHOLDERS + FETCH_DATA (Explorer).
/// </summary>
internal sealed class ProviderHost : IDisposable
{
    private const int QueueBatchSize = 8;

    private static ProviderHost? _active;
    private static readonly CF_CALLBACK FetchPlaceholdersCb = OnFetchPlaceholders;
    private static readonly CF_CALLBACK FetchDataCb = OnFetchData;
    private static readonly CF_CALLBACK NotifyDeleteCb = OnNotifyDelete;
    private static readonly CF_CALLBACK NotifyFileCloseCb = OnNotifyFileClose;
    private static readonly CF_CALLBACK NotifyDehydrateCb = OnNotifyDehydrate;
    private static readonly CF_CALLBACK NotifyRenameCb = OnNotifyRename;

    private readonly AppConfig _cfg;
    private readonly HttpClient _http = new() { Timeout = TimeSpan.FromMinutes(30) };
    private readonly object _cfapiGate = new();
    private CF_CONNECTION_KEY _connection;
    private GCHandle _callbackTableHandle;
    private bool _connected;
    private Timer? _queueTimer;

    internal ProviderHost(AppConfig cfg) => _cfg = cfg;

    internal void Connect()
    {
        _active = this;

        var callbacks = new CF_CALLBACK_REGISTRATION[]
        {
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_FETCH_PLACEHOLDERS,
                Callback = FetchPlaceholdersCb,
            },
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_FETCH_DATA,
                Callback = FetchDataCb,
            },
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_NOTIFY_DELETE,
                Callback = NotifyDeleteCb,
            },
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_NOTIFY_FILE_CLOSE_COMPLETION,
                Callback = NotifyFileCloseCb,
            },
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_NOTIFY_DEHYDRATE,
                Callback = NotifyDehydrateCb,
            },
            new()
            {
                Type = CF_CALLBACK_TYPE.CF_CALLBACK_TYPE_NOTIFY_RENAME,
                Callback = NotifyRenameCb,
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
        {
            throw new InvalidOperationException(
                $"CfConnectSyncRoot({_cfg.LocalFolder}): {CfSyncRoot.HrMessage(hr)}\n" +
                "Confira se local_folder no JSON e o mesmo registrado no Explorer (-status).\n" +
                "Rode: netodrive-provider.exe -register -config \"%APPDATA%\\NetoDrive\\netodrive.json\"");
        }
        _connected = true;
        ProcessQueueSafe();
        _queueTimer = new Timer(_ => ProcessQueueSafe(), null, TimeSpan.FromMilliseconds(500), TimeSpan.FromMilliseconds(500));
    }

    private void ProcessQueueSafe()
    {
        lock (_cfapiGate)
        {
            try
            {
                ProviderCommandQueue.ProcessPending(_cfg, QueueBatchSize);
                PlaceholderQueue.ProcessPending(_cfg, QueueBatchSize);
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"provider queue: {ex.Message}");
            }
        }
    }

    private static void OnFetchPlaceholders(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            _active?.HandleFetchPlaceholders(info);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"FETCH_PLACEHOLDERS erro: {ex.Message}");
        }
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

    private static void OnNotifyDelete(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            _active?.HandleNotifyDelete(info);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"NOTIFY_DELETE erro: {ex.Message}");
        }
    }

    private static void OnNotifyFileClose(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            _active?.HandleNotifyFileClose(info, parameters);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"NOTIFY_FILE_CLOSE_COMPLETION erro: {ex.Message}");
        }
    }

    private static void OnNotifyDehydrate(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            _active?.HandleNotifyDehydrate(info);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"NOTIFY_DEHYDRATE erro: {ex.Message}");
        }
    }

    private static void OnNotifyRename(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        try
        {
            _active?.HandleNotifyRename(info, parameters);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"NOTIFY_RENAME erro: {ex.Message}");
        }
    }

    private void HandleNotifyFileClose(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        if (parameters.CloseCompletion.Flags.HasFlag(
                CF_CALLBACK_CLOSE_COMPLETION_FLAGS.CF_CALLBACK_CLOSE_COMPLETION_FLAG_DELETED))
        {
            return;
        }

        var path = info.VolumeDosName + info.NormalizedPath;
        if (!path.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
            return;
        var rel = Path.GetRelativePath(_cfg.LocalFolder, path).Replace('\\', '/').Trim('/');
        if (!string.IsNullOrEmpty(rel) && rel != ".")
            LocalChangesQueue.EnqueueModify(_cfg, rel);
    }

    private void HandleNotifyDehydrate(in CF_CALLBACK_INFO info)
    {
        var path = info.VolumeDosName + info.NormalizedPath;
        if (path.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
        {
            var rel = Path.GetRelativePath(_cfg.LocalFolder, path).Replace('\\', '/').Trim('/');
            if (!string.IsNullOrEmpty(rel) && rel != ".")
            {
                PlaceholderCatalog.SetCloudOnly(_cfg, rel, true);
                LocalChangesQueue.EnqueuePinOp(_cfg, "unpin", rel);
            }
        }
        AckDehydrate(in info, (NTStatus)0);
    }

    private void HandleNotifyRename(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        var sourcePath = info.VolumeDosName + info.NormalizedPath;
        var targetPath = parameters.Rename.TargetPath ?? "";
        if (string.IsNullOrEmpty(targetPath))
        {
            AckRename(in info, (NTStatus)0);
            return;
        }

        if (!sourcePath.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
        {
            AckRename(in info, (NTStatus)0);
            return;
        }

        var fromRel = Path.GetRelativePath(_cfg.LocalFolder, sourcePath).Replace('\\', '/').Trim('/');
        var toRel = ResolveRenameTarget(fromRel, targetPath, _cfg.LocalFolder);

        if (!string.IsNullOrEmpty(fromRel) && fromRel != "." &&
            !string.IsNullOrEmpty(toRel) && toRel != "." && !string.Equals(fromRel, toRel, StringComparison.OrdinalIgnoreCase))
        {
            PlaceholderCatalog.MoveMeta(_cfg, fromRel, toRel);
            LocalChangesQueue.EnqueueRename(_cfg, fromRel, toRel);
            TryRenameRemote(fromRel, toRel);
        }

        AckRename(in info, (NTStatus)0);
    }

    private void TryRenameRemote(string fromRel, string toRel)
    {
        try
        {
            var body = JsonSerializer.Serialize(new { from = fromRel, to = toRel });
            using var req = new HttpRequestMessage(HttpMethod.Post,
                $"{_cfg.ServerURL.TrimEnd('/')}/api/sync/rename")
            {
                Content = new StringContent(body, System.Text.Encoding.UTF8, "application/json"),
            };
            req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _cfg.Token);
            using var res = _http.Send(req);
            res.EnsureSuccessStatusCode();
            Console.WriteLine($"rename remoto ok: {fromRel} -> {toRel}");
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"rename remoto {fromRel}->{toRel}: {ex.Message}");
        }
    }

    private static string ResolveRenameTarget(string fromRel, string targetPath, string localFolder)
    {
        targetPath = targetPath.Trim();
        if (targetPath.StartsWith(localFolder, StringComparison.OrdinalIgnoreCase))
        {
            return Path.GetRelativePath(localFolder, targetPath).Replace('\\', '/').Trim('/');
        }

        var fileName = Path.GetFileName(targetPath.Replace('/', Path.DirectorySeparatorChar));
        if (string.IsNullOrEmpty(fileName))
            return "";

        fromRel = fromRel.Replace('\\', '/').Trim('/');
        var slash = fromRel.LastIndexOf('/');
        if (slash < 0)
            return fileName;
        return fromRel[..slash] + "/" + fileName;
    }

    private void HandleNotifyDelete(in CF_CALLBACK_INFO info)
    {
        var path = info.VolumeDosName + info.NormalizedPath;
        if (!path.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
        {
            AckDelete(in info, (NTStatus)0);
            return;
        }
        var rel = Path.GetRelativePath(_cfg.LocalFolder, path).Replace('\\', '/').Trim('/');
        if (!string.IsNullOrEmpty(rel) && rel != ".")
            LocalChangesQueue.EnqueueDelete(_cfg, rel);
        AckDelete(in info, (NTStatus)0);
    }

    private void HandleFetchPlaceholders(in CF_CALLBACK_INFO info)
    {
        lock (_cfapiGate)
        {
            var path = info.VolumeDosName + info.NormalizedPath;
            var relDir = ResolveRelDir(path);
            var children = PlaceholderCatalog.DirectChildren(_cfg, relDir);
            var pending = new List<PlaceholderQueueEntry>();
            foreach (var child in children)
            {
                if (!PlaceholderManager.Exists(_cfg, child.Rel))
                    pending.Add(child);
            }
            TransferPlaceholders(info, relDir, pending);
        }
    }

    private string ResolveRelDir(string fullPath)
    {
        if (!fullPath.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
            return "";
        var rel = Path.GetRelativePath(_cfg.LocalFolder, fullPath).Replace('\\', '/').Trim('/');
        return rel == "." ? "" : rel;
    }

    private static void TransferPlaceholders(
        in CF_CALLBACK_INFO info,
        string relDir,
        IReadOnlyList<PlaceholderQueueEntry> entries)
    {
        var placeholders = new List<CF_PLACEHOLDER_CREATE_INFO>();
        var handles = new List<IDisposable>();

        try
        {
            foreach (var entry in entries)
            {
                var fileName = Path.GetFileName(entry.Rel.Replace('/', Path.DirectorySeparatorChar));
                var identity = PlaceholderIdentity.Encode(entry.Rel, entry.Hash, entry.Size);
                var idMem = new SafeCoTaskMemHandle(identity);
                handles.Add(idMem);

                placeholders.Add(new CF_PLACEHOLDER_CREATE_INFO
                {
                    RelativeFileName = fileName,
                    FsMetadata = new CF_FS_METADATA
                    {
                        FileSize = entry.Size,
                        BasicInfo = new FILE_BASIC_INFO
                        {
                            FileAttributes = FileFlagsAndAttributes.FILE_ATTRIBUTE_ARCHIVE,
                        },
                    },
                    FileIdentity = idMem.DangerousGetHandle(),
                    FileIdentityLength = (uint)identity.Length,
                    Flags = CF_PLACEHOLDER_CREATE_FLAGS.CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC,
                });
            }

            IntPtr raw = IntPtr.Zero;
            var count = placeholders.Count;
            if (count > 0)
            {
                var elemSize = Marshal.SizeOf<CF_PLACEHOLDER_CREATE_INFO>();
                raw = Marshal.AllocHGlobal(elemSize * count);
                for (var i = 0; i < count; i++)
                    Marshal.StructureToPtr(placeholders[i], raw + (i * elemSize), false);
            }

            var transfer = new CF_OPERATION_PARAMETERS.TRANSFERPLACEHOLDERS
            {
                CompletionStatus = (NTStatus)0,
                PlaceholderArray = raw,
                PlaceholderCount = (uint)count,
                PlaceholderTotalCount = (uint)count,
                EntriesProcessed = 0,
                Flags = CF_OPERATION_TRANSFER_PLACEHOLDERS_FLAGS.CF_OPERATION_TRANSFER_PLACEHOLDERS_FLAG_DISABLE_ON_DEMAND_POPULATION,
            };

            var opParams = CF_OPERATION_PARAMETERS.Create(transfer);
            var opInfo = new CF_OPERATION_INFO
            {
                StructSize = (uint)Marshal.SizeOf<CF_OPERATION_INFO>(),
                Type = CF_OPERATION_TYPE.CF_OPERATION_TYPE_TRANSFER_PLACEHOLDERS,
                ConnectionKey = info.ConnectionKey,
                TransferKey = info.TransferKey,
                RequestKey = info.RequestKey,
                CorrelationVector = info.CorrelationVector,
            };

            CfExecute(opInfo, ref opParams).ThrowIfFailed();

            if (raw != IntPtr.Zero)
            {
                var elemSize = Marshal.SizeOf<CF_PLACEHOLDER_CREATE_INFO>();
                for (var i = 0; i < count; i++)
                    Marshal.DestroyStructure<CF_PLACEHOLDER_CREATE_INFO>(raw + (i * elemSize));
                Marshal.FreeHGlobal(raw);
            }

            if (count > 0)
                Console.WriteLine($"FETCH_PLACEHOLDERS {relDir}: {count} placeholder(s)");
        }
        finally
        {
            foreach (var h in handles)
                h.Dispose();
        }
    }

    private static void AckDehydrate(in CF_CALLBACK_INFO info, NTStatus status)
    {
        var ack = new CF_OPERATION_PARAMETERS.ACKDEHYDRATE
        {
            Flags = CF_OPERATION_ACK_DEHYDRATE_FLAGS.CF_OPERATION_ACK_DEHYDRATE_FLAG_NONE,
            CompletionStatus = status,
        };
        var opParams = CF_OPERATION_PARAMETERS.Create(ack);
        var opInfo = new CF_OPERATION_INFO
        {
            StructSize = (uint)Marshal.SizeOf<CF_OPERATION_INFO>(),
            Type = CF_OPERATION_TYPE.CF_OPERATION_TYPE_ACK_DEHYDRATE,
            ConnectionKey = info.ConnectionKey,
            TransferKey = info.TransferKey,
            RequestKey = info.RequestKey,
            CorrelationVector = info.CorrelationVector,
        };
        CfExecute(opInfo, ref opParams).ThrowIfFailed();
    }

    private static void AckDelete(in CF_CALLBACK_INFO info, NTStatus status)
    {
        var ack = new CF_OPERATION_PARAMETERS.ACKDELETE
        {
            Flags = CF_OPERATION_ACK_DELETE_FLAGS.CF_OPERATION_ACK_DELETE_FLAG_NONE,
            CompletionStatus = status,
        };
        var opParams = CF_OPERATION_PARAMETERS.Create(ack);
        var opInfo = new CF_OPERATION_INFO
        {
            StructSize = (uint)Marshal.SizeOf<CF_OPERATION_INFO>(),
            Type = CF_OPERATION_TYPE.CF_OPERATION_TYPE_ACK_DELETE,
            ConnectionKey = info.ConnectionKey,
            TransferKey = info.TransferKey,
            RequestKey = info.RequestKey,
            CorrelationVector = info.CorrelationVector,
        };
        CfExecute(opInfo, ref opParams).ThrowIfFailed();
    }

    private static void AckRename(in CF_CALLBACK_INFO info, NTStatus status)
    {
        var ack = new CF_OPERATION_PARAMETERS.ACKRENAME
        {
            Flags = CF_OPERATION_ACK_RENAME_FLAGS.CF_OPERATION_ACK_RENAME_FLAG_NONE,
            CompletionStatus = status,
        };
        var opParams = CF_OPERATION_PARAMETERS.Create(ack);
        var opInfo = new CF_OPERATION_INFO
        {
            StructSize = (uint)Marshal.SizeOf<CF_OPERATION_INFO>(),
            Type = CF_OPERATION_TYPE.CF_OPERATION_TYPE_ACK_RENAME,
            ConnectionKey = info.ConnectionKey,
            TransferKey = info.TransferKey,
            RequestKey = info.RequestKey,
            CorrelationVector = info.CorrelationVector,
        };
        CfExecute(opInfo, ref opParams).ThrowIfFailed();
    }

    private void HandleFetchData(in CF_CALLBACK_INFO info, in CF_CALLBACK_PARAMETERS parameters)
    {
        var fetch = parameters.FetchData;
        var path = info.VolumeDosName + info.NormalizedPath;
        if (!path.StartsWith(_cfg.LocalFolder, StringComparison.OrdinalIgnoreCase))
            return;

        var pathRel = Path.GetRelativePath(_cfg.LocalFolder, path).Replace('\\', '/').Trim('/');
        if (string.IsNullOrEmpty(pathRel) || pathRel == ".")
            return;

        var offset = fetch.RequiredFileOffset;
        var need = fetch.RequiredLength;
        if (need < 0)
            need = 0;

        try
        {
            using var res = OpenDownloadResponse(info, pathRel, offset, need);
            using var stream = res.Content.ReadAsStream();

            var buffer = new byte[Math.Min(Math.Max(need, 1), 1024 * 1024)];
            long done = 0;
            var total = need > 0 ? need : (res.Content.Headers.ContentLength ?? 0);
            if (total <= 0 && need == 0)
                total = 1;

            while (need == 0 || done < need)
            {
                var toRead = need > 0
                    ? (int)Math.Min(buffer.Length, need - done)
                    : buffer.Length;
                var read = stream.Read(buffer, 0, toRead);
                if (read <= 0)
                    break;

                var chunk = buffer.AsSpan(0, read).ToArray();
                using var bufMem = new SafeCoTaskMemHandle(chunk);

                var transfer = new CF_OPERATION_PARAMETERS.TRANSFERDATA
                {
                    CompletionStatus = (NTStatus)0,
                    Buffer = bufMem.DangerousGetHandle(),
                    Offset = offset + done,
                    Length = read,
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
                if (need == 0)
                    break;
            }

            if (done > 0 || need == 0)
                PlaceholderCatalog.SetCloudOnly(_cfg, pathRel, false);
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"FETCH_DATA {pathRel}: {ex.Message}");
            TransferFetchDataFailure(info, offset, need, ex);
        }
    }

    private HttpResponseMessage OpenDownloadResponse(in CF_CALLBACK_INFO info, string pathRel, long offset, long need)
    {
        var candidates = DownloadRelCandidates(info, pathRel);
        Exception? last = null;
        foreach (var rel in candidates)
        {
            try
            {
                using var req = new HttpRequestMessage(HttpMethod.Get,
                    $"{_cfg.ServerURL.TrimEnd('/')}/api/sync/download/{RemotePathUtil.EscapeRemotePath(rel)}");
                req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _cfg.Token);
                if (need > 0)
                    req.Headers.Range = new RangeHeaderValue(offset, offset + need - 1);

                var res = _http.Send(req);
                if (res.IsSuccessStatusCode)
                    return res;
                last = new HttpRequestException($"HTTP {(int)res.StatusCode} for {rel}");
                res.Dispose();
            }
            catch (Exception ex)
            {
                last = ex;
            }
        }
        throw last ?? new InvalidOperationException($"download failed for {pathRel}");
    }

    private static IEnumerable<string> DownloadRelCandidates(in CF_CALLBACK_INFO info, string pathRel)
    {
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        if (seen.Add(pathRel))
            yield return pathRel;

        if (info.FileIdentity != IntPtr.Zero && info.FileIdentityLength > 0)
        {
            var bytes = new byte[info.FileIdentityLength];
            Marshal.Copy(info.FileIdentity, bytes, 0, bytes.Length);
            if (PlaceholderIdentity.TryDecode(bytes, out var idRel, out _, out _) &&
                !string.IsNullOrEmpty(idRel) && seen.Add(idRel))
            {
                yield return idRel;
            }
        }
    }

    private static void TransferFetchDataFailure(in CF_CALLBACK_INFO info, long offset, long need, Exception ex)
    {
        var transfer = new CF_OPERATION_PARAMETERS.TRANSFERDATA
        {
            CompletionStatus = (NTStatus)0xC0000001, // STATUS_UNSUCCESSFUL
            Buffer = IntPtr.Zero,
            Offset = offset,
            Length = 0,
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
        try
        {
            CfExecute(opInfo, ref opParams);
        }
        catch
        {
            Console.Error.WriteLine($"FETCH_DATA ack failed: {ex.Message}");
        }
    }

    public void Dispose()
    {
        _queueTimer?.Dispose();
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
