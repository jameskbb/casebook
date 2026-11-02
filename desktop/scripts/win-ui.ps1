# Windows host UI helper for Casebook smoke checks (D-27). Run through
# desktop/scripts/win-run.sh ui <action>.
#
# The person at the Windows machine keeps working while this runs: no action
# activates a window, moves the cursor or injects global input. The dev
# window is parked off-screen (left of every monitor) without activation;
# elements are driven through UI Automation, clicks and keys are posted to
# the WebView2 window itself, and captures use PrintWindow, which renders an
# off-screen window. Each action checks that the cursor stayed put and that
# no Casebook window took the foreground, prints a warning when either
# happened, and appends it to win-ui-warnings.log in the dev folder.
#
#   -Action waitwindow [-TimeoutSeconds 20]
#       Exit 0 once a visible top-level window titled Casebook owned by
#       Casebook.exe exists, after parking it off-screen.
#   -Action stop
#       Stop every Casebook.exe started from this folder (exit 0 when none).
#   -Action find -Text <substring> [-TimeoutSeconds 20]
#       Exit 0 when any UI Automation element under the Casebook window has a
#       Name containing the substring, else 1.
#   -Action capture -Out <path.png> [-Width 1280 -Height 840] [-Client] [-CropLeft <px>]
#       Resize the window, then save it with PrintWindow (PW_RENDERFULLCONTENT).
#       With -Client the size is the client area (the web view) and the
#       capture leaves out the title bar and frame; -CropLeft then drops that
#       many pixels from the left (the sidebar).
#   -Action invoke -Text <name> [-Last] [-TimeoutSeconds 20]
#       Invoke (click) the first UI Automation element whose Name is exactly
#       the text (falling back to the first Name containing it); -Last picks
#       the last match instead.
#   -Action scroll -Text <substring> [-TimeoutSeconds 20]
#       Scroll the first element whose Name contains the text into view.
#   -Action keys -Text <SendKeys string>
#       Post keystrokes to the web view (a subset of SendKeys syntax: plain
#       characters, {NAME} or {NAME n} for ESC, ENTER, TAB, arrows, HOME,
#       END, PGUP, PGDN, DEL, BS, SPACE, and the ^ + % modifiers). The
#       page's focused element receives them once the page has taken a
#       click (invoke and click post one); the window is not activated.
#   -Action type -Text <field name> -Value <text>
#       Click into the first text field whose Name matches the text (a
#       labelled input), select its content and type the value with posted
#       keys, so React sees real key events.
#   -Action clipboard
#       Print the text on the Windows clipboard.
#   -Action click -X <px> -Y <px> [-Client]
#       Left-click at a point measured from the top-left of the visible
#       window frame (the same origin as capture), or of the client area
#       with -Client, by posting mouse messages to the web view. Clicking a
#       blank spot moves focus off a control the automation focused, so no
#       keyboard focus ring is left in a screenshot.
#   -Action wheel -X <px> -Y <px> -Notches <n> [-Delta 120] [-Client]
#       Turn the mouse wheel n steps down (negative: up) over that point by
#       posting WM_MOUSEWHEEL to the web view, for exact scroll positions in
#       screenshots. A step is -Delta wheel units (120 is one notch, 100px
#       in WebView2; smaller values scroll proportionally less).
#   -Action filedialog -Title <dialog title> -Text <full path>
#       Fill the open native file dialog titled <dialog title> (for example
#       "Restore from backup") with the path and confirm it: WM_SETTEXT to
#       its file name box and a posted IDOK command, as the Open button
#       sends. The dialog is parked off-screen beside the main window.
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('waitwindow', 'find', 'capture', 'stop', 'invoke', 'scroll', 'keys', 'filedialog', 'type', 'clipboard', 'click', 'wheel')]
    [string]$Action,
    [string]$Text = '',
    [string]$Value = '',
    [switch]$Last,
    [switch]$Uia,
    [string]$Title = '',
    [int]$TimeoutSeconds = 20,
    [string]$Out = '',
    [int]$Width = 1280,
    [int]$Height = 840,
    [int]$X = 0,
    [int]$Y = 0,
    [int]$Notches = 1,
    [int]$Delta = 120,
    [switch]$Client,
    [int]$CropLeft = 0
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Drawing

if (-not ('CasebookWin32' -as [type])) {
    # (Each run is a new PowerShell process, so the type is always fresh.)
    Add-Type @'
using System;
using System.Text;
using System.Collections.Generic;
using System.Runtime.InteropServices;
public static class CasebookWin32 {
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)]
    public struct POINT { public int X, Y; }
    public delegate bool EnumProc(IntPtr hWnd, IntPtr lParam);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr hWnd, ref POINT pt);
    [DllImport("user32.dll")] public static extern bool ScreenToClient(IntPtr hWnd, ref POINT pt);
    [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
    [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdc, uint flags);
    [DllImport("dwmapi.dll")] public static extern int DwmGetWindowAttribute(IntPtr hWnd, int attr, out RECT rect, int size);
    [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT pt);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern IntPtr GetAncestor(IntPtr hWnd, uint flags);
    [DllImport("user32.dll")] public static extern IntPtr GetWindow(IntPtr hWnd, uint cmd);
    [DllImport("user32.dll")] public static extern int GetSystemMetrics(int index);
    [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hWnd, uint msg, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr SendMessage(IntPtr hWnd, uint msg, IntPtr wParam, string lParam);
    [DllImport("user32.dll", CharSet = CharSet.Unicode, EntryPoint = "SendMessageW")] static extern IntPtr SendMessageText(IntPtr hWnd, uint msg, IntPtr wParam, StringBuilder lParam);
    [DllImport("user32.dll")] public static extern int GetDlgCtrlID(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern IntPtr GetParent(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
    [DllImport("user32.dll")] public static extern uint MapVirtualKey(uint code, uint mapType);
    [DllImport("kernel32.dll")] public static extern uint GetCurrentThreadId();
    [DllImport("user32.dll")] public static extern bool AttachThreadInput(uint attach, uint to, bool on);
    [DllImport("user32.dll")] public static extern bool GetKeyboardState(byte[] state);
    [DllImport("user32.dll")] public static extern bool SetKeyboardState(byte[] state);
    [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr hWnd, EnumProc proc, IntPtr lParam);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetClassName(IntPtr hWnd, StringBuilder name, int size);

    // The first descendant window of the given class and control id, or zero.
    public static IntPtr FindControl(IntPtr root, string cls, int id) {
        IntPtr found = IntPtr.Zero;
        EnumChildWindows(root, (h, l) => {
            if (ClassOf(h) == cls && GetDlgCtrlID(h) == id) { found = h; return false; }
            return true;
        }, IntPtr.Zero);
        return found;
    }

    // A control's text, even in another process (WM_GETTEXT).
    public static string TextOf(IntPtr h) {
        var sb = new StringBuilder(1024);
        SendMessageText(h, 0x000D, (IntPtr)sb.Capacity, sb);
        return sb.ToString();
    }

    public static string ClassOf(IntPtr h) {
        var sb = new StringBuilder(256);
        GetClassName(h, sb, sb.Capacity);
        return sb.ToString();
    }

    // The first descendant window of the given class, or zero.
    public static IntPtr FindChild(IntPtr root, string cls) {
        IntPtr found = IntPtr.Zero;
        EnumChildWindows(root, (h, l) => {
            if (ClassOf(h) == cls) { found = h; return false; }
            return true;
        }, IntPtr.Zero);
        return found;
    }
}
'@
}

# Physical pixels everywhere, so the capture matches the requested size.
[void][CasebookWin32]::SetProcessDPIAware()

# Only Casebook processes started from this dev folder (where win-run.sh
# copies the build) are ever automated, so a real Casebook the user runs is
# never touched.
$DevDir = $PSScriptRoot
$WarnLog = Join-Path $DevDir 'win-ui-warnings.log'

# SWP_NOZORDER | SWP_NOACTIVATE
$SwpQuiet = 0x0004 -bor 0x0010

$cursor0 = New-Object CasebookWin32+POINT
[void][CasebookWin32]::GetCursorPos([ref]$cursor0)
$foreground0 = [CasebookWin32]::GetForegroundWindow()

function Get-DevCasebook {
    Get-Process -Name 'Casebook' -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -and $_.Path.StartsWith($DevDir + '\', [System.StringComparison]::OrdinalIgnoreCase) }
}

function Get-CasebookWindow {
    $procs = Get-DevCasebook
    foreach ($p in $procs) {
        $p.Refresh()
        $h = $p.MainWindowHandle
        if ($h -ne [IntPtr]::Zero -and $p.MainWindowTitle -eq 'Casebook' -and [CasebookWin32]::IsWindowVisible($h)) {
            return $h
        }
    }
    return [IntPtr]::Zero
}

function Wait-CasebookWindow([int]$timeout) {
    $deadline = (Get-Date).AddSeconds($timeout)
    do {
        $h = Get-CasebookWindow
        if ($h -ne [IntPtr]::Zero) { return $h }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    return [IntPtr]::Zero
}

function Write-Warn([string]$msg) {
    [Console]::Out.WriteLine("win-ui: warning: $msg")
    try { Add-Content -Path $WarnLog -Value "$(Get-Date -Format s) $Action $Text : $msg" } catch { }
}

# Done <code>: report a moved cursor or a stolen foreground, then exit.
function Done([int]$code) {
    $c = New-Object CasebookWin32+POINT
    [void][CasebookWin32]::GetCursorPos([ref]$c)
    if ($c.X -ne $cursor0.X -or $c.Y -ne $cursor0.Y) {
        Write-Warn "the cursor moved from $($cursor0.X),$($cursor0.Y) to $($c.X),$($c.Y) (by the person, or by this action)"
    }
    $fg = [CasebookWin32]::GetForegroundWindow()
    # Right after a launch, any Casebook foreground counts.
    if (($fg -ne $foreground0 -or $Action -eq 'waitwindow') -and $fg -ne [IntPtr]::Zero) {
        $owner = [uint32]0
        [void][CasebookWin32]::GetWindowThreadProcessId($fg, [ref]$owner)
        if (@(Get-DevCasebook | Where-Object { $_.Id -eq $owner }).Count -gt 0) {
            Write-Warn 'a Casebook window took the foreground'
        }
    }
    exit $code
}

# Park <hwnd> [w h]: move a window left of every monitor, without activating
# it or changing the z-order (and with a size when given). PrintWindow still
# renders it there.
function Park([IntPtr]$h, [int]$w = 0, [int]$ht = 0) {
    if ([CasebookWin32]::IsIconic($h)) { [void][CasebookWin32]::ShowWindow($h, 4) }   # SW_SHOWNOACTIVATE
    $r = New-Object CasebookWin32+RECT
    [void][CasebookWin32]::GetWindowRect($h, [ref]$r)
    if ($w -le 0) { $w = $r.Right - $r.Left }
    if ($ht -le 0) { $ht = $r.Bottom - $r.Top }
    $left = [CasebookWin32]::GetSystemMetrics(76) - $w - 400   # SM_XVIRTUALSCREEN
    $top = [CasebookWin32]::GetSystemMetrics(77)               # SM_YVIRTUALSCREEN
    [void][CasebookWin32]::SetWindowPos($h, [IntPtr]::Zero, $left, $top, $w, $ht, $SwpQuiet)
}

if ($Action -eq 'clipboard') {
    Add-Type -AssemblyName System.Windows.Forms
    $clip = [System.Windows.Forms.Clipboard]::GetText()
    [Console]::Out.Write($clip)
    exit 0
}

if ($Action -eq 'stop') {
    $procs = @(Get-DevCasebook)
    if ($procs.Count -eq 0) { Write-Output 'win-ui: Casebook is not running'; exit 0 }
    $procs | Stop-Process -Force
    foreach ($p in $procs) { [void]$p.WaitForExit(10000) }
    Write-Output "win-ui: stopped $($procs.Count) Casebook process(es)"
    exit 0
}

$hwnd = Wait-CasebookWindow $TimeoutSeconds
if ($hwnd -eq [IntPtr]::Zero) {
    Write-Output "win-ui: no visible Casebook window after $TimeoutSeconds s"
    exit 1
}

# The first element whose Name equals $text (else contains it), or $null.
# With -Last the last match in tree order wins (an open drawer comes after
# the reading column, so its copy of a row name is the last one).
function Find-Element([IntPtr]$h, [string]$text, [int]$timeout) {
    $deadline = (Get-Date).AddSeconds($timeout)
    $root = [System.Windows.Automation.AutomationElement]::FromHandle($h)
    $all = [System.Windows.Automation.Condition]::TrueCondition
    do {
        try {
            $items = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $all)
            $partial = $null
            $exact = $null
            foreach ($el in $items) {
                $name = $el.Current.Name
                if (-not $name) { continue }
                if ($name -eq $text) {
                    if (-not $Last) { return $el }
                    $exact = $el
                }
                if ($name.Contains($text) -and ($Last -or -not $partial)) { $partial = $el }
            }
            if ($exact) { return $exact }
            if ($partial) { return $partial }
        } catch {
            # The WebView2 tree can change while it is walked; try again.
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    return $null
}

# The screen point for -X -Y: from the visible frame, or the client area
# with -Client.
function Get-TargetPoint([IntPtr]$h) {
    $pt = New-Object CasebookWin32+POINT
    if ($Client) {
        [void][CasebookWin32]::ClientToScreen($h, [ref]$pt)
    } else {
        $fr = New-Object CasebookWin32+RECT
        $size = [System.Runtime.InteropServices.Marshal]::SizeOf([type][CasebookWin32+RECT])
        if ([CasebookWin32]::DwmGetWindowAttribute($h, 9, [ref]$fr, $size) -ne 0) {
            [void][CasebookWin32]::GetWindowRect($h, [ref]$fr)
        }
        $pt.X = $fr.Left
        $pt.Y = $fr.Top
    }
    $pt.X += $X
    $pt.Y += $Y
    return $pt
}

# The WebView2 input windows: Chrome_RenderWidgetHostHWND takes mouse
# messages, and its parent Chrome_WidgetWin_1 (the browser's widget) takes
# keyboard messages.
function Get-WebViewWindows([IntPtr]$h) {
    $render = [CasebookWin32]::FindChild($h, 'Chrome_RenderWidgetHostHWND')
    $widget = [CasebookWin32]::FindChild($h, 'Chrome_WidgetWin_1')
    if ($render -eq [IntPtr]::Zero -or $widget -eq [IntPtr]::Zero) {
        [Console]::Out.WriteLine('win-ui: no WebView2 window found')
        Done 1
    }
    return @($render, $widget)
}

function MakeLParam([int]$lo, [int]$hi) {
    return [IntPtr](([int]($hi -band 0xFFFF) -shl 16) -bor ($lo -band 0xFFFF))
}

# Invoke-Pattern <element>: the UI Automation way to press a control,
# which reaches it even under an overlay (invoke -Uia).
function Invoke-Pattern($el) {
    $pattern = $null
    if ($el.TryGetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern, [ref]$pattern)) {
        $pattern.Invoke()
    } elseif ($el.TryGetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern, [ref]$pattern)) {
        $pattern.Select()
    } elseif ($el.TryGetCurrentPattern([System.Windows.Automation.TogglePattern]::Pattern, [ref]$pattern)) {
        $pattern.Toggle()
    } elseif ($el.TryGetCurrentPattern([System.Windows.Automation.ExpandCollapsePattern]::Pattern, [ref]$pattern)) {
        # Menu buttons (aria-haspopup) expose expand and collapse only.
        $pattern.Expand()
    } else {
        Write-Output "win-ui: '$Text' cannot be invoked"
        Done 1
    }
}

# Send-Click <screen point>: post a move, press and release of the left
# button to the web view at that point. Chromium hit-tests the point in the
# page; the real cursor stays where it is.
function Send-Click($pt) {
    $render, $widget = Get-WebViewWindows $hwnd
    $p = New-Object CasebookWin32+POINT
    $p.X = $pt.X
    $p.Y = $pt.Y
    [void][CasebookWin32]::ScreenToClient($render, [ref]$p)
    $lp = MakeLParam $p.X $p.Y
    [void][CasebookWin32]::PostMessage($render, 0x0200, [IntPtr]::Zero, $lp)   # WM_MOUSEMOVE
    Start-Sleep -Milliseconds 60
    [void][CasebookWin32]::PostMessage($render, 0x0201, [IntPtr]1, $lp)        # WM_LBUTTONDOWN, MK_LBUTTON
    Start-Sleep -Milliseconds 60
    [void][CasebookWin32]::PostMessage($render, 0x0202, [IntPtr]::Zero, $lp)   # WM_LBUTTONUP
}

# The screen centre of an element when it lies inside the web view's client
# area, else $null.
function Get-ElementCentre($el) {
    $r = $el.Current.BoundingRectangle
    if ($r.IsEmpty -or $r.Width -le 0 -or $r.Height -le 0) { return $null }
    $render, $widget = Get-WebViewWindows $hwnd
    $cr = New-Object CasebookWin32+RECT
    [void][CasebookWin32]::GetClientRect($render, [ref]$cr)
    $o = New-Object CasebookWin32+POINT
    [void][CasebookWin32]::ClientToScreen($render, [ref]$o)
    $p = New-Object CasebookWin32+POINT
    $p.X = [int][Math]::Floor($r.Left + $r.Width / 2)
    $p.Y = [int][Math]::Floor($r.Top + $r.Height / 2)
    if ($p.X -lt $o.X -or $p.Y -lt $o.Y -or $p.X -ge $o.X + $cr.Right -or $p.Y -ge $o.Y + $cr.Bottom) { return $null }
    return $p
}

# Virtual-key codes for the keys action.
$KeyCodes = @{
    'ESC' = 0x1B; 'ESCAPE' = 0x1B; 'ENTER' = 0x0D; 'TAB' = 0x09; 'LEFT' = 0x25; 'UP' = 0x26
    'RIGHT' = 0x27; 'DOWN' = 0x28; 'HOME' = 0x24; 'END' = 0x23; 'PGUP' = 0x21; 'PGDN' = 0x22
    'DEL' = 0x2E; 'DELETE' = 0x2E; 'BS' = 0x08; 'BACKSPACE' = 0x08; 'SPACE' = 0x20
}

# Send-Key <window> <vk>: post one key press (down and up). Extended keys
# (arrows, Home, End, Page, Delete) carry bit 24. The target's message loop
# translates the press into the character it types, as for a real key.
function Send-Key([IntPtr]$target, [int]$vk) {
    $scan = [CasebookWin32]::MapVirtualKey([uint32]$vk, 0)
    $ext = if ($vk -in 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2E) { 1 -shl 24 } else { 0 }
    $down = [IntPtr](1 -bor ($scan -shl 16) -bor $ext)
    $up = [IntPtr]([int64](1 -bor ($scan -shl 16) -bor $ext -bor (1 -shl 30)) -bor 0x80000000L)
    [void][CasebookWin32]::PostMessage($target, 0x0100, [IntPtr]$vk, $down)   # WM_KEYDOWN
    Start-Sleep -Milliseconds 40
    [void][CasebookWin32]::PostMessage($target, 0x0101, [IntPtr]$vk, $up)     # WM_KEYUP
    Start-Sleep -Milliseconds 60
}

# Send-KeyString <SendKeys string>: post the keys to the web view (see the
# keys action).
function Send-KeyString([string]$keyString) {
    $render, $widget = Get-WebViewWindows $hwnd
    # Chromium reads Ctrl, Shift and Alt from the keyboard state of its
    # own input thread. For a chord, share that thread's input state for
    # a moment and set the modifiers there; no other thread's state or
    # the real keyboard changes.
    $me = [CasebookWin32]::GetCurrentThreadId()
    $pid0 = [uint32]0
    $them = [CasebookWin32]::GetWindowThreadProcessId($widget, [ref]$pid0)
    $i = 0
    $s = $keyString
    while ($i -lt $s.Length) {
        $mods = @()
        while ($i -lt $s.Length -and '^+%'.Contains($s[$i])) {
            $mods += $(switch ($s[$i]) { '^' { 0x11 } '+' { 0x10 } '%' { 0x12 } })
            $i++
        }
        if ($i -ge $s.Length) { break }
        $vk = 0
        $char = $null
        $count = 1
        if ($s[$i] -eq '{') {
            $end = $s.IndexOf('}', $i + 2)
            if ($end -lt 0) { Write-Output "win-ui: bad key string '$s'"; Done 2 }
            $tok = $s.Substring($i + 1, $end - $i - 1).Split(' ')
            $i = $end + 1
            if ($tok.Count -gt 1) { $count = [int]$tok[1] }
            if ($KeyCodes.ContainsKey($tok[0].ToUpper())) { $vk = $KeyCodes[$tok[0].ToUpper()] }
            elseif ($tok[0].Length -eq 1) { $char = $tok[0] }
            else { Write-Output "win-ui: unknown key {$($tok[0])}"; Done 2 }
        } else {
            $char = [string]$s[$i]
            $i++
        }
        for ($n = 0; $n -lt $count; $n++) {
            if ($mods.Count -gt 0) {
                $code = if ($vk) { $vk } else { [int][char]$char.ToUpper() }
                [void][CasebookWin32]::AttachThreadInput($me, $them, $true)
                $state = New-Object byte[] 256
                [void][CasebookWin32]::GetKeyboardState($state)
                $saved = $state.Clone()
                foreach ($m in $mods) { $state[$m] = 0x80 }
                [void][CasebookWin32]::SetKeyboardState($state)
                Send-Key $widget $code
                Start-Sleep -Milliseconds 300
                [void][CasebookWin32]::SetKeyboardState($saved)
                [void][CasebookWin32]::AttachThreadInput($me, $them, $false)
            } elseif ($vk) {
                Send-Key $widget $vk
            } else {
                [void][CasebookWin32]::PostMessage($widget, 0x0102, [IntPtr][int][char]$char, [IntPtr]1)   # WM_CHAR
                Start-Sleep -Milliseconds 40
            }
        }
    }
}

switch ($Action) {
    'invoke' {
        if (-not $Text) { Write-Output 'win-ui: -Text is required'; Done 2 }
        $el = Find-Element $hwnd $Text $TimeoutSeconds
        if (-not $el) { Write-Output "win-ui: '$Text' not found"; Done 1 }
        # Click the element's centre with posted mouse messages, as a person
        # would. (A UI Automation Invoke lets the app take the foreground,
        # and Chromium does when the page then moves focus, as a menu opens
        # or closes.) An element out of view is scrolled into view first.
        if ($Uia) {
            Invoke-Pattern $el
            Start-Sleep -Milliseconds 400
            Write-Output "win-ui: invoked '$Text' through UI Automation"
            Done 0
        }
        $pt = Get-ElementCentre $el
        if (-not $pt) {
            $sp = $null
            if ($el.TryGetCurrentPattern([System.Windows.Automation.ScrollItemPattern]::Pattern, [ref]$sp)) {
                $sp.ScrollIntoView()
                Start-Sleep -Milliseconds 400
                $pt = Get-ElementCentre $el
                Write-Output "win-ui: scrolled '$Text' into view first"
            }
        }
        if (-not $pt) { Write-Output "win-ui: '$Text' is not on screen in the window"; Done 1 }
        Send-Click $pt
        Start-Sleep -Milliseconds 400
        Write-Output "win-ui: invoked '$Text'"
        Done 0
    }

    'scroll' {
        if (-not $Text) { Write-Output 'win-ui: -Text is required'; Done 2 }
        $el = Find-Element $hwnd $Text $TimeoutSeconds
        if (-not $el) { Write-Output "win-ui: '$Text' not found"; Done 1 }
        $pattern = $null
        if (-not $el.TryGetCurrentPattern([System.Windows.Automation.ScrollItemPattern]::Pattern, [ref]$pattern)) {
            Write-Output "win-ui: '$Text' cannot scroll into view"
            Done 1
        }
        $pattern.ScrollIntoView()
        Start-Sleep -Milliseconds 400
        Write-Output "win-ui: scrolled to '$Text'"
        Done 0
    }

    'type' {
        if (-not $Text) { Write-Output 'win-ui: -Text is required'; Done 2 }
        # Only text inputs count: a label's own text node carries the same
        # Name but cannot take focus.
        $el = $null
        $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
        $root = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
        $edit = New-Object System.Windows.Automation.PropertyCondition(
            [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
            [System.Windows.Automation.ControlType]::Edit)
        do {
            try {
                $partial = $null
                foreach ($c in $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $edit)) {
                    $name = $c.Current.Name
                    if (-not $name -or -not $c.Current.IsKeyboardFocusable) { continue }
                    if ($name -eq $Text) { $el = $c; break }
                    if (-not $partial -and $name.StartsWith($Text)) { $partial = $c }
                }
                if (-not $el) { $el = $partial }
            } catch {
                # The WebView2 tree can change while it is walked; try again.
            }
            if (-not $el) { Start-Sleep -Milliseconds 500 }
        } while (-not $el -and (Get-Date) -lt $deadline)
        if (-not $el) { Write-Output "win-ui: no text field '$Text'"; Done 1 }
        # Click into the field, select its content and type over it with
        # posted keys, so React sees real key events. (Setting the value
        # through UI Automation lets the app take the foreground.)
        $pt = Get-ElementCentre $el
        if (-not $pt) {
            $sp = $null
            if ($el.TryGetCurrentPattern([System.Windows.Automation.ScrollItemPattern]::Pattern, [ref]$sp)) {
                $sp.ScrollIntoView()
                Start-Sleep -Milliseconds 400
                $pt = Get-ElementCentre $el
            }
        }
        if (-not $pt) { Write-Output "win-ui: '$Text' is not on screen in the window"; Done 1 }
        Send-Click $pt
        Start-Sleep -Milliseconds 300
        Send-KeyString '^a'
        if ($Value) {
            $render, $widget = Get-WebViewWindows $hwnd
            foreach ($ch in $Value.ToCharArray()) {
                [void][CasebookWin32]::PostMessage($widget, 0x0102, [IntPtr][int]$ch, [IntPtr]1)   # WM_CHAR
                Start-Sleep -Milliseconds 15
            }
        } else {
            Send-KeyString '{DEL}'
        }
        Start-Sleep -Milliseconds 500
        $pattern = $null
        if ($el.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$pattern) -and $pattern.Current.Value -ne $Value) {
            Write-Output "win-ui: '$Text' holds '$($pattern.Current.Value)', not the value"
            Done 1
        }
        Write-Output "win-ui: typed into '$Text'"
        Done 0
    }

    'click' {
        Send-Click (Get-TargetPoint $hwnd)
        Start-Sleep -Milliseconds 300
        Write-Output "win-ui: clicked at $X,$Y"
        Done 0
    }

    'wheel' {
        # WM_MOUSEWHEEL carries the point in screen coordinates. Chromium
        # scrolls the element under it whether or not the window is on
        # screen or in front.
        $render, $widget = Get-WebViewWindows $hwnd
        $pt = Get-TargetPoint $hwnd
        $lp = MakeLParam $pt.X $pt.Y
        $steps = [Math]::Abs($Notches)
        $delta = if ($Notches -lt 0) { $Delta } else { -$Delta }
        $wp = [IntPtr]([int](($delta -band 0xFFFF) -shl 16))
        for ($i = 0; $i -lt $steps; $i++) {
            [void][CasebookWin32]::PostMessage($render, 0x020A, $wp, $lp)   # WM_MOUSEWHEEL
            Start-Sleep -Milliseconds 120
        }
        Start-Sleep -Milliseconds 400
        Write-Output "win-ui: turned the wheel $Notches at $X,$Y"
        Done 0
    }

    'keys' {
        if (-not $Text) { Write-Output 'win-ui: -Text is required'; Done 2 }
        Send-KeyString $Text
        Start-Sleep -Milliseconds 300
        Write-Output 'win-ui: sent keys'
        Done 0
    }

    'filedialog' {
        if (-not $Title -or -not $Text) { Write-Output 'win-ui: -Title and -Text are required'; Done 2 }
        $A = [System.Windows.Automation.AutomationElement]
        $scope = [System.Windows.Automation.TreeScope]::Children
        $root = $A::FromHandle($hwnd)
        $byClass = New-Object System.Windows.Automation.PropertyCondition($A::ClassNameProperty, '#32770')
        $dlg = $null
        $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
        do {
            $d = $root.FindFirst($scope, $byClass)
            if ($d -and $d.Current.Name -eq $Title) { $dlg = $d } else { Start-Sleep -Milliseconds 250 }
        } while (-not $dlg -and (Get-Date) -lt $deadline)
        if (-not $dlg) { Write-Output "win-ui: no file dialog '$Title'"; Done 1 }
        $dh = [IntPtr]$dlg.Current.NativeWindowHandle
        # The dialog opens over its owner's monitor; park it beside the
        # main window so it never covers the screen.
        Park $dh
        # The file name box is the Edit inside the ComboBoxEx32 with control
        # id 1148 (cmb13); set its text with WM_SETTEXT, then send the
        # dialog IDOK as the Open or Save button would.
        $combo = [CasebookWin32]::FindControl($dh, 'ComboBoxEx32', 1148)
        $box = if ($combo -ne [IntPtr]::Zero) { [CasebookWin32]::FindChild($combo, 'Edit') } else { [IntPtr]::Zero }
        if ($box -eq [IntPtr]::Zero) { $box = [CasebookWin32]::FindControl($dh, 'Edit', 1148) }
        if ($box -eq [IntPtr]::Zero) { Write-Output "win-ui: no file name box in '$Title'"; Done 1 }
        [void][CasebookWin32]::SendMessage($box, 0x000C, [IntPtr]::Zero, $Text)   # WM_SETTEXT
        Start-Sleep -Milliseconds 200
        if ([CasebookWin32]::TextOf($box) -ne $Text) { Write-Output "win-ui: the file name box did not take '$Text'"; Done 1 }
        $ok = [CasebookWin32]::FindControl($dh, 'Button', 1)
        [void][CasebookWin32]::PostMessage($dh, 0x0111, [IntPtr]1, $ok)   # WM_COMMAND IDOK, BN_CLICKED
        $deadline = (Get-Date).AddSeconds(10)
        while ([CasebookWin32]::IsWindow($dh) -and [CasebookWin32]::IsWindowVisible($dh) -and (Get-Date) -lt $deadline) {
            Start-Sleep -Milliseconds 200
        }
        if ([CasebookWin32]::IsWindow($dh) -and [CasebookWin32]::IsWindowVisible($dh)) {
            Write-Output "win-ui: '$Title' stayed open"
            Done 1
        }
        Start-Sleep -Milliseconds 400
        Write-Output "win-ui: chose '$Text' in '$Title'"
        Done 0
    }

    'waitwindow' {
        Park $hwnd
        Write-Output 'win-ui: Casebook window is open'
        Done 0
    }

    'find' {
        if (-not $Text) { Write-Output 'win-ui: -Text is required'; Done 2 }
        $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
        $root = [System.Windows.Automation.AutomationElement]::FromHandle($hwnd)
        $all = [System.Windows.Automation.Condition]::TrueCondition
        do {
            try {
                $items = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $all)
                foreach ($el in $items) {
                    $name = $el.Current.Name
                    if ($name -and $name.Contains($Text)) {
                        Write-Output "win-ui: found '$Text'"
                        Done 0
                    }
                }
            } catch {
                # The WebView2 tree can change while it is walked; try again.
            }
            Start-Sleep -Milliseconds 500
        } while ((Get-Date) -lt $deadline)
        Write-Output "win-ui: '$Text' not found after $TimeoutSeconds s"
        Done 1
    }

    'capture' {
        if (-not $Out) { Write-Output 'win-ui: -Out is required'; Done 2 }

        # Size the visible frame (DWM extended bounds) to Width x Height, or
        # with -Client the client area; the window rect also holds invisible
        # resize borders (and the title bar, for -Client).
        if ([CasebookWin32]::IsIconic($hwnd)) { [void][CasebookWin32]::ShowWindow($hwnd, 4) }   # SW_SHOWNOACTIVATE
        $wr = New-Object CasebookWin32+RECT
        $fr = New-Object CasebookWin32+RECT
        $cr = New-Object CasebookWin32+RECT
        [void][CasebookWin32]::GetWindowRect($hwnd, [ref]$wr)
        $size = [System.Runtime.InteropServices.Marshal]::SizeOf([type][CasebookWin32+RECT])
        if ($Client) {
            [void][CasebookWin32]::GetClientRect($hwnd, [ref]$cr)
            $fr.Right = $cr.Right
            $fr.Bottom = $cr.Bottom
        } elseif ([CasebookWin32]::DwmGetWindowAttribute($hwnd, 9, [ref]$fr, $size) -ne 0) { $fr = $wr }
        $padW = ($wr.Right - $wr.Left) - ($fr.Right - $fr.Left)
        $padH = ($wr.Bottom - $wr.Top) - ($fr.Bottom - $fr.Top)
        Park $hwnd ($Width + $padW) ($Height + $padH)
        Start-Sleep -Milliseconds 800

        [void][CasebookWin32]::GetWindowRect($hwnd, [ref]$wr)
        if ($Client) {
            [void][CasebookWin32]::GetClientRect($hwnd, [ref]$cr)
            $pt = New-Object CasebookWin32+POINT
            [void][CasebookWin32]::ClientToScreen($hwnd, [ref]$pt)
            $fr.Left = $pt.X
            $fr.Top = $pt.Y
            $fr.Right = $pt.X + $cr.Right
            $fr.Bottom = $pt.Y + $cr.Bottom
        } elseif ([CasebookWin32]::DwmGetWindowAttribute($hwnd, 9, [ref]$fr, $size) -ne 0) { $fr = $wr }
        $w = $wr.Right - $wr.Left
        $h = $wr.Bottom - $wr.Top
        $bmp = New-Object System.Drawing.Bitmap $w, $h
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        $hdc = $g.GetHdc()
        $ok = [CasebookWin32]::PrintWindow($hwnd, $hdc, 2)   # PW_RENDERFULLCONTENT
        $g.ReleaseHdc($hdc)
        $g.Dispose()
        if (-not $ok) { $bmp.Dispose(); Write-Output 'win-ui: PrintWindow failed'; Done 1 }

        $crop = New-Object System.Drawing.Rectangle ($fr.Left - $wr.Left + $CropLeft), ($fr.Top - $wr.Top), ($fr.Right - $fr.Left - $CropLeft), ($fr.Bottom - $fr.Top)
        $shot = $bmp.Clone($crop, $bmp.PixelFormat)
        $bmp.Dispose()
        $dir = Split-Path -Parent $Out
        if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }
        $shot.Save($Out, [System.Drawing.Imaging.ImageFormat]::Png)
        Write-Output "win-ui: captured $($shot.Width)x$($shot.Height) to $Out"
        $shot.Dispose()
        Done 0
    }
}
