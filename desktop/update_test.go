package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/update"
	"github.com/jameskbb/casebook/internal/version"
)

// fakeCheck is an update.Check stand-in that records the options it got and
// answers with res and err, so no test reaches the network.
type fakeCheck struct {
	res   update.Result
	err   error
	calls []update.Options
}

func (f *fakeCheck) check(_ context.Context, o update.Options) (update.Result, error) {
	f.calls = append(f.calls, o)
	return f.res, f.err
}

// releaseResult is a successful check that found v9.9.9.
func releaseResult(state update.State) update.Result {
	return update.Result{
		Current:     "v0.4.0",
		Latest:      "v9.9.9",
		State:       state,
		ReleaseURL:  "https://github.com/jameskbb/casebook/releases/tag/v9.9.9",
		PublishedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		Asset: &update.Asset{
			Name:      "Casebook-v9.9.9-windows-amd64-portable.zip",
			URL:       "https://github.com/jameskbb/casebook/releases/download/v9.9.9/Casebook-v9.9.9-windows-amd64-portable.zip",
			SizeBytes: 12345,
		},
		CheckedAt: time.Date(2026, 10, 2, 14, 30, 0, 0, time.UTC),
	}
}

func newUpdateDesktop(t *testing.T, f *fakeCheck) (*Desktop, *fakeNative) {
	t.Helper()
	d, fake := newTestDesktop(t)
	d.checkUpdate = f.check
	return d, fake
}

// TestCheckForUpdatesPassesResultThrough: every state reaches the page
// unchanged, and the check is asked for the desktop download of this build.
func TestCheckForUpdatesPassesResultThrough(t *testing.T) {
	for _, st := range []update.State{update.StateUpdateAvailable, update.StateUpToDate, update.StateAhead, update.StateUnknown} {
		t.Run(string(st), func(t *testing.T) {
			f := &fakeCheck{res: releaseResult(st)}
			d, _ := newUpdateDesktop(t, f)
			got, err := d.CheckForUpdates()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, f.res) {
				t.Fatalf("result = %+v, want %+v", got, f.res)
			}
			if len(f.calls) != 1 {
				t.Fatalf("check ran %d times, want 1", len(f.calls))
			}
			o := f.calls[0]
			if o.Flavor != update.FlavorDesktop || o.Current != version.Version {
				t.Fatalf("options = %+v, want flavor desktop and current %q", o, version.Version)
			}
		})
	}
}

// TestCheckForUpdatesError: a failed check is kind other with the update
// package's message, and it clears any earlier pin.
func TestCheckForUpdatesError(t *testing.T) {
	f := &fakeCheck{res: releaseResult(update.StateUpdateAvailable)}
	d, fake := newUpdateDesktop(t, f)
	if _, err := d.CheckForUpdates(); err != nil {
		t.Fatal(err)
	}
	msg := "GitHub lists no release for jameskbb/casebook: the repository may be private, or no release is published yet"
	f.res, f.err = update.Result{}, fmt.Errorf("%w: the repository may be private, or no release is published yet", update.ErrNoRelease)
	_, err := d.CheckForUpdates()
	ue := decodeUIError(t, err)
	if ue.Kind != kindOther || ue.Message != msg || ue.Field != "" {
		t.Fatalf("error = %+v, want kind other with %q", ue, msg)
	}
	ue = decodeUIError(t, d.OpenReleasePage())
	if ue.Kind != kindOther || ue.Message != "Check for updates first" {
		t.Fatalf("OpenReleasePage after a failed check = %+v", ue)
	}
	if len(fake.urls) != 0 {
		t.Fatalf("opened %v after a failed check", fake.urls)
	}
}

// TestOpenReleasePage opens exactly the pinned page, refuses before any
// check, and refuses a page outside the Casebook repository.
func TestOpenReleasePage(t *testing.T) {
	f := &fakeCheck{res: releaseResult(update.StateUpdateAvailable)}
	d, fake := newUpdateDesktop(t, f)

	ue := decodeUIError(t, d.OpenReleasePage())
	if ue.Kind != kindOther || ue.Message != "Check for updates first" {
		t.Fatalf("OpenReleasePage before a check = %+v", ue)
	}

	if _, err := d.CheckForUpdates(); err != nil {
		t.Fatal(err)
	}
	if err := d.OpenReleasePage(); err != nil {
		t.Fatal(err)
	}
	if want := []string{f.res.ReleaseURL}; !reflect.DeepEqual(fake.urls, want) {
		t.Fatalf("opened %v, want %v", fake.urls, want)
	}

	fake.urlErr = errors.New("no browser")
	ue = decodeUIError(t, d.OpenReleasePage())
	if ue.Kind != kindOther || ue.Message != "no browser" {
		t.Fatalf("browser failure = %+v", ue)
	}
	fake.urlErr = nil

	for _, bad := range []string{
		"https://example.com/jameskbb/casebook/releases",
		"https://github.com/jameskbb/casebook/../../evil/releases",
		"https://github.com/jameskbb/casebook/..",
		"https://user@github.com/jameskbb/casebook/releases",
		"https://github.com:443@evil.example/jameskbb/casebook/releases",
		"https://github.com/jameskbb/casebook-evil/releases",
		"http://github.com/jameskbb/casebook/releases",
		"file:///C:/Windows/System32/calc.exe",
	} {
		f.res.ReleaseURL = bad
		if _, err := d.CheckForUpdates(); err != nil {
			t.Fatal(err)
		}
		fake.urls = nil
		ue := decodeUIError(t, d.OpenReleasePage())
		if ue.Kind != kindOther || ue.Message == "" {
			t.Fatalf("OpenReleasePage(%q) = %+v", bad, ue)
		}
		if len(fake.urls) != 0 {
			t.Fatalf("opened %v for %q", fake.urls, bad)
		}
	}
}

// TestCheckForUpdatesWithoutData: checking for updates works when the
// database failed to open, like the startup recovery bindings (AR-11).
func TestCheckForUpdatesWithoutData(t *testing.T) {
	fake := &fakeNative{}
	f := &fakeCheck{res: releaseResult(update.StateUpToDate)}
	d := &Desktop{ctx: context.Background(), native: fake, openErr: errors.New("disk on fire"), actor: app.ActorDesktop(), now: time.Now, checkUpdate: f.check}
	got, err := d.CheckForUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if got.State != update.StateUpToDate {
		t.Fatalf("state = %q", got.State)
	}
	if err := d.OpenReleasePage(); err != nil {
		t.Fatal(err)
	}
	if len(fake.urls) != 1 {
		t.Fatalf("opened %v", fake.urls)
	}
}
