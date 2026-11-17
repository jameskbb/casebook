package main

import "testing"

// TestFileManagerIsAbsolute is S6: Open data folder runs the file manager by
// absolute path, so a directory early in PATH cannot swap it.
func TestFileManagerIsAbsolute(t *testing.T) {
	env := func(vals map[string]string) func(string) string {
		return func(k string) string { return vals[k] }
	}
	files := func(present ...string) func(string) bool {
		return func(p string) bool {
			for _, f := range present {
				if f == p {
					return true
				}
			}
			return false
		}
	}
	cases := []struct {
		name   string
		goos   string
		env    map[string]string
		exists []string
		want   string
	}{
		{"windows system root", "windows", map[string]string{"SystemRoot": `D:\WINNT`}, nil, `D:\WINNT\explorer.exe`},
		{"windows trailing slash", "windows", map[string]string{"SystemRoot": `C:\Windows\`}, nil, `C:\Windows\explorer.exe`},
		{"windows unset", "windows", nil, nil, `C:\Windows\explorer.exe`},
		{"windows relative root", "windows", map[string]string{"SystemRoot": `Windows`}, nil, `C:\Windows\explorer.exe`},
		{"windows unc root", "windows", map[string]string{"SystemRoot": `\\host\share`}, nil, `C:\Windows\explorer.exe`},
		{"darwin", "darwin", nil, nil, "/usr/bin/open"},
		{"linux usr bin", "linux", nil, []string{"/usr/bin/xdg-open", "/bin/xdg-open"}, "/usr/bin/xdg-open"},
		{"linux bin", "linux", nil, []string{"/bin/xdg-open"}, "/bin/xdg-open"},
		{"linux usr local", "freebsd", nil, []string{"/usr/local/bin/xdg-open"}, "/usr/local/bin/xdg-open"},
		{"linux elsewhere", "linux", nil, nil, "xdg-open"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fileManager(tc.goos, env(tc.env), files(tc.exists...)); got != tc.want {
				t.Fatalf("fileManager(%s) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}
}
