[Setup]
AppId={{8A7E1C2B-3F4D-5E6F-7A8B-9C0D1E2F3A4B}
AppName=Chatbox后端
AppVersion=1.7.8
AppPublisher=Ps-Student-Catalog-Team
DefaultDirName={autopf}\Chatbox
DefaultGroupName=Chatbox-front
AllowNoIcons=yes
OutputDir=D:\项目\Chatbox-Front\Output
OutputBaseFilename=Chatbox_Setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"

[Messages]
chinesesimplified.WelcomeLabel1=欢迎使用聊天室安装向导！
chinesesimplified.WelcomeLabel2=即将将聊天室安装到您的电脑中。
chinesesimplified.SelectDirLabel3=请选择安装位置：
chinesesimplified.ReadyLabel1=安装准备就绪，点击“安装”开始安装聊天室。

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "D:\项目\Chatbox-Front\后端.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\项目\Chatbox-Front\*.html"; DestDir: "{app}"; Flags: ignoreversion
Source: "D:\项目\Chatbox-Front\css\*"; DestDir: "{app}\css"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "D:\项目\Chatbox-Front\js\*"; DestDir: "{app}\js"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\Chatbox 系统"; Filename: "{app}\后端.exe"
Name: "{userdesktop}\Chatbox 系统"; Filename: "{app}\后端.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\后端.exe"; Description: "{cm:LaunchProgram,Chatbox 系统}"; Flags: nowait postinstall skipifsilent