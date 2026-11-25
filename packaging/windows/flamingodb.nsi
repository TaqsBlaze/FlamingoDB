Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"
!define APP_VERSION "1.3.0"
!ifndef BUILD_DIR
  !define BUILD_DIR "../../build/windows"
!endif
!ifndef OUTPUT_FILE
  !define OUTPUT_FILE "../../build/FlamingoDB-Setup-${APP_VERSION}-x64.exe"
!endif
Name "FlamingoDB"
OutFile "${OUTPUT_FILE}"
InstallDir "$LOCALAPPDATA\Programs\FlamingoDB"
RequestExecutionLevel user
SetCompressor /SOLID lzma
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\FlamingoDB.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch FlamingoDB"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Function CheckStopped
  System::Call 'kernel32::OpenMutexW(i 0x100000, i 0, w "Local\FlamingoDBLauncher") p .r0'
  ${If} $0 != 0
    System::Call 'kernel32::CloseHandle(p r0)'
    MessageBox MB_OK|MB_ICONEXCLAMATION "FlamingoDB is running. Use the Stop FlamingoDB shortcut, then try again."
    Abort
  ${EndIf}
FunctionEnd
Function un.CheckStopped
  System::Call 'kernel32::OpenMutexW(i 0x100000, i 0, w "Local\FlamingoDBLauncher") p .r0'
  ${If} $0 != 0
    System::Call 'kernel32::CloseHandle(p r0)'
    MessageBox MB_OK|MB_ICONEXCLAMATION "FlamingoDB is running. Use the Stop FlamingoDB shortcut, then try again."
    Abort
  ${EndIf}
FunctionEnd
Function .onInit
  Call CheckStopped
FunctionEnd
Function un.onInit
  Call un.CheckStopped
FunctionEnd

Section "FlamingoDB" SEC_APP
  SetOutPath "$INSTDIR"
  File "${BUILD_DIR}/flamingodbd.exe"
  File /oname=FlamingoDB.exe "${BUILD_DIR}/FlamingoDB.exe"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateDirectory "$SMPROGRAMS\FlamingoDB"
  CreateShortcut "$SMPROGRAMS\FlamingoDB\FlamingoDB.lnk" "$INSTDIR\FlamingoDB.exe"
  CreateShortcut "$SMPROGRAMS\FlamingoDB\Stop FlamingoDB.lnk" "$INSTDIR\FlamingoDB.exe" "--stop"
  CreateShortcut "$DESKTOP\FlamingoDB.lnk" "$INSTDIR\FlamingoDB.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\FlamingoDB" "DisplayName" "FlamingoDB"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\FlamingoDB" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\FlamingoDB" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\FlamingoDB" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\FlamingoDB" "NoRepair" 1
SectionEnd
Section "Uninstall"
  Delete "$DESKTOP\FlamingoDB.lnk"
  Delete "$SMPROGRAMS\FlamingoDB\FlamingoDB.lnk"
  Delete "$SMPROGRAMS\FlamingoDB\Stop FlamingoDB.lnk"
  RMDir "$SMPROGRAMS\FlamingoDB"
  Delete "$INSTDIR\FlamingoDB.exe"
  Delete "$INSTDIR\flamingodbd.exe"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\FlamingoDB"
SectionEnd
