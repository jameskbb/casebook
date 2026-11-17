package integrations

import "testing"

func TestWSLPath(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{`C:\Users\Jo Doe\AppData\Local\Programs\Casebook\bin\casebook.exe`, "/mnt/c/Users/Jo Doe/AppData/Local/Programs/Casebook/bin/casebook.exe", true},
		{`D:\Tools\casebook.exe`, "/mnt/d/Tools/casebook.exe", true},
		{`e:/mixed\slashes/casebook.exe`, "/mnt/e/mixed/slashes/casebook.exe", true},
		{`C:\`, "/mnt/c/", true},
		{`\\?\C:\Very\Long\casebook.exe`, "/mnt/c/Very/Long/casebook.exe", true},
		{`\\?\UNC\server\share\casebook.exe`, "", false},
		{`\\server\share\casebook.exe`, "", false},
		{`\\wsl$\Ubuntu\home\jo\casebook`, "", false},
		{`C:relative\casebook.exe`, "", false},
		{`casebook`, "", false},
		{`1:\x`, "", false},
		{"", "", false},
		{"/usr/local/bin/casebook", "", false},
	}
	for _, c := range cases {
		got, ok := WSLPath(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("WSLPath(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestWindowsPathFromWSL(t *testing.T) {
	cases := []struct {
		in, distro, host, want string
		ok                     bool
	}{
		{"/mnt/c/src/proj", "", "", `C:\src\proj`, true},
		{"/mnt/c/src/proj", "Ubuntu", "", `C:\src\proj`, true},
		{"/mnt/d/Users/Jo Doe/x", "", "", `D:\Users\Jo Doe\x`, true},
		{"/mnt/C/x", "", "", `C:\x`, true},
		{"/mnt/c", "", "", `C:\`, true},
		{"/mnt/c/", "", "", `C:\`, true},
		{"/mnt/c/a/../b//c/", "", "", `C:\b\c`, true},
		{"/home/jo/proj", "Ubuntu", "", `\\wsl.localhost\Ubuntu\home\jo\proj`, true},
		{"/home/jo/proj", "Debian", "wsl$", `\\wsl$\Debian\home\jo\proj`, true},
		{"/", "Ubuntu-22.04", "", `\\wsl.localhost\Ubuntu-22.04\`, true},
		{"/mnt/wsl/x", "Ubuntu", "", `\\wsl.localhost\Ubuntu\mnt\wsl\x`, true},
		{"/mnt/cd/x", "Ubuntu", "", `\\wsl.localhost\Ubuntu\mnt\cd\x`, true},
		{"/mnt/1/x", "Ubuntu", "", `\\wsl.localhost\Ubuntu\mnt\1\x`, true},
		{"/mnt", "Ubuntu", "", `\\wsl.localhost\Ubuntu\mnt`, true},
		{"/mnt/c/../../etc", "Ubuntu", "", `\\wsl.localhost\Ubuntu\etc`, true},
		{"/home/jo/proj", "", "", "", false},
		{"/mnt/wsl/x", "", "", "", false},
		{"/home/jo", `a\b`, "", "", false},
		{"/home/jo", "a/b", "", "", false},
		{"/home/jo", "..", "", "", false},
		{"relative/dir", "Ubuntu", "", "", false},
		{`C:\src`, "Ubuntu", "", "", false},
		{"", "Ubuntu", "", "", false},
	}
	for _, c := range cases {
		got, ok := WindowsPathFromWSL(c.in, c.distro, c.host)
		if got != c.want || ok != c.ok {
			t.Errorf("WindowsPathFromWSL(%q, %q, %q) = %q, %v; want %q, %v", c.in, c.distro, c.host, got, ok, c.want, c.ok)
		}
	}
}

func TestWSLDistro(t *testing.T) {
	cases := []struct {
		env, wd, distro, host string
		ok                    bool
	}{
		{"Ubuntu", `C:\Windows`, "Ubuntu", "", true},
		{"Ubuntu", `\\wsl.localhost\Debian\home`, "Ubuntu", "", true},
		{"", `\\wsl.localhost\Debian\home\jo\proj`, "Debian", "wsl.localhost", true},
		{"", `\\WSL.LOCALHOST\Debian\home`, "Debian", "WSL.LOCALHOST", true},
		{"", `\\wsl$\Ubuntu-22.04\home\jo`, "Ubuntu-22.04", "wsl$", true},
		{"", `\\wsl.localhost\Debian`, "Debian", "wsl.localhost", true},
		{"", `\\wsl.localhost\`, "", "", false},
		{"", `\\server\share\x`, "", "", false},
		{"", `C:\src\proj`, "", "", false},
		{"", "", "", "", false},
		{`bad\name`, `C:\x`, "", "", false},
	}
	for _, c := range cases {
		distro, host, ok := WSLDistro(c.env, c.wd)
		if distro != c.distro || host != c.host || ok != c.ok {
			t.Errorf("WSLDistro(%q, %q) = %q, %q, %v; want %q, %q, %v", c.env, c.wd, distro, host, ok, c.distro, c.host, c.ok)
		}
	}
}

func TestHookSessionDir(t *testing.T) {
	const unc = `\\wsl.localhost\Debian\home\jo\proj`
	cases := []struct {
		cwd, goos, env, wd, want string
	}{
		// Off Windows, and for Windows paths, cwd is used as given.
		{"/home/jo/proj", "linux", "", "", "/home/jo/proj"},
		{"/mnt/c/src", "darwin", "Ubuntu", "", "/mnt/c/src"},
		{`C:\src\proj`, "windows", "", "", `C:\src\proj`},
		{"", "windows", "", unc, ""},
		{"relative", "windows", "", "", "relative"},
		// A Linux cwd on Windows is translated.
		{"/mnt/c/src/proj", "windows", "", `C:\Windows`, `C:\src\proj`},
		{"/home/jo/proj", "windows", "Ubuntu", `C:\Windows`, `\\wsl.localhost\Ubuntu\home\jo\proj`},
		{"/home/jo/proj/sub", "windows", "", unc, `\\wsl.localhost\Debian\home\jo\proj\sub`},
		{"/home/jo/proj", "windows", "", `\\wsl$\Debian\home`, `\\wsl$\Debian\home\jo\proj`},
		// No distribution: "" (the hooks stay silent).
		{"/home/jo/proj", "windows", "", `C:\Windows`, ""},
	}
	for _, c := range cases {
		if got := HookSessionDir(c.cwd, c.goos, c.env, c.wd); got != c.want {
			t.Errorf("HookSessionDir(%q, %q, %q, %q) = %q, want %q", c.cwd, c.goos, c.env, c.wd, got, c.want)
		}
	}
}

func TestWSLSetupCommand(t *testing.T) {
	const spaced = `C:\Users\Jo Doe\AppData\Local\Programs\Casebook\bin\casebook.exe`
	cases := []struct{ client, cli, db, want string }{
		{"claude", spaced, "", `claude mcp add --scope user casebook -- '/mnt/c/Users/Jo Doe/AppData/Local/Programs/Casebook/bin/casebook.exe' mcp`},
		{"codex", spaced, "", `codex mcp add casebook -- '/mnt/c/Users/Jo Doe/AppData/Local/Programs/Casebook/bin/casebook.exe' mcp`},
		{"claude", `C:\Casebook\bin\casebook.exe`, "", `claude mcp add --scope user casebook -- /mnt/c/Casebook/bin/casebook.exe mcp`},
		{"claude", `C:\Users\O'Neil\Casebook\bin\casebook.exe`, "", `claude mcp add --scope user casebook -- '/mnt/c/Users/O'\''Neil/Casebook/bin/casebook.exe' mcp`},
		{"codex", `C:\Casebook\bin\casebook.exe`, `D:\My Cases\casebook.db`, `codex mcp add casebook -- /mnt/c/Casebook/bin/casebook.exe mcp --db 'D:\My Cases\casebook.db'`},
		{"claude", `C:\Program Files (x86)\Casebook\bin\casebook.exe`, "", `claude mcp add --scope user casebook -- '/mnt/c/Program Files (x86)/Casebook/bin/casebook.exe' mcp`},
		// bash expands nothing inside single quotes; % means nothing to bash.
		{"claude", `C:\Users\$USER\a` + "`id`" + `\casebook.exe`, `C:\%TEMP%\$x.db`, "claude mcp add --scope user casebook -- '/mnt/c/Users/$USER/a`id`/casebook.exe' mcp --db 'C:\\%TEMP%\\$x.db'"},
		{"codex", `C:\Users\100%user\casebook.exe`, "", `codex mcp add casebook -- /mnt/c/Users/100%user/casebook.exe mcp`},
		{"claude", `\\server\share\casebook.exe`, "", ""},
		{"claude", "casebook", "", ""},
		{"other", spaced, "", ""},
	}
	for _, c := range cases {
		if got := WSLSetupCommand(c.client, c.cli, c.db); got != c.want {
			t.Errorf("WSLSetupCommand(%q, %q, %q) =\n  %s\nwant\n  %s", c.client, c.cli, c.db, got, c.want)
		}
	}
}
