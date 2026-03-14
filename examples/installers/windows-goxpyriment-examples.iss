[Setup]
AppId={{9A02C8E1-8D6C-4F2E-B7E3-EXAMPLES-GOXPYRIMENT}}
AppName=Goxpyriment Examples
AppVersion=0.1.0
DefaultDirName={pf}\Goxpyriment Examples
DefaultGroupName=Goxpyriment Examples
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
; 1. Recurse through the examples directory (..) to find all .exe files.
; We use '..\*.exe' with 'recursesubdirs' to find all binaries in subfolders.
; We exclude the 'installers' folder itself to prevent the setup from packaging itself.
Source: "..\*.exe"; DestDir: "{app}\bin"; Flags: recursesubdirs ignoreversion; Excludes: "installers\*"

; 2. Install the assets folder from the root directory
Source: "..\..\assets\*"; DestDir: "{app}\assets"; Flags: ignoreversion recursesubdirs

[Icons]
; Create a shortcut to the folder containing the binaries
Name: "{group}\Browse Example Binaries"; Filename: "{app}\bin"
; Shortcut for uninstallation
Name: "{group}\Uninstall Goxpyriment Examples"; Filename: "{uninstallexe}"
