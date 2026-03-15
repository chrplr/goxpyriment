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
; 1. Install the compiled .exe files into the \bin folder [cite: 49]
Source: "..\*.exe"; DestDir: "{app}\bin"; Flags: recursesubdirs ignoreversion; Excludes: "installers\*"

; 2. NEW: Install the 'assets' subfolders found within each individual example [cite: 50]
; This looks for any folder named 'assets' inside the example directories and 
; places them into the \bin folder alongside the .exe files to maintain the relative paths.
Source: "..\*\assets\*"; DestDir: "{app}\bin"; Flags: recursesubdirs ignoreversion

; 3. Install the global assets folder from the root directory [cite: 51]
Source: "..\..\assets\*"; DestDir: "{app}\assets"; Flags: ignoreversion recursesubdirs

[Icons]
Name: "{group}\Browse Example Binaries"; Filename: "{app}\bin"
Name: "{group}\Uninstall Goxpyriment Examples"; Filename: "{uninstallexe}"
