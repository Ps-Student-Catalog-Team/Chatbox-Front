[Setup]
AppId={{8A7E1C2B-3F4D-5E6F-7A8B-9C0D1E2F3A4B}
AppName=Chatbox-Front
AppVersion=1.9.7
AppPublisher=Ps-Student-Catalog-Team
DefaultDirName={autopf}\Chatbox
DefaultGroupName=Chatbox-front
AllowNoIcons=yes
OutputDir=Output
OutputBaseFilename=Chatbox_Setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Languages]

[Messages]

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "后端.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "*.html"; DestDir: "{app}"; Flags: ignoreversion
Source: "css\*"; DestDir: "{app}\css"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "js\*"; DestDir: "{app}\js"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "photos\*"; DestDir: "{app}\photos"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "custom-extensions\*"; DestDir: "{app}\custom-extensions"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "config.txt"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Chatbox"; Filename: "{app}\后端.exe"
Name: "{userdesktop}\Chatbox"; Filename: "{app}\后端.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\后端.exe"; Description: "{cm:LaunchProgram,Chatbox}"; Flags: nowait postinstall skipifsilent