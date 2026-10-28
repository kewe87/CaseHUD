; CaseHUD Windows setup (Inno Setup 6).
; Build: ISCC.exe /DAppVersion=1.0.0 /DSourceDir=..\stage CaseHUD.iss
; One UAC prompt installs everything that needs admin rights once:
; app files, the sensor service (LibreHardwareMonitorLib), the PawnIO driver
; and the Performance Log Users membership for FPS capture.

#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
#ifndef SourceDir
  #define SourceDir "..\stage"
#endif

[Setup]
AppId={{6F3A9C2E-5B71-4C0D-9E8A-CA5E0D1A2B30}
AppName=CaseHUD
AppVersion={#AppVersion}
AppVerName=CaseHUD {#AppVersion}
AppPublisher=kewe87
AppPublisherURL=https://github.com/kewe87/casehud
AppSupportURL=https://github.com/kewe87/casehud/issues
AppCopyright=Copyright (C) 2026 kewe87 - GPL-3.0
DefaultDirName={autopf}\CaseHUD
DefaultGroupName=CaseHUD
DisableProgramGroupPage=yes
; No custom install folder: the SYSTEM service runs a binary from {app}, so {app} must
; stay in a place only admins can write (Program Files), and is locked down below.
DisableDirPage=yes
OutputDir=..\dist
OutputBaseFilename=CaseHUD-Setup-{#AppVersion}
SetupIconFile=casehud.ico
UninstallDisplayIcon={app}\CaseHUD.exe
LicenseFile={#SourceDir}\LICENSE
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=yes

[Languages]
Name: "de"; MessagesFile: "compiler:Languages\German.isl"; InfoBeforeFile: "disclaimer-de.txt"
Name: "en"; MessagesFile: "compiler:Default.isl"; InfoBeforeFile: "disclaimer-en.txt"

[CustomMessages]
de.Autostart=CaseHUD mit Windows starten (im Tray)
en.Autostart=Start CaseHUD with Windows (in the tray)
de.FpsCapture=FPS-Messung erlauben (PresentMon, ohne Admin-Rechte)
en.FpsCapture=Allow FPS capture (PresentMon, without admin rights)
de.InstallDriver=Installiere Hardware-Treiber (PawnIO)...
en.InstallDriver=Installing hardware driver (PawnIO)...
de.InstallService=Richte Sensor-Dienst ein...
en.InstallService=Setting up the sensor service...
de.Launch=CaseHUD starten
en.Launch=Launch CaseHUD

[Tasks]
Name: "autostart"; Description: "{cm:Autostart}"
Name: "fps"; Description: "{cm:FpsCapture}"
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Excludes: "PawnIO_setup.exe"; Flags: recursesubdirs ignoreversion
Source: "{#SourceDir}\PawnIO_setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall skipifsourcedoesntexist

[Dirs]
; Service settings (sensors.json). Nothing in CaseHUD writes here at runtime; an admin
; creates the files by hand. The ACL is tightened in [Run] (icacls), because Inno
; permissions only add ACEs and would keep the inherited "Users may create files".
Name: "{commonappdata}\CaseHUD"; Flags: uninsneveruninstall

[Icons]
Name: "{group}\CaseHUD"; Filename: "{app}\CaseHUD.exe"
Name: "{autodesktop}\CaseHUD"; Filename: "{app}\CaseHUD.exe"; Tasks: desktopicon

[Run]
; Autostart is a per-user Run key (the app toggles it in HKCU as well). The setup
; runs elevated, possibly as a different admin account via UAC, so the key is written
; as the original user instead of through [Registry].
Filename: "{sys}\reg.exe"; Parameters: "add HKCU\Software\Microsoft\Windows\CurrentVersion\Run /v CaseHUD /t REG_SZ /d ""\""{app}\CaseHUD.exe\"" --minimized"" /f"; Tasks: autostart; Flags: runhidden waituntilterminated runasoriginaluser
; Hardware driver used by LibreHardwareMonitorLib, only if missing.
; -silent: without it PawnIO shows a "successfully installed" message box that
; blocks silent installs.
Filename: "{tmp}\PawnIO_setup.exe"; Parameters: "-install -silent"; StatusMsg: "{cm:InstallDriver}"; Check: PawnIONeeded; Flags: waituntilterminated
; ProgramData\CaseHUD is read-only for normal users: no inheritance, SYSTEM and
; Administrators full control, Users read/execute (the SYSTEM service trusts files there).
; Files and folders that already exist inside (from older installs or by hand) are
; reset to inherit that ACL, which would otherwise keep a user-writable ACL or owner.
; Granting (OI)(CI) with /T would strip their inherited entries and leave files with
; an empty ACL. /C continues past entries icacls cannot change; the service still
; ignores a file whose ACL is not safe, so a miss stays fail-closed.
; /L: act on a link itself, never on what it points to (PrepareToInstall also removes
; links at the two top-level paths).
Filename: "{sys}\icacls.exe"; Parameters: """{commonappdata}\CaseHUD"" /L /inheritance:r /grant:r *S-1-5-18:(OI)(CI)F *S-1-5-32-544:(OI)(CI)F *S-1-5-32-545:(OI)(CI)RX"; Flags: runhidden waituntilterminated
Filename: "{sys}\icacls.exe"; Parameters: """{commonappdata}\CaseHUD\*"" /L /reset /T /C"; Flags: runhidden waituntilterminated
Filename: "{sys}\icacls.exe"; Parameters: """{commonappdata}\CaseHUD"" /L /setowner *S-1-5-32-544 /T /C"; Flags: runhidden waituntilterminated
; The SYSTEM service starts {app}\sensors\CaseHUD.Sensors.exe, so {app} gets the same
; ACL before the service is created or reconfigured (same steps as above).
Filename: "{sys}\icacls.exe"; Parameters: """{app}"" /L /inheritance:r /grant:r *S-1-5-18:(OI)(CI)F *S-1-5-32-544:(OI)(CI)F *S-1-5-32-545:(OI)(CI)RX"; Flags: runhidden waituntilterminated
Filename: "{sys}\icacls.exe"; Parameters: """{app}\*"" /L /reset /T /C"; Flags: runhidden waituntilterminated
Filename: "{sys}\icacls.exe"; Parameters: """{app}"" /L /setowner *S-1-5-32-544 /T /C"; Flags: runhidden waituntilterminated
; Sensor service (create fails harmlessly on upgrades, config updates it).
Filename: "{sys}\sc.exe"; Parameters: "create CaseHUDSensors binPath= ""\""{app}\sensors\CaseHUD.Sensors.exe\"""" start= auto DisplayName= ""CaseHUD Sensors"""; StatusMsg: "{cm:InstallService}"; Flags: runhidden waituntilterminated
Filename: "{sys}\sc.exe"; Parameters: "config CaseHUDSensors binPath= ""\""{app}\sensors\CaseHUD.Sensors.exe\"""" start= auto"; Flags: runhidden waituntilterminated
Filename: "{sys}\sc.exe"; Parameters: "description CaseHUDSensors ""Reads hardware sensors for CaseHUD (LibreHardwareMonitorLib)"""; Flags: runhidden waituntilterminated
Filename: "{sys}\sc.exe"; Parameters: "failure CaseHUDSensors reset= 86400 actions= restart/5000/restart/5000/restart/30000"; Flags: runhidden waituntilterminated
Filename: "{sys}\sc.exe"; Parameters: "start CaseHUDSensors"; Flags: runhidden waituntilterminated
; AWTRIX clocks answer the network search on UDP 4211 (private networks only).
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""CaseHUD AWTRIX search"""; Flags: runhidden waituntilterminated
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall add rule name=""CaseHUD AWTRIX search"" dir=in action=allow protocol=UDP localport=4211 profile=private program=""{app}\CaseHUD.exe"""; Flags: runhidden waituntilterminated
; Performance Log Users (S-1-5-32-559) lets PresentMon trace frames.
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -WindowStyle Hidden -Command ""Add-LocalGroupMember -SID 'S-1-5-32-559' -Member '{code:QuotedUser}' -ErrorAction SilentlyContinue"""; Tasks: fps; Flags: runhidden waituntilterminated
; Optional: /API=1 switches the API for other programs on (default off), as the original user.
Filename: "{app}\CaseHUD.exe"; Parameters: "--set-api=on"; Check: APIRequested; Flags: runhidden waituntilterminated runasoriginaluser skipifdoesntexist
Filename: "{app}\CaseHUD.exe"; Description: "{cm:Launch}"; Flags: nowait postinstall skipifsilent runasoriginaluser
; Self-update (CaseHUD starts this setup with /RELAUNCH=window|tray|hidden):
; start CaseHUD again the way it ran before, as the original user.
Filename: "{app}\CaseHUD.exe"; Parameters: "{code:RelaunchArgs}"; Check: RelaunchRequested; Flags: nowait runasoriginaluser skipifdoesntexist

[UninstallRun]
; Best effort: removes the Run entry of the uninstalling account. Another user's entry
; can only be removed by the app itself (Settings) and is harmless once the exe is gone.
Filename: "{sys}\reg.exe"; Parameters: "delete HKCU\Software\Microsoft\Windows\CurrentVersion\Run /v CaseHUD /f"; Flags: runhidden waituntilterminated; RunOnceId: "DeleteAutostart"
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM CaseHUD.exe"; Flags: runhidden; RunOnceId: "KillApp"
Filename: "{sys}\sc.exe"; Parameters: "stop CaseHUDSensors"; Flags: runhidden waituntilterminated; RunOnceId: "StopService"
Filename: "{sys}\sc.exe"; Parameters: "delete CaseHUDSensors"; Flags: runhidden waituntilterminated; RunOnceId: "DeleteService"
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""CaseHUD AWTRIX search"""; Flags: runhidden waituntilterminated; RunOnceId: "DeleteFirewallRule"

[Code]
// QuotedUser escapes the user name for a single-quoted PowerShell string.
function QuotedUser(Param: String): String;
begin
  Result := ExpandConstant('{username}');
  StringChangeEx(Result, '''', '''''', True);
end;

function RelaunchRequested: Boolean;
var
  r: String;
begin
  r := ExpandConstant('{param:RELAUNCH|}');
  Result := (r = 'window') or (r = 'tray') or (r = 'hidden');
end;

function RelaunchArgs(Param: String): String;
var
  r: String;
begin
  r := ExpandConstant('{param:RELAUNCH|}');
  if r = 'tray' then
    Result := '--minimized'
  else if r = 'hidden' then
    Result := '--background --no-tray'
  else
    Result := '';
end;

function APIRequested: Boolean;
begin
  Result := ExpandConstant('{param:API|0}') = '1';
end;

function PawnIONeeded: Boolean;
begin
  Result := not RegKeyExists(HKLM64, 'SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO')
        and not RegKeyExists(HKLM32, 'SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO');
end;

function GetFileAttributesW(lpFileName: String): Cardinal;
  external 'GetFileAttributesW@kernel32.dll stdcall';
function RemoveDirectoryW(lpPathName: String): Boolean;
  external 'RemoveDirectoryW@kernel32.dll stdcall';
function DeleteFileW(lpFileName: String): Boolean;
  external 'DeleteFileW@kernel32.dll stdcall';

// Removes Path if it is a junction or symbolic link (only the link, never its target)
// so that the permission changes below cannot be redirected. Returns False if a link
// is there and cannot be removed.
function ClearReparsePoint(Path: String): Boolean;
var
  attr: Cardinal;
begin
  Result := True;
  attr := GetFileAttributesW(Path);
  if (attr = $FFFFFFFF) or ((attr and $400) = 0) then
    Exit;
  if not RemoveDirectoryW(Path) then
    DeleteFileW(Path);
  attr := GetFileAttributesW(Path);
  Result := (attr = $FFFFFFFF) or ((attr and $400) = 0);
end;

// Stop the running app and service so their files can be replaced. CaseHUD
// is first asked to quit cleanly (it switches displays and lights off the way
// the user chose), as the user it runs as; it holds the mutex CaseHUD.Running
// until it has exited. Only if it is still there after 20 s, or is a version
// without --quit, it is ended hard.
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  rc, i: Integer;
begin
  if not ClearReparsePoint(ExpandConstant('{commonappdata}\CaseHUD')) or
     not ClearReparsePoint(ExpandConstant('{app}')) then
  begin
    Result := 'A link (junction or symbolic link) sits where CaseHUD must be installed and could not be removed. Remove it and run setup again.';
    Exit;
  end;
  if FileExists(ExpandConstant('{app}\CaseHUD.exe')) then
  begin
    ExecAsOriginalUser(ExpandConstant('{app}\CaseHUD.exe'), '--quit', '', SW_HIDE, ewWaitUntilTerminated, rc);
    for i := 1 to 80 do
    begin
      if not CheckForMutexes('CaseHUD.Running') then
        Break;
      Sleep(250);
    end;
  end;
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM CaseHUD.exe', '', SW_HIDE, ewWaitUntilTerminated, rc);
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop CaseHUDSensors', '', SW_HIDE, ewWaitUntilTerminated, rc);
  Sleep(1500);
  Result := '';
end;
