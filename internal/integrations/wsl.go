package integrations

import (
	"path"
	"strings"
)

// Claude Code or Codex can run inside WSL on a Windows computer. Those
// installs keep their own settings under the Linux home folder, so the
// Windows `casebook setup` commands (which write the Windows-side settings)
// do not reach them. Instead the user registers the Windows casebook.exe with
// the client's own `mcp add` command, run from a WSL shell: WSL runs Windows
// programs through its /mnt/<drive> mounts, and the Windows program resolves
// the same per-user database as the desktop app.

// WSLPath returns the path WSL mounts a Windows drive path at, under the
// default automount root /mnt: C:\Users\Jo Doe\casebook.exe becomes
// /mnt/c/Users/Jo Doe/casebook.exe. The drive letter is lowercased and
// backslashes become slashes; a \\?\ long-path prefix is dropped. It reports
// false for anything else (UNC and \\wsl$ paths, relative paths, bare
// command names), which WSL cannot run through a drive mount.
func WSLPath(win string) (string, bool) {
	if strings.HasPrefix(win, `\\?\`) && !strings.HasPrefix(strings.ToUpper(win), `\\?\UNC\`) {
		win = win[len(`\\?\`):]
	}
	if len(win) < 3 || win[1] != ':' || (win[2] != '\\' && win[2] != '/') {
		return "", false
	}
	d := win[0]
	if !(d >= 'a' && d <= 'z' || d >= 'A' && d <= 'Z') {
		return "", false
	}
	rest := strings.ReplaceAll(win[2:], `\`, "/")
	return "/mnt/" + strings.ToLower(string(d)) + rest, true
}

// WSLDistroEnv is the variable WSL sets to the distribution's name inside
// it. WSL passes it on to a Windows program only when WSLENV lists it.
const WSLDistroEnv = "WSL_DISTRO_NAME"

// WindowsPathFromWSL returns the Windows path of an absolute Linux path p
// seen inside the WSL distribution distro, the inverse of WSLPath:
// /mnt/c/src/proj becomes C:\src\proj (any single drive letter under the
// default automount root /mnt), and any other absolute path becomes a
// path under the distribution's network share, \\<host>\<distro>\..., with
// host "wsl.localhost" when host is "". p is cleaned first. It reports
// false for a relative path, and for a path outside /mnt/<drive> when
// distro is empty or not a plain name.
func WindowsPathFromWSL(p, distro, host string) (string, bool) {
	if !strings.HasPrefix(p, "/") {
		return "", false
	}
	p = path.Clean(p)
	if rest, ok := strings.CutPrefix(p, "/mnt/"); ok {
		drive, tail, _ := strings.Cut(rest, "/")
		if len(drive) == 1 && (drive[0] >= 'a' && drive[0] <= 'z' || drive[0] >= 'A' && drive[0] <= 'Z') {
			return strings.ToUpper(drive) + `:\` + strings.ReplaceAll(tail, "/", `\`), true
		}
	}
	if !validDistro(distro) {
		return "", false
	}
	if host == "" {
		host = "wsl.localhost"
	}
	return `\\` + host + `\` + distro + strings.ReplaceAll(p, "/", `\`), true
}

// validDistro reports whether name can be a WSL distribution name in a
// network path: not empty, no path separators, no dot-only name.
func validDistro(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\:`)
}

// WSLDistro finds the WSL distribution a Windows program was started from:
// distroEnv (WSL_DISTRO_NAME, set when the user lists it in WSLENV), else
// the distribution share of wd, the program's working directory, which WSL
// sets to \\wsl.localhost\<distro>\... (or \\wsl$\<distro>\...) when the
// program starts in a Linux folder. host is the share's host name as wd
// spells it ("" from distroEnv). ok is false when neither names one.
func WSLDistro(distroEnv, wd string) (distro, host string, ok bool) {
	if validDistro(distroEnv) {
		return distroEnv, "", true
	}
	for _, h := range []string{"wsl.localhost", "wsl$"} {
		prefix := `\\` + h + `\`
		if len(wd) > len(prefix) && strings.EqualFold(wd[:len(prefix)], prefix) {
			name, _, _ := strings.Cut(wd[len(prefix):], `\`)
			if validDistro(name) {
				return name, wd[2 : 2+len(h)], true
			}
		}
	}
	return "", "", false
}

// HookSessionDir is the directory a hook applies the "marked" scope to. On
// Windows a Linux path in the payload's cwd (from a Claude Code running
// inside WSL whose hooks run the Windows casebook.exe) is translated with
// WindowsPathFromWSL, the distribution taken from WSLDistro(distroEnv, wd);
// it is "" when that fails, so the hooks stay silent. Anywhere else, and
// for any other cwd, it is cwd unchanged.
func HookSessionDir(cwd, goos, distroEnv, wd string) string {
	if goos != "windows" || !strings.HasPrefix(cwd, "/") {
		return cwd
	}
	distro, host, _ := WSLDistro(distroEnv, wd)
	win, ok := WindowsPathFromWSL(cwd, distro, host)
	if !ok {
		return ""
	}
	return win
}

// WSLSetupCommand is the one-line command a WSL shell (bash) runs to register
// the Windows CLI at cliPath with client ("claude" or "codex") through that
// client's own `mcp add`, quoted for bash. dbPath, when set, is passed with
// --db as written: the Windows program reads it, so it stays a Windows
// path. It is "" when cliPath has no WSL form (see WSLPath) or client is
// unknown.
func WSLSetupCommand(client, cliPath, dbPath string) string {
	wsl, ok := WSLPath(cliPath)
	if !ok {
		return ""
	}
	var b strings.Builder
	switch client {
	case "claude":
		b.WriteString("claude mcp add --scope user casebook -- ")
	case "codex":
		b.WriteString("codex mcp add casebook -- ")
	default:
		return ""
	}
	b.WriteString(QuotePOSIX(wsl))
	b.WriteString(" mcp")
	if dbPath != "" {
		b.WriteString(" --db ")
		b.WriteString(QuotePOSIX(dbPath))
	}
	return b.String()
}
