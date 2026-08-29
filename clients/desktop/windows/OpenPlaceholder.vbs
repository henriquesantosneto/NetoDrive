' NetoDrive - abre placeholder: baixa do servidor e abre com o app padrao.
If WScript.Arguments.Count < 1 Then WScript.Quit 1
Set sh = CreateObject("WScript.Shell")
install = sh.ExpandEnvironmentStrings("%LOCALAPPDATA%\NetoDrive")
cfg = sh.ExpandEnvironmentStrings("%APPDATA%\NetoDrive\netodrive.json")
rel = WScript.Arguments(0)
cmd = """" & install & "\netodrive-sync.exe"" -open """ & rel & """ -config """ & cfg & """"
sh.Run cmd, 0, False
