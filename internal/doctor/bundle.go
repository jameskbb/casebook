package doctor

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jameskbb/casebook/internal/version"
)

// maxBundleScan bounds how much of the desktop executable bundleVersionCheck
// reads. A release Casebook.exe is a few tens of MB; the marker sits in its
// read-only data, well inside this limit.
const maxBundleScan = 256 << 20

// bundleChunk is the read size of the streaming marker scan.
const bundleChunk = 1 << 20

// maxStampLen bounds the version text after the marker prefix.
const maxStampLen = 64

// desktopBeside returns the desktop app executable the packaged layout puts
// at dir(dir(CLI)) (ADR-0012): <folder>/Casebook.exe next to
// <folder>/bin/casebook.exe on Windows (portable zip and installer alike),
// Casebook.app/Contents/MacOS/Casebook next to .../MacOS/bin/casebook on
// macOS. It returns "" when there is no such regular file, or when it is the
// CLI itself (a case-insensitive volume).
func desktopBeside(o Options) string {
	if o.CLIPath == "" {
		return ""
	}
	goos := o.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	name := "Casebook"
	if goos == "windows" {
		name += ".exe"
	}
	desktop := filepath.Join(filepath.Dir(filepath.Dir(o.CLIPath)), name)
	fi, err := os.Stat(desktop)
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	if cli, err := os.Stat(o.CLIPath); err == nil && os.SameFile(fi, cli) {
		return ""
	}
	return desktop
}

// bundleVersionCheck compares the version stamped into the desktop app next
// to this CLI (desktopBeside) with the CLI's own. Both executables of one
// release carry the same "casebook-version=<version>" marker text
// (internal/version), so a difference means the folder was only partly
// replaced during an update. It skips for an unstamped (dev) CLI, when no
// desktop app sits next to it, and when the app carries no marker (a
// development build).
func bundleVersionCheck(o Options) Check {
	if o.Version == "" || o.Version == "dev" {
		return check(idBundleVersion, StatusSkip, "development build of the CLI")
	}
	desktop := desktopBeside(o)
	if desktop == "" {
		return check(idBundleVersion, StatusSkip, "no desktop app next to the CLI")
	}
	match, other, err := scanStamp(desktop, o.Version)
	switch {
	case err != nil:
		return check(idBundleVersion, StatusWarn, "cannot read %s: %v", desktop, osReason(err))
	case match:
		return check(idBundleVersion, StatusOK, "desktop app and CLI are both %s", o.Version)
	case other != "":
		return check(idBundleVersion, StatusWarn,
			"the desktop app is %s, this CLI is %s: replace the whole folder when you update (%s)", other, o.Version, desktop)
	}
	return check(idBundleVersion, StatusSkip, "%s carries no version stamp (a development build)", desktop)
}

// scanStamp streams at most maxBundleScan bytes of the file at path looking
// for version.MarkerPrefix followed by a version. match is true when the
// version equals want. Otherwise other is the first other stamped version
// found, or "" when there is none. The prefix also occurs as a plain string
// constant followed by unrelated text, so a version only counts when it is
// followed by a NUL byte, as a stamped value is.
func scanStamp(path, want string) (match bool, other string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	defer f.Close()
	prefix := []byte(version.MarkerPrefix)
	keep := len(prefix) + maxStampLen + 1
	r := io.LimitReader(f, maxBundleScan)
	buf := make([]byte, 0, bundleChunk+keep)
	chunk := make([]byte, bundleChunk)
	for {
		n, rerr := io.ReadFull(r, chunk)
		buf = append(buf, chunk[:n]...)
		eof := errors.Is(rerr, io.EOF) || errors.Is(rerr, io.ErrUnexpectedEOF)
		if rerr != nil && !eof {
			return false, "", rerr
		}
		// Scan every prefix whose version fits in buf, or all of them at
		// the end of the input.
		limit := len(buf) - keep
		if eof {
			limit = len(buf)
		}
		i := 0
		for {
			j := bytes.Index(buf[i:], prefix)
			if j < 0 || i+j >= limit {
				break
			}
			start := i + j + len(prefix)
			if v, ok := stampAt(buf[start:]); ok {
				if v == want {
					return true, "", nil
				}
				if other == "" {
					other = v
				}
			}
			i = start
		}
		if eof {
			return false, other, nil
		}
		// Carry the tail over so a marker split across chunks is found.
		if limit < 0 {
			limit = 0
		}
		buf = append(buf[:0], buf[limit:]...)
	}
}

// stampAt reads a version at the start of b: 1 to maxStampLen bytes of
// letters, digits and . + - _ followed by a NUL byte.
func stampAt(b []byte) (string, bool) {
	for i := 0; i < len(b) && i <= maxStampLen; i++ {
		c := b[i]
		switch {
		case c == 0:
			return string(b[:i]), i > 0
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '.', c == '+', c == '-', c == '_':
		default:
			return "", false
		}
	}
	return "", false
}
