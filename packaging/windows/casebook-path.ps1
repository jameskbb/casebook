<#
.SYNOPSIS
Adds or removes exactly one folder in the Path environment variable.

.DESCRIPTION
Used by the Casebook installer (packaging/windows/project.nsi) for the
optional "Add the casebook command to PATH" component and by its
uninstaller. Only the CLI folder (<install dir>\bin, see
docs/decisions/0012-cli-placement.md) is ever passed in, so typing casebook
never starts the desktop app.

Path is read from the registry with DoNotExpandEnvironmentNames and written
back as ExpandString, so entries such as %SystemRoot%\system32 or
%JAVA_HOME%\bin stay unexpanded. The .NET environment-variable setter is not
used for Path on purpose: it reads expanded values and writes REG_SZ, which
would flatten every %VAR% reference for good.

Compatible with Windows PowerShell 5.1. Keep this file ASCII only.

Exit codes: 0 success (including nothing to change), 1 error.

.PARAMETER Action
Add appends Dir when it is not present; Remove drops every matching entry.

.PARAMETER Dir
The folder to add or remove. Matching ignores case and trailing backslashes.

.PARAMETER Scope
Machine (HKLM, needs elevation; the default installer is per-machine) or
User (HKCU).
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('Add', 'Remove')]
    [string]$Action,

    [Parameter(Mandatory = $true)]
    [string]$Dir,

    [ValidateSet('Machine', 'User')]
    [string]$Scope = 'Machine'
)

$ErrorActionPreference = 'Stop'

function Test-SamePathEntry([string]$Entry, [string]$Target) {
    return [string]::Equals($Entry.Trim().TrimEnd('\'), $Target, [System.StringComparison]::OrdinalIgnoreCase)
}

function Send-EnvironmentChange {
    # WM_SETTINGCHANGE with lParam "Environment" tells Explorer and other
    # top-level windows to reload the environment, so new terminals see the
    # change without a sign-out.
    $signature = @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
public static extern IntPtr SendMessageTimeout(
    IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    if (-not ('CasebookInstaller.NativeMethods' -as [type])) {
        Add-Type -Namespace 'CasebookInstaller' -Name 'NativeMethods' -MemberDefinition $signature
    }
    $HWND_BROADCAST = [IntPtr]0xffff
    $WM_SETTINGCHANGE = [uint32]0x001A
    $SMTO_ABORTIFHUNG = [uint32]0x0002
    $result = [UIntPtr]::Zero
    [void][CasebookInstaller.NativeMethods]::SendMessageTimeout(
        $HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment',
        $SMTO_ABORTIFHUNG, [uint32]5000, [ref]$result)
}

try {
    $target = $Dir.Trim().TrimEnd('\')
    if ($target.Length -eq 0) {
        throw 'Dir must not be empty'
    }
    if ($target.IndexOfAny([char[]]@(';', '"')) -ge 0) {
        throw 'Dir must not contain a semicolon or a double quote'
    }

    if ($Scope -eq 'Machine') {
        $root = [Microsoft.Win32.Registry]::LocalMachine
        $subKey = 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment'
        $label = 'HKLM'
    }
    else {
        $root = [Microsoft.Win32.Registry]::CurrentUser
        $subKey = 'Environment'
        $label = 'HKCU'
    }

    $key = $root.OpenSubKey($subKey, $true)
    if ($null -eq $key) {
        if ($Scope -eq 'User') {
            $key = $root.CreateSubKey($subKey)
        }
        if ($null -eq $key) {
            throw "cannot open $label\$subKey for writing"
        }
    }

    try {
        $current = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $entries = @($current.Split(';') | Where-Object { $_.Trim().Length -gt 0 })
        $matching = @($entries | Where-Object { Test-SamePathEntry $_ $target })

        $changed = $false
        if ($Action -eq 'Add') {
            if ($matching.Count -gt 0) {
                Write-Output "casebook-path: $target is already in the $Scope Path; nothing changed"
            }
            else {
                $entries = @($entries) + @($target)
                $changed = $true
            }
        }
        else {
            if ($matching.Count -eq 0) {
                Write-Output "casebook-path: $target is not in the $Scope Path; nothing changed"
            }
            else {
                $entries = @($entries | Where-Object { -not (Test-SamePathEntry $_ $target) })
                $changed = $true
            }
        }

        if ($changed) {
            $key.SetValue('Path', ($entries -join ';'), [Microsoft.Win32.RegistryValueKind]::ExpandString)
            try {
                Send-EnvironmentChange
            }
            catch {
                Write-Output "casebook-path: Path updated, but the change broadcast failed; new terminals may need a sign-out: $($_.Exception.Message)"
            }
            if ($Action -eq 'Add') {
                Write-Output "casebook-path: added $target to the $Scope Path"
            }
            else {
                Write-Output "casebook-path: removed $target from the $Scope Path"
            }
        }
    }
    finally {
        $key.Close()
    }
    exit 0
}
catch {
    Write-Output "casebook-path: error: $($_.Exception.Message)"
    exit 1
}
