using Microsoft.Win32;
using Windows.Storage.Provider;

namespace NetoDriveProvider;

internal static class SyncRootStatus
{
    internal static string InstallDir => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "NetoDrive");

    internal static int Report(AppConfig cfg, string cfgPath)
    {
        var expected = Normalize(cfg.LocalFolder);
        var installDir = Normalize(InstallDir);
        var rawJson = JsonConfigReader.ReadLocalFolderRawFromFile(cfgPath);

        Console.WriteLine($"Config: {cfgPath}");
        if (!string.IsNullOrWhiteSpace(rawJson))
            Console.WriteLine($"local_folder (JSON): {rawJson}");
        else
            Console.WriteLine("local_folder (JSON): (nao definido — usa padrao)");
        Console.WriteLine($"local_folder (usado pelo provider): {cfg.LocalFolder}");
        Console.WriteLine($"Pasta de instalacao: {installDir}");
        Console.WriteLine();

        if (!string.IsNullOrWhiteSpace(rawJson))
        {
            var want = Normalize(LocalFolderResolver.Resolve(cfgPath, rawJson));
            if (!want.Equals(expected, StringComparison.OrdinalIgnoreCase))
            {
                Console.WriteLine("AVISO: valor resolvido difere do JSON; recompile o provider.");
                expected = want;
            }
        }

        var roots = ListNetoDriveRoots();
        if (roots.Count == 0)
        {
            Console.WriteLine("Explorer: nenhum sync root NetoDrive registrado.");
            Console.WriteLine("  Rode: netodrive-provider.exe -register -config <json>");
            return 1;
        }

        var ok = IsRegisteredForFolder(expected, installDir, roots);
        Console.WriteLine("Explorer (acesso rapido / barra lateral):");
        foreach (var (id, path) in roots)
        {
            var norm = Normalize(path);
            var status = "OK";
            if (norm.Equals(installDir, StringComparison.OrdinalIgnoreCase))
                status = "ERRADO (pasta do programa, nao e local_folder)";
            else if (!norm.Equals(expected, StringComparison.OrdinalIgnoreCase))
                status = "DIVERGENTE";

            Console.WriteLine($"  [{status}] {path}");
            Console.WriteLine($"           id: {id}");
        }

        DumpRegistryRoots(expected, installDir);

        if (!ok)
        {
            Console.WriteLine();
            Console.WriteLine("Corrija com:");
            Console.WriteLine("  netodrive-provider.exe -unregister -config \"%APPDATA%\\NetoDrive\\netodrive.json\"");
            Console.WriteLine("  netodrive-provider.exe -register -config \"%APPDATA%\\NetoDrive\\netodrive.json\"");
            Console.WriteLine("Se continuar errado, feche todas as janelas do Explorer e repita.");
            return 1;
        }

        Console.WriteLine();
        Console.WriteLine("Sync root confere com local_folder.");
        return 0;
    }

    /// <summary>True when Explorer sync root matches local_folder (not the install dir).</summary>
    internal static bool IsRegisteredFor(AppConfig cfg)
    {
        var expected = Normalize(cfg.LocalFolder);
        var installDir = Normalize(InstallDir);
        return IsRegisteredForFolder(expected, installDir, ListNetoDriveRoots());
    }

    private static bool IsRegisteredForFolder(string expected, string installDir, List<(string id, string path)> roots)
    {
        foreach (var (_, path) in roots)
        {
            var norm = Normalize(path);
            if (norm.Equals(installDir, StringComparison.OrdinalIgnoreCase))
                continue;
            if (norm.Equals(expected, StringComparison.OrdinalIgnoreCase))
                return true;
        }
        return false;
    }

    internal static void ConfirmRegistration(AppConfig cfg)
    {
        if (IsRegisteredFor(cfg))
        {
            Console.WriteLine($"Explorer (acesso rapido): {Normalize(cfg.LocalFolder)}");
            return;
        }
        Console.Error.WriteLine(
            "AVISO: registro concluido mas Explorer ainda nao lista a pasta esperada. " +
            "Rode netodrive-provider.exe -status para detalhes.");
    }

    private static List<(string id, string path)> ListNetoDriveRoots()
    {
        var list = new List<(string, string)>();
        try
        {
            foreach (var root in StorageProviderSyncRootManager.GetCurrentSyncRoots())
            {
                if (string.IsNullOrEmpty(root.Id))
                    continue;
                if (!root.Id.StartsWith(Paths.ProviderName + "!", StringComparison.OrdinalIgnoreCase))
                    continue;
                list.Add((root.Id, root.Path?.Path ?? ""));
            }
        }
        catch
        {
            // ignore
        }
        return list;
    }

    private static void DumpRegistryRoots(string expected, string installDir)
    {
        const string basePath = @"Software\Microsoft\Windows\CurrentVersion\Explorer\SyncRootManager";
        try
        {
            using var mgr = Registry.CurrentUser.OpenSubKey(basePath);
            if (mgr == null)
                return;

            Console.WriteLine();
            Console.WriteLine("Registro SyncRootManager:");
            var any = false;
            foreach (var name in mgr.GetSubKeyNames())
            {
                if (!name.StartsWith(Paths.ProviderName + "!", StringComparison.OrdinalIgnoreCase))
                    continue;
                any = true;
                using var sk = mgr.OpenSubKey(name);
                var userRoot = sk?.GetValue("UserSyncRootPath") as string ?? "(vazio)";
                var norm = Normalize(userRoot);
                var tag = "OK";
                if (norm.Equals(installDir, StringComparison.OrdinalIgnoreCase))
                    tag = "ERRADO";
                else if (!norm.Equals(expected, StringComparison.OrdinalIgnoreCase))
                    tag = "DIVERGENTE";
                Console.WriteLine($"  [{tag}] UserSyncRootPath={userRoot}");
            }
            if (!any)
                Console.WriteLine("  (sem chaves NetoDrive)");
        }
        catch
        {
            // ignore
        }
    }

    private static string Normalize(string path)
    {
        try
        {
            return Path.GetFullPath(path).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        }
        catch
        {
            return path.TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        }
    }
}
