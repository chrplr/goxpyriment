; Inno Setup script for packaging goxpyriment examples on Windows.
; Build this on Windows with Inno Setup; the installer EXE will be written
; next to this script (in examples\installers).

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
; Grab only the compiled .exe files from each example folder 
Source: "..\*\*.exe"; DestDir: "{app}\bin"; Flags: recursesubdirs ignoreversion

; Include assets needed by the examples 
Source: "..\..\assets\*"; DestDir: "{app}\assets"; Flags: ignoreversion 

[Icons]
; Create a shortcut to the binaries folder
Name: "{group}\Goxpyriment Examples"; Filename: "{app}\bin" 


