Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

!ifndef INFO_DISPLAYVERSION
  !define INFO_DISPLAYVERSION "${INFO_PRODUCTVERSION}"
!endif

# Keep the installed application name for metadata and migration, while using
# the shorter customer-facing name for Start menu and desktop shortcuts.
!define SHORTCUT_NAME "Rescene"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_DISPLAYVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}" # Offer to launch Rescene immediately; checked by default.
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Section
    !insertmacro wails.setShellContext

    # The application executable cannot be replaced while it is running.
    # Ask Windows to close it first, then force termination only if it did not
    # leave within the grace period. A missing process is harmless here.
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    Sleep 1500
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    # Earlier portable/development builds used the compact ResceneAgent name.
    # Remove their launch entries before recreating the canonical shortcuts so
    # upgrades do not leave an old name and icon next to the installed app.
    Delete "$SMPROGRAMS\ResceneAgent.lnk"
    Delete "$DESKTOP\ResceneAgent.lnk"
    Delete "$SMSTARTUP\ResceneAgent.lnk"
    Delete "$SMPROGRAMS\Rescene.lnk"
    Delete "$DESKTOP\Rescene.lnk"
    Delete "$SMSTARTUP\Rescene.lnk"
    Delete "$INSTDIR\ResceneAgent.exe"
    Delete "$INSTDIR\server.exe"
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    Delete "$SMSTARTUP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${SHORTCUT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${SHORTCUT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortcut "$SMSTARTUP\${SHORTCUT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "--background"

    # Notify Explorer that shortcut/icon data changed. Without this, Windows can
    # keep displaying the icon cached for a previous executable at the same path.
    System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller

    # Windows requires a numeric executable version, but Apps & Features can
    # display the complete SemVer (for example 0.1.2-alpha.4).
    !ifdef WAILS_INSTALL_SCOPE
      !if "${WAILS_INSTALL_SCOPE}" == "user"
        WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${INFO_DISPLAYVERSION}"
      !else
        WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${INFO_DISPLAYVERSION}"
      !endif
    !else
      WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${INFO_DISPLAYVERSION}"
    !endif
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    Delete "$SMSTARTUP\${INFO_PRODUCTNAME}.lnk"
    Delete "$SMPROGRAMS\${SHORTCUT_NAME}.lnk"
    Delete "$DESKTOP\${SHORTCUT_NAME}.lnk"
    Delete "$SMSTARTUP\${SHORTCUT_NAME}.lnk"
    Delete "$SMPROGRAMS\ResceneAgent.lnk"
    Delete "$DESKTOP\ResceneAgent.lnk"
    Delete "$SMSTARTUP\ResceneAgent.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
