namespace NetoDriveProvider;

internal static class Program
{
    private static int Main(string[] args)
    {
        try
        {
            if (args.Length == 0)
            {
                PrintUsage();
                return 1;
            }

            var cfgPath = Paths.ConfigPath;
            for (var i = 0; i < args.Length - 1; i++)
            {
                if (args[i] == "-config" && i + 1 < args.Length)
                    cfgPath = args[++i];
            }

            if (!File.Exists(cfgPath))
            {
                Console.Error.WriteLine($"Config nao encontrada: {cfgPath}");
                return 1;
            }

            var cfg = AppConfig.Load(cfgPath);

            switch (args[0])
            {
                case "-unregister":
                case "-cleanup":
                    SyncRootRegistrar.Unregister(cfg.LocalFolder);
                    Console.WriteLine("Sync root removido.");
                    return 0;

                case "-register":
                    SyncRootRegistrar.Register(cfg);
                    Console.WriteLine("Sync root registrado.");
                    return 0;

                case "-status":
                    return SyncRootStatus.Report(cfg, cfgPath);

                case "-placeholder":
                    if (args.Length < 4)
                    {
                        Console.Error.WriteLine("Uso: -placeholder <rel> <hash> <size>");
                        return 1;
                    }
                    var rel = args[1];
                    var hash = args[2];
                    if (!long.TryParse(args[3], out var size))
                        size = 0;
                    PlaceholderManager.Create(cfg, rel, hash, size);
                    return 0;

                case "-run":
                    using (var host = new ProviderHost(cfg))
                    {
                        host.Connect();
                        Console.WriteLine("NetoDrive provider ativo (CFAPI). Ctrl+C para sair.");
                        using var evt = new ManualResetEventSlim(false);
                        Console.CancelKeyPress += (_, e) => { e.Cancel = true; evt.Set(); };
                        evt.Wait();
                    }
                    return 0;

                default:
                    PrintUsage();
                    return 1;
            }
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine(ex);
            return 1;
        }
    }

    private static void PrintUsage()
    {
        Console.WriteLine("""
            netodrive-provider -register | -unregister | -run | -status | -placeholder <rel> <hash> <size>
              -config <path>   (opcional, padrao %APPDATA%\\NetoDrive\\netodrive.json)
            """);
    }
}
