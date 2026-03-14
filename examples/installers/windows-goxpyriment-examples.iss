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
; 1. Install ONLY the compiled .exe files. 
; The 'skipifsourcedoesntexist' flag ensures that excluded video examples 
; (which weren't built) don't cause the installer creation to fail. [cite: 3]
Source: "..\*\*.exe"; DestDir: "{app}\bin"; Flags: recursesubdirs ignoreversion skipifsourcedoesntexist

; 2. Install the assets folder from the root directory [cite: 4]
Source: "..\..\assets\*"; DestDir: "{app}\assets"; Flags: ignoreversion recursesubdirs

[Icons]
; Create a shortcut to the binaries folder [cite: 5]
Name: "{group}\Browse Example Binaries"; Filename: "{app}\bin"
Name: "{group}\Uninstall Goxpyriment Examples"; Filename: "{uninstallexe}"
