package main

import (
	"net/url"
	"path"
	"strings"

	"github.com/jameskbb/casebook/internal/update"
	"github.com/jameskbb/casebook/internal/version"
)

// Check for updates (ADR 0022). The person starts every check from the Data
// menu: nothing here runs at startup or on a timer, and nothing is
// downloaded or installed. internal/update makes the one request to GitHub;
// the page's Content-Security-Policy stays connect-src 'self'.
//
// Like RestoreBackup (D-07), OpenReleasePage takes nothing from JavaScript:
// it opens the release page pinned by the last successful check, and only
// when that page belongs to the Casebook repository.

// pinnedRelease is what the last successful check found: the release page
// OpenReleasePage opens. It is empty before a check and after a failed one.
// The download link is not pinned: the page copies it from the result with
// CopyText.
type pinnedRelease struct {
	pageURL string
}

// CheckForUpdates asks GitHub for the latest release and compares it with
// this build. It needs no open database, so it works on a failed startup
// too. internal/update's DefaultTimeout bounds the request. Errors are kind
// "other" with a message safe to show as is.
func (d *Desktop) CheckForUpdates() (update.Result, error) {
	check := d.checkUpdate
	if check == nil {
		check = update.Check
	}
	res, err := check(d.ctx, update.Options{Current: version.Version, Flavor: update.FlavorDesktop})
	if err != nil {
		d.setLastRelease(pinnedRelease{})
		return update.Result{}, updateErr(err)
	}
	d.setLastRelease(pinnedRelease{pageURL: res.ReleaseURL})
	return res, nil
}

// OpenReleasePage opens the release page found by the last successful
// CheckForUpdates in the default browser. JavaScript passes no URL.
func (d *Desktop) OpenReleasePage() error {
	d.mu.Lock()
	page := d.lastRelease.pageURL
	d.mu.Unlock()
	if page == "" {
		return uiErr(kindOther, "", "Check for updates first")
	}
	if !isReleasePage(page) {
		return uiErr(kindOther, "", "Casebook only opens release pages on github.com/"+update.Repo)
	}
	return nativeErr("browser", d.native.OpenURL(page))
}

// isReleasePage reports whether raw is an https page of the Casebook
// repository on github.com: no user info, no other host, and a path that
// stays under the repository once dot segments are resolved. GitHub
// supplies the value, but this is the only address the app ever opens.
func isReleasePage(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host != "github.com" {
		return false
	}
	return strings.HasPrefix(path.Clean("/"+u.Path)+"/", "/"+update.Repo+"/")
}

func (d *Desktop) setLastRelease(p pinnedRelease) {
	d.mu.Lock()
	d.lastRelease = p
	d.mu.Unlock()
}

// updateErr reports a failed check as kind "other". internal/update writes
// every error for a person to read, so the message passes through.
func updateErr(err error) error {
	return uiErr(kindOther, "", err.Error())
}
