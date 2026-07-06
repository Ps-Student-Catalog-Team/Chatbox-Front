[Setup]
AppId={{8A7E1C2B-3F4D-5E6F-7A8B-9C0D1E2F3A4B}
AppName=Chatbox后端
AppVersion=1.7.0
AppPublisher=Ps-Student-Catalog-Team
DefaultDirName={autopf}\Chatbox
DefaultGroupName=Chatbox-front+backend
AllowNoIcons=yes
OutputDir=D:\chat\Chatbox-Front-1\Output
OutputBaseFilename=Chatbox_Setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "D:\chat\Chatbox-Front-1\后端.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\admin.html"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\chatHistory.js"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\forgot-password.html"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\index.html"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\style-desktop.css"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\style-mobile.css"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\tailwind.browser.js"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\tailwind.js"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\viewport-adapter.js"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\accounts-desktop.css"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\accounts-mobile.css"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\accounts.html"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\notification.js"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\settings.html"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\chat\Chatbox-Front-1\aiChat.js"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Chatbox 系统"; Filename: "{app}\后端.exe"
Name: "{userdesktop}\Chatbox 系统"; Filename: "{app}\后端.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\后端.exe"; Description: "{cm:LaunchProgram,Chatbox 系统}"; Flags: nowait postinstall skipifsilent