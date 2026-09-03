; Inno Setup Script for Kite VPN Client
; Download Inno Setup from: https://jrsoftware.org/isdl.php
; Compile using: iscc kite.iss

#define MyAppName "Kite"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Kite"
#define MyAppURL "https://github.com/KiteXRay/desktop"
#define MyAppExeName "kite.exe"

[Setup]
AppId={{E8B62569-8C9B-4F8A-9D45-7B034E5D4761}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
AppendDefaultDirName=no
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\..\bin
OutputBaseFilename=kite-windows-setup
SetupIconFile=..\icon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "..\..\bin\kite.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\icon.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
