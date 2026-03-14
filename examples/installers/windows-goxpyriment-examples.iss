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
; Install the whole examples tree except installers themselves.
; Assumes you've already built the example binaries with build.sh.
Source: "..\*"; DestDir: "{app}\examples"; Flags: recursesubdirs createallsubdirs; Excludes: "..\installers\*"

[Icons]
; Start Menu shortcut to the examples folder in Explorer
Name: "{group}\Open Examples Folder"; Filename: "{cmd}"; Parameters: "/C start """" ""{app}\examples"""

