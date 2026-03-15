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
Source: "..\*.exe"; DestDir: "{app}\bin"; Flags: recursesubdirs ignoreversion skipifsourcedoesntexist; Excludes: "installers\*"

; 2. Install the 'assets' subfolders for each individual example.
; Inno Setup does not support wildcards in intermediate path components, so each
; example with assets must be listed explicitly. DestDir mirrors the subdirectory
; structure so that when an .exe runs from its own folder it can find "assets\" via
; a relative path (which is how all examples locate their media files).
Source: "..\Posner_task_simple\assets\*";  DestDir: "{app}\bin\Posner_task_simple\assets";  Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\card_game\assets\*";           DestDir: "{app}\bin\card_game\assets";           Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\hello_world\assets\*";         DestDir: "{app}\bin\hello_world\assets";         Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\play_gvideo\assets\*";         DestDir: "{app}\bin\play_gvideo\assets";         Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\play_two_videos\assets\*";     DestDir: "{app}\bin\play_two_videos\assets";     Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\play_videos\assets\*";         DestDir: "{app}\bin\play_videos\assets";         Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\retinotopy\assets\*";          DestDir: "{app}\bin\retinotopy\assets";          Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\simple_example\assets\*";      DestDir: "{app}\bin\simple_example\assets";      Flags: recursesubdirs ignoreversion skipifsourcedoesntexist
Source: "..\test_stream_images\assets\*";  DestDir: "{app}\bin\test_stream_images\assets";  Flags: recursesubdirs ignoreversion skipifsourcedoesntexist

; 3. Install the global assets folder from the root directory [cite: 51]
Source: "..\..\assets\*"; DestDir: "{app}\assets"; Flags: ignoreversion recursesubdirs

[Icons]
Name: "{group}\Browse Example Binaries"; Filename: "{app}\bin"
Name: "{group}\Uninstall Goxpyriment Examples"; Filename: "{uninstallexe}"
