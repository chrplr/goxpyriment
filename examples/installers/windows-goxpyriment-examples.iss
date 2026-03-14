[Setup]
AppId={{9A02C8E1-8D6C-4F2E-B7E3-EXAMPLES-GOXPYRIMENT}}
AppName=Goxpyriment Examples
AppVersion=0.1.0
; Use {localappdata} instead of {pf} (Program Files) to ensure write permissions [cite: 48]
DefaultDirName={localappdata}\Goxpyriment Examples
DefaultGroupName=Goxpyriment Examples
; Ensures the installer doesn't ask for Admin privileges 
PrivilegesRequired=lowest 
DisableDirPage=no
DisableProgramGroupPage=yes
OutputDir=.
OutputBaseFilename=goxpyriment-examples-setup
Compression=lzma
SolidCompression=yes
SetupIconFile=..\\..\\assets\\icon.ico

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
; Source binaries with stripping already handled in build step [cite: 50]
Source: "..\*.exe"; DestDir: "{app}\bin"; Flags: recursesubdirs ignoreversion; Excludes: "installers\*"
; Source assets [cite: 51]
Source: "..\..\assets\*"; DestDir: "{app}\assets"; Flags: ignoreversion recursesubdirs

[Icons]
; Shortcut to the bin folder so users can find the .exe files [cite: 52]
Name: "{group}\Browse Examples"; Filename: "{app}\bin"
Name: "{group}\Uninstall Goxpyriment Examples"; Filename: "{uninstallexe}"
