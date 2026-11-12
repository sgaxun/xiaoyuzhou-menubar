; Override MyGo's per-user default without changing the application name.
; Keep shortcuts/registration per user; only the program files require elevation.
RequestExecutionLevel admin
InstallDir "$PROGRAMFILES64\XiaoyuzhouPlayer"
; A separate remembered location prevents an older AppData install from
; overriding the new default. /D= still overrides this for explicit installs.
InstallDirRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\dev.sgaxun.xiaoyuzhou-menubar" "ProgramFilesInstallLocation"

!macro customInstall
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\dev.sgaxun.xiaoyuzhou-menubar" "ProgramFilesInstallLocation" "$INSTDIR"
!macroend
