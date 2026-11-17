package update_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jameskbb/casebook/internal/update"
)

// release is a minimal GitHub "latest release" answer for v0.5.0 with every
// asset the release workflow publishes that a check can point at.
const release = `{
  "tag_name": "v0.5.0",
  "html_url": "https://github.com/jameskbb/casebook/releases/tag/v0.5.0",
  "published_at": "2026-10-01T12:00:00Z",
  "draft": false,
  "prerelease": false,
  "body": "ignored",
  "assets": [
    {"name": "Casebook-v0.5.0-windows-amd64-portable.zip", "browser_download_url": "https://github.com/jameskbb/casebook/releases/download/v0.5.0/win-amd64.zip", "size": 111},
    {"name": "Casebook-v0.5.0-windows-arm64-portable.zip", "browser_download_url": "https://github.com/jameskbb/casebook/releases/download/v0.5.0/win-arm64.zip", "size": 112},
    {"name": "Casebook-v0.5.0-macos-universal.dmg", "browser_download_url": "https://github.com/jameskbb/casebook/releases/download/v0.5.0/mac.dmg", "size": 222},
    {"name": "casebook-cli-v0.5.0-linux-arm64.tar.gz", "browser_download_url": "https://github.com/jameskbb/casebook/releases/download/v0.5.0/linux-arm64.tgz", "size": 333},
    {"name": "casebook-cli-v0.5.0-windows-amd64.zip", "browser_download_url": "https://github.com/jameskbb/casebook/releases/download/v0.5.0/cli-win.zip", "size": 444}
  ]
}`

var fixedNow = time.Date(2026, 10, 14, 9, 30, 0, 0, time.FixedZone("x", 2*3600))

// server answers every request with status, headers and body, counting
// requests and recording the last one.
type server struct {
	*httptest.Server
	count atomic.Int32
	last  atomic.Pointer[http.Request]
}

func newServer(t *testing.T, status int, header map[string]string, body string) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.count.Add(1)
		s.last.Store(r.Clone(context.Background()))
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func opts(s *server, current string, flavor update.Flavor, goos, goarch string) update.Options {
	return update.Options{
		Current: current, Flavor: flavor, GOOS: goos, GOARCH: goarch,
		BaseURL: s.URL, Client: s.Client(), Now: func() time.Time { return fixedNow },
	}
}

func TestCheckRequest(t *testing.T) {
	s := newServer(t, 200, nil, release)
	if _, err := update.Check(context.Background(), opts(s, "v0.5.0", update.FlavorCLI, "linux", "amd64")); err != nil {
		t.Fatal(err)
	}
	if n := s.count.Load(); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
	r := s.last.Load()
	if r.Method != http.MethodGet {
		t.Errorf("method = %s", r.Method)
	}
	if r.URL.Path != "/repos/jameskbb/casebook/releases/latest" || r.URL.RawQuery != "" {
		t.Errorf("path = %s?%s", r.URL.Path, r.URL.RawQuery)
	}
	want := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-Github-Api-Version": "2022-11-28",
		"User-Agent":           update.UserAgent,
	}
	for k, v := range want {
		if got := r.Header.Values(k); len(got) != 1 || got[0] != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
	if r.ContentLength > 0 {
		t.Errorf("request has a body of %d bytes", r.ContentLength)
	}
}

func TestCheckStates(t *testing.T) {
	cases := []struct {
		current string
		want    update.State
	}{
		{"v0.5.0", update.StateUpToDate},
		{"v0.4.0", update.StateUpdateAvailable},
		{"v0.5.0-rc.1", update.StateUpdateAvailable},
		{"v0.5.1", update.StateAhead},
		{"v0.5.1-0.20261013120000-abcdef123456", update.StateAhead},
		{"dev", update.StateUnknown},
		{"(devel)", update.StateUnknown},
	}
	for _, c := range cases {
		s := newServer(t, 200, nil, release)
		res, err := update.Check(context.Background(), opts(s, c.current, update.FlavorCLI, "linux", "amd64"))
		if err != nil {
			t.Fatalf("%s: %v", c.current, err)
		}
		if res.State != c.want {
			t.Errorf("%s: state %s, want %s", c.current, res.State, c.want)
		}
		if res.Current != c.current || res.Latest != "v0.5.0" {
			t.Errorf("%s: current %q latest %q", c.current, res.Current, res.Latest)
		}
		if res.ReleaseURL != "https://github.com/jameskbb/casebook/releases/tag/v0.5.0" {
			t.Errorf("release url %q", res.ReleaseURL)
		}
		if !res.PublishedAt.Equal(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)) {
			t.Errorf("published %v", res.PublishedAt)
		}
		if !res.CheckedAt.Equal(fixedNow) || res.CheckedAt.Location() != time.UTC {
			t.Errorf("checked %v, want %v in UTC", res.CheckedAt, fixedNow)
		}
	}
}

func TestCheckAsset(t *testing.T) {
	cases := []struct {
		flavor       update.Flavor
		goos, goarch string
		want         *update.Asset
	}{
		{update.FlavorDesktop, "windows", "amd64", &update.Asset{Name: "Casebook-v0.5.0-windows-amd64-portable.zip", URL: "https://github.com/jameskbb/casebook/releases/download/v0.5.0/win-amd64.zip", SizeBytes: 111}},
		{update.FlavorDesktop, "windows", "arm64", &update.Asset{Name: "Casebook-v0.5.0-windows-arm64-portable.zip", URL: "https://github.com/jameskbb/casebook/releases/download/v0.5.0/win-arm64.zip", SizeBytes: 112}},
		{update.FlavorDesktop, "darwin", "arm64", &update.Asset{Name: "Casebook-v0.5.0-macos-universal.dmg", URL: "https://github.com/jameskbb/casebook/releases/download/v0.5.0/mac.dmg", SizeBytes: 222}},
		{update.FlavorDesktop, "linux", "amd64", nil},
		{update.FlavorCLI, "linux", "arm64", &update.Asset{Name: "casebook-cli-v0.5.0-linux-arm64.tar.gz", URL: "https://github.com/jameskbb/casebook/releases/download/v0.5.0/linux-arm64.tgz", SizeBytes: 333}},
		{update.FlavorCLI, "windows", "amd64", &update.Asset{Name: "casebook-cli-v0.5.0-windows-amd64.zip", URL: "https://github.com/jameskbb/casebook/releases/download/v0.5.0/cli-win.zip", SizeBytes: 444}},
		// The fixture has no darwin CLI archive: a missing asset is nil, not an error.
		{update.FlavorCLI, "darwin", "arm64", nil},
	}
	for _, c := range cases {
		s := newServer(t, 200, nil, release)
		res, err := update.Check(context.Background(), opts(s, "v0.4.0", c.flavor, c.goos, c.goarch))
		if err != nil {
			t.Fatalf("%s %s/%s: %v", c.flavor, c.goos, c.goarch, err)
		}
		if res.State != update.StateUpdateAvailable {
			t.Errorf("%s %s/%s: state %s", c.flavor, c.goos, c.goarch, res.State)
		}
		switch {
		case c.want == nil && res.Asset != nil:
			t.Errorf("%s %s/%s: asset %+v, want nil", c.flavor, c.goos, c.goarch, *res.Asset)
		case c.want != nil && (res.Asset == nil || *res.Asset != *c.want):
			t.Errorf("%s %s/%s: asset %+v, want %+v", c.flavor, c.goos, c.goarch, res.Asset, *c.want)
		}
	}
}

func TestCheckAssetOffGitHubIsDropped(t *testing.T) {
	body := strings.Replace(release,
		"https://github.com/jameskbb/casebook/releases/download/v0.5.0/win-amd64.zip",
		"https://example.test/win-amd64.zip", 1)
	s := newServer(t, 200, nil, body)
	res, err := update.Check(context.Background(), opts(s, "v0.4.0", update.FlavorDesktop, "windows", "amd64"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Asset != nil {
		t.Errorf("asset %+v, want nil for a download link off github.com", *res.Asset)
	}
	if res.State != update.StateUpdateAvailable || res.Latest != "v0.5.0" {
		t.Errorf("state %s latest %s: the rest of the result must stand", res.State, res.Latest)
	}
}

func TestCheckDefaultsGOOSAndGOARCH(t *testing.T) {
	s := newServer(t, 200, nil, release)
	o := opts(s, "v0.5.0", update.FlavorCLI, "", "")
	if _, err := update.Check(context.Background(), o); err != nil {
		t.Fatal(err)
	}
}

func TestCheckLenientFields(t *testing.T) {
	// No html_url and an unparseable published_at: the check still succeeds.
	s := newServer(t, 200, nil, `{"tag_name":"v0.5.0","published_at":"yesterday"}`)
	res, err := update.Check(context.Background(), opts(s, "v0.5.0", update.FlavorCLI, "linux", "amd64"))
	if err != nil {
		t.Fatal(err)
	}
	if res.ReleaseURL != update.ReleasesURL+"/tag/v0.5.0" {
		t.Errorf("release url %q", res.ReleaseURL)
	}
	if !res.PublishedAt.IsZero() || res.Asset != nil {
		t.Errorf("published %v asset %v", res.PublishedAt, res.Asset)
	}
}

func TestCheckInvalidOptions(t *testing.T) {
	s := newServer(t, 200, nil, release)
	for _, o := range []update.Options{
		opts(s, "", update.FlavorCLI, "linux", "amd64"),
		opts(s, "v0.4.0", "", "linux", "amd64"),
		opts(s, "v0.4.0", "server", "linux", "amd64"),
	} {
		_, err := update.Check(context.Background(), o)
		if err == nil {
			t.Errorf("%+v: no error", o)
			continue
		}
		for _, sentinel := range []error{update.ErrUnavailable, update.ErrNoRelease, update.ErrRateLimited} {
			if errors.Is(err, sentinel) {
				t.Errorf("%+v: %v wraps %v", o, err, sentinel)
			}
		}
	}
	if n := s.count.Load(); n != 0 {
		t.Errorf("invalid options made %d requests", n)
	}
}

func TestCheckErrors(t *testing.T) {
	big := `{"tag_name":"v0.5.0","body":"` + strings.Repeat("x", 1<<20) + `"}`
	cases := []struct {
		name     string
		status   int
		header   map[string]string
		body     string
		sentinel error
		text     []string
	}{
		{"not found", 404, nil, `{"message":"Not Found"}`, update.ErrNoRelease,
			[]string{"GitHub lists no release for jameskbb/casebook", "the repository may be private, or no release is published yet"}},
		{"forbidden", 403, nil, `{"message":"API rate limit exceeded for 1.2.3.4."}`, update.ErrRateLimited,
			[]string{"rate limiting", "API rate limit exceeded"}},
		{"forbidden no json", 403, nil, `nope`, update.ErrRateLimited, []string{"rate limiting"}},
		{"too many", 429, map[string]string{"Retry-After": "120"}, ``, update.ErrRateLimited,
			[]string{"rate limiting", "try again in 2 minutes"}},
		{"too many reset", 429, map[string]string{"X-RateLimit-Reset": "1792000800"}, ``, update.ErrRateLimited,
			[]string{"rate limiting", "try again after"}},
		{"server error", 500, nil, `oops`, update.ErrUnavailable, []string{"GitHub answered 500 Internal Server Error instead of a release; try again later"}},
		{"bad gateway", 502, nil, ``, update.ErrUnavailable, []string{"502 Bad Gateway"}},
		// A redirect is reported, never followed: one request per check.
		{"redirect", 301, map[string]string{"Location": "/elsewhere"}, ``, update.ErrUnavailable, []string{"301 Moved Permanently"}},
		{"not modified", 304, nil, ``, update.ErrUnavailable, []string{"304 Not Modified"}},
		{"forbidden escape", 403, nil, `{"message":"slow\u001b[31m down\n now"}`, update.ErrRateLimited,
			[]string{"(GitHub says: slow[31m down now)"}},
		{"too large", 200, nil, big, update.ErrUnavailable, []string{"GitHub's answer was too large"}},
		{"malformed", 200, nil, `{"tag_name":`, update.ErrUnavailable, []string{"GitHub's answer was not a release"}},
		{"html", 200, nil, `<html>hello</html>`, update.ErrUnavailable, []string{"not a release"}},
		{"no tag", 200, nil, `{"html_url":"x"}`, update.ErrUnavailable, []string{"not a release"}},
		{"no v", 200, nil, `{"tag_name":"0.5.0"}`, update.ErrUnavailable, []string{"not a release"}},
		{"draft", 200, nil, `{"tag_name":"v0.5.0","draft":true}`, update.ErrUnavailable, []string{"not a release"}},
		{"prerelease", 200, nil, `{"tag_name":"v0.5.0-rc.1","prerelease":true}`, update.ErrUnavailable, []string{"not a release"}},
		{"null", 200, nil, `null`, update.ErrUnavailable, []string{"not a release"}},
		{"array", 200, nil, `[]`, update.ErrUnavailable, []string{"not a release"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newServer(t, c.status, c.header, c.body)
			res, err := update.Check(context.Background(), opts(s, "v0.4.0", update.FlavorCLI, "linux", "amd64"))
			if err == nil {
				t.Fatalf("no error; result %+v", res)
			}
			if !errors.Is(err, c.sentinel) {
				t.Errorf("error %q does not wrap %v", err, c.sentinel)
			}
			for _, w := range c.text {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
			if n := s.count.Load(); n != 1 {
				t.Errorf("requests = %d, want 1", n)
			}
		})
	}
}

func TestCheckExactlyOneMiB(t *testing.T) {
	// A body of exactly 1 MiB is read in full and is not "too large".
	body := `{"tag_name":"v0.5.0","body":"`
	body += strings.Repeat("x", 1<<20-len(body)-2) + `"}`
	if len(body) != 1<<20 {
		t.Fatalf("body is %d bytes", len(body))
	}
	s := newServer(t, 200, nil, body)
	if _, err := update.Check(context.Background(), opts(s, "v0.5.0", update.FlavorCLI, "linux", "amd64")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckConnectionRefused(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	_, err = update.Check(context.Background(), update.Options{
		Current: "v0.4.0", Flavor: update.FlavorCLI, BaseURL: "http://" + addr,
	})
	if !errors.Is(err, update.ErrUnavailable) {
		t.Fatalf("error %v does not wrap ErrUnavailable", err)
	}
	if !strings.HasPrefix(err.Error(), "could not reach GitHub: ") {
		t.Errorf("error %q", err)
	}
}

func TestCheckContextTimeout(t *testing.T) {
	release := make(chan struct{})
	var count atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(s.Close)
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := update.Check(ctx, update.Options{
		Current: "v0.4.0", Flavor: update.FlavorDesktop, BaseURL: s.URL, Client: s.Client(),
	})
	if !errors.Is(err, update.ErrUnavailable) {
		t.Errorf("error %v does not wrap ErrUnavailable", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %v does not wrap context.DeadlineExceeded", err)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestCheckCancelled(t *testing.T) {
	s := newServer(t, 200, nil, release)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := update.Check(ctx, opts(s, "v0.4.0", update.FlavorCLI, "linux", "amd64"))
	if !errors.Is(err, update.ErrUnavailable) || !errors.Is(err, context.Canceled) {
		t.Errorf("error %v", err)
	}
}

func TestCheckReportsAssetURLWithoutFetching(t *testing.T) {
	// The asset URL is reported, never fetched: a release with a download
	// link still produces exactly one request, to the release endpoint.
	const link = "https://github.com/jameskbb/casebook/releases/download/v0.5.0/casebook-cli-v0.5.0-linux-amd64.tar.gz"
	s := newServer(t, 200, nil, `{"tag_name":"v0.5.0","assets":[{"name":"casebook-cli-v0.5.0-linux-amd64.tar.gz","browser_download_url":"`+link+`","size":1}]}`)
	res, err := update.Check(context.Background(), opts(s, "v0.4.0", update.FlavorCLI, "linux", "amd64"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Asset == nil || res.Asset.URL != link {
		t.Errorf("asset %+v", res.Asset)
	}
	if got := s.last.Load(); got == nil || got.URL.Path != "/repos/jameskbb/casebook/releases/latest" {
		t.Errorf("request path %v, want the release endpoint", got)
	}
	if n := s.count.Load(); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestCheckNotFoundMessage(t *testing.T) {
	// This is the message a person sees from a live check when GitHub lists
	// no release (a private repository or one with no published release); the
	// CLI and the desktop app print it as is.
	s := newServer(t, 404, nil, `{"message":"Not Found"}`)
	_, err := update.Check(context.Background(), opts(s, "v0.4.0", update.FlavorDesktop, "windows", "amd64"))
	want := "GitHub lists no release for jameskbb/casebook: the repository may be private, or no release is published yet"
	if err == nil || err.Error() != want {
		t.Errorf("error %v, want %q", err, want)
	}
}

func TestCheckLeavesCallerClientAlone(t *testing.T) {
	s := newServer(t, 200, nil, release)
	c := s.Client()
	o := opts(s, "v0.5.0", update.FlavorCLI, "linux", "amd64")
	o.Client = c
	if _, err := update.Check(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.CheckRedirect != nil {
		t.Error("Check changed the caller's client")
	}
}
