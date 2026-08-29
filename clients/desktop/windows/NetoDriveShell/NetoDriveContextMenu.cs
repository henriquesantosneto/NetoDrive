using System;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Windows.Forms;
using SharpShell.Attributes;
using SharpShell.SharpContextMenu;

namespace NetoDriveShell;

/// <summary>
/// Menu de contexto do Explorer: Manter neste dispositivo / Baixar / Liberar espaco.
/// </summary>
[ComVisible(true)]
[Guid("6E4F8A2B-9C1D-4E5F-A3B7-2D8E91C04F6A")]
[ClassInterface(ClassInterfaceType.None)]
[COMServerAssociation(AssociationType.AllFiles)]
public class NetoDriveContextMenu : SharpContextMenu
{
    protected override bool CanShowMenu()
    {
        if (SelectedItemPaths is null || SelectedItemPaths.Count() != 1)
            return false;
        return NetoDriveConfig.IsUnderSyncRoot(SelectedItemPaths.First(), out _);
    }

    protected override ContextMenuStrip CreateMenu()
    {
        var menu = new ContextMenuStrip();
        var path = SelectedItemPaths!.First();
        if (!NetoDriveConfig.IsUnderSyncRoot(path, out var rel))
            return menu;

        var cfg = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
            "NetoDrive",
            "netodrive.json");

        menu.Items.Add("NetoDrive: Baixar agora", null, (_, _) =>
            SafeRun($"-hydrate \"{rel}\" -config \"{cfg}\""));

        menu.Items.Add("NetoDrive: Manter neste dispositivo", null, (_, _) =>
            SafeRun($"-pin \"{rel}\" -config \"{cfg}\""));

        menu.Items.Add("NetoDrive: Liberar espaco (nuvem)", null, (_, _) =>
            SafeRun($"-unpin \"{rel}\" -config \"{cfg}\""));

        return menu;
    }

    private static void SafeRun(string args)
    {
        try
        {
            NetoDriveConfig.RunSync(args);
        }
        catch (Exception ex)
        {
            MessageBox.Show(ex.Message, "NetoDrive", MessageBoxButtons.OK, MessageBoxIcon.Warning);
        }
    }
}
