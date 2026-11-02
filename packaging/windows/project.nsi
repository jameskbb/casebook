Unicode true

####
## Casebook NSIS project, based on the Wails v2.14.0 template.
##
## .github/workflows/release.yml copies every file in packaging/windows/ into
## desktop/build/windows/installer before running `wails build -nsis`. Wails
## keeps this project.nsi (it only writes the template when the file is
## missing) and regenerates wails_tools.nsh on every build.
##
## The installer puts the desktop app and the CLI in one folder, following
## docs/decisions/0012-cli-placement.md:
##   <install dir>\Casebook.exe
##   <install dir>\bin\casebook.exe
##
## The CLI path comes from the CASEBOOK_CLI_BINARY environment variable at
## compile time (or -DCASEBOOK_CLI_BINARY=<path> on the makensis command
## line). Without it the installer does not compile.
##
## The optional "Add the casebook command to PATH" component runs
## casebook-path.ps1 (copied next to this file by the workflow), which adds or
## removes exactly <install dir>\bin and keeps other PATH entries unexpanded.
##
## Signing happens in the release workflow, not here.
####
!ifndef CASEBOOK_CLI_BINARY
  !define CASEBOOK_CLI_BINARY "$%CASEBOOK_CLI_BINARY%"
!endif
# Check that the file exists rather than comparing with "": Windows cannot
# hold an empty environment variable, and an unset one leaves the literal
# $%CASEBOOK_CLI_BINARY% text behind, which no empty-string test catches.
!if /FileExists "${CASEBOOK_CLI_BINARY}"
!else
  !error "CASEBOOK_CLI_BINARY does not name an existing casebook.exe (set it to the path of the built CLI): ${CASEBOOK_CLI_BINARY}"
!endif
!define CASEBOOK_CLI_SUBDIR "bin"
!define CASEBOOK_CLI_EXE "casebook.exe"

####
## The following information is taken from the ProjectInfo file (wails.json),
## but can be overwritten here:
## INFO_PROJECTNAME, INFO_COMPANYNAME, INFO_PRODUCTNAME, INFO_PRODUCTVERSION,
## INFO_COPYRIGHT, PRODUCT_EXECUTABLE, UNINST_KEY_NAME, REQUEST_EXECUTION_LEVEL.
####
# The installed desktop executable is Casebook.exe (CB_GUI_NAME in
# scripts/lib.sh, docs/decisions/0012-cli-placement.md), not whatever the
# wails.json name field says, so the installer matches the zip layout.
!define PRODUCT_EXECUTABLE "Casebook.exe"
!include "wails_tools.nsh"

# PATH scope follows the install scope: the default Wails installer is
# per-machine and elevated (HKLM); a per-user build edits HKCU.
!if "${REQUEST_EXECUTION_LEVEL}" == "user"
  !define CASEBOOK_PATH_SCOPE "User"
!else
  !define CASEBOOK_PATH_SCOPE "Machine"
!endif
!define CASEBOOK_PATH_HELPER "casebook-path.ps1"
!define CASEBOOK_POWERSHELL "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_COMPONENTS # Required app plus the optional PATH entry.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
# The per-machine folder is Program Files\Casebook whatever the company name
# in wails.json says (the Wails template would nest it under the company
# name); the docs and the Connect AI clients panel assume this path.
!define CASEBOOK_MACHINE_DIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "${CASEBOOK_MACHINE_DIR}"
  !endif
!else
  InstallDir "${CASEBOOK_MACHINE_DIR}"
!endif # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Section "Casebook (required)" SecCasebook
    SectionIn RO

    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    # The CLI goes in bin\ next to Casebook.exe (docs/decisions/0012-cli-placement.md).
    # No /nonfatal: a missing CLI fails the installer build.
    SetOutPath "$INSTDIR\${CASEBOOK_CLI_SUBDIR}"
    File "/oname=${CASEBOOK_CLI_EXE}" "${CASEBOOK_CLI_BINARY}"
    SetOutPath $INSTDIR

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

# Unchecked by default: AI client setup stores the absolute CLI path, so PATH
# is only for people who type casebook in a terminal. The install folder
# reaches PowerShell only as a -File parameter value, never as code.
Section /o "Add the casebook command to PATH" SecPath
    InitPluginsDir
    File "/oname=$PLUGINSDIR\${CASEBOOK_PATH_HELPER}" "${CASEBOOK_PATH_HELPER}"
    nsExec::ExecToLog '"${CASEBOOK_POWERSHELL}" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$PLUGINSDIR\${CASEBOOK_PATH_HELPER}" -Action Add -Dir "$INSTDIR\${CASEBOOK_CLI_SUBDIR}" -Scope ${CASEBOOK_PATH_SCOPE}'
    Pop $0
    ${If} $0 != "0"
        DetailPrint "PATH could not be updated (result: $0). Casebook is installed; add $INSTDIR\${CASEBOOK_CLI_SUBDIR} to PATH by hand to use the casebook command."
    ${EndIf}
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SecCasebook} "The Casebook desktop app and the casebook command line tool (in the bin folder)."
    !insertmacro MUI_DESCRIPTION_TEXT ${SecPath} "Add the bin folder to PATH so the casebook command works in any new terminal. Not needed for connecting AI clients."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall"
    # The WebView2 data folder is per user (%AppData%\Casebook.exe by
    # default), but wails.setShellContext selects the all-users context for
    # the default admin install, where $AppData is C:\ProgramData. Switch to
    # the current user to remove the cache of the account running the
    # uninstaller; other accounts keep theirs (docs/privacy.md says so).
    SetShellVarContext current
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"
    !insertmacro wails.setShellContext

    # Remove the PATH entry the installer may have added; harmless when it
    # was never added. A failure only prints a detail line.
    InitPluginsDir
    File "/oname=$PLUGINSDIR\${CASEBOOK_PATH_HELPER}" "${CASEBOOK_PATH_HELPER}"
    nsExec::ExecToLog '"${CASEBOOK_POWERSHELL}" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "$PLUGINSDIR\${CASEBOOK_PATH_HELPER}" -Action Remove -Dir "$INSTDIR\${CASEBOOK_CLI_SUBDIR}" -Scope ${CASEBOOK_PATH_SCOPE}'
    Pop $0
    ${If} $0 != "0"
        DetailPrint "PATH entry $INSTDIR\${CASEBOOK_CLI_SUBDIR} could not be removed (result: $0); remove it by hand if present."
    ${EndIf}

    # Delete exactly the files the installer wrote, never the folder
    # recursively: the user may have chosen an existing folder that holds
    # unrelated files. Casebook's own data in %LocalAppData%\Casebook is
    # intentionally kept; docs/privacy.md explains how to delete it.
    Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
    Delete "$INSTDIR\${CASEBOOK_CLI_SUBDIR}\${CASEBOOK_CLI_EXE}"
    RMDir "$INSTDIR\${CASEBOOK_CLI_SUBDIR}"

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    # Deletes uninstall.exe and the uninstall registry key.
    !insertmacro wails.deleteUninstaller

    # Not /r: removes the install folder only when it is now empty.
    RMDir "$INSTDIR"
SectionEnd
