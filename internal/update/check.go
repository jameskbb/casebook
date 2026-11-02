package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// maxBody bounds how much of GitHub's answer is read. A real "latest
// release" answer is a few kilobytes; anything over 1 MiB is not one.
const maxBody = 1 << 20

// latestPath is the GitHub REST endpoint for the latest published release.
const latestPath = "/repos/" + Repo + "/releases/latest"

// checkError is what Check returns for every failure after the options are
// valid. Its message is written for a person; it unwraps to the sentinel and,
// for a transport failure, to the underlying error too, so errors.Is finds
// context.DeadlineExceeded through it.
type checkError struct {
	msg  string
	errs []error
}

func (e *checkError) Error() string   { return e.msg }
func (e *checkError) Unwrap() []error { return e.errs }

func fail(sentinel error, msg string, cause ...error) error {
	return &checkError{msg: msg, errs: append([]error{sentinel}, cause...)}
}

// assetURLPrefix is where every release file of Repo is served from. A
// download link that starts elsewhere is not reported.
const assetURLPrefix = "https://github.com/" + Repo + "/releases/download/"

// unreachable is the error for a transport failure: one sentence for a
// person (Go's dial and DNS text is kept in the chain for errors.Is and
// logs, not shown), with a note when the request timed out.
func unreachable(cause error) error {
	msg := "could not reach GitHub: check your network connection and try again"
	var ue *url.Error
	if errors.Is(cause, context.DeadlineExceeded) || (errors.As(cause, &ue) && ue.Timeout()) {
		msg += " (the request timed out)"
	}
	return fail(ErrUnavailable, msg, cause)
}

// notRelease is the error for a 200 answer that does not describe a
// published release.
func notRelease(why string) error {
	return fail(ErrUnavailable, "GitHub's answer was not a release: "+why)
}

// githubRelease holds the fields Check reads from GitHub's answer; every
// other field is ignored.
type githubRelease struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

func check(ctx context.Context, o Options) (Result, error) {
	if o.Current == "" {
		return Result{}, errors.New("update: Options.Current is empty")
	}
	if o.Flavor != FlavorDesktop && o.Flavor != FlavorCLI {
		return Result{}, fmt.Errorf("update: unknown flavor %q (want %q or %q)", o.Flavor, FlavorDesktop, FlavorCLI)
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}
	if o.BaseURL == "" {
		o.BaseURL = DefaultBaseURL
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	client := &http.Client{Timeout: DefaultTimeout}
	if o.Client != nil {
		c := *o.Client
		client = &c
	}
	// One request per check: a redirect is reported as the status it is,
	// never followed. The copy above keeps the caller's client unchanged.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(o.BaseURL, "/")+latestPath, nil)
	if err != nil {
		return Result{}, fmt.Errorf("update: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, unreachable(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return Result{}, unreachable(err)
	}
	tooLarge := len(body) > maxBody

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Result{}, fail(ErrNoRelease, ErrNoRelease.Error()+": the repository may be private, or no release is published yet")
	case http.StatusForbidden, http.StatusTooManyRequests:
		return Result{}, rateLimited(resp.Header, body, tooLarge)
	default:
		status := strconv.Itoa(resp.StatusCode)
		if text := http.StatusText(resp.StatusCode); text != "" {
			status += " " + text
		}
		return Result{}, fail(ErrUnavailable, "GitHub answered "+status+" instead of a release; try again later")
	}
	if tooLarge {
		return Result{}, fail(ErrUnavailable, "GitHub's answer was too large (over 1 MiB)")
	}

	var rel *githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return Result{}, notRelease("it is not the JSON GitHub sends for a release")
	}
	switch {
	case rel == nil:
		return Result{}, notRelease("it is empty")
	case !strings.HasPrefix(rel.TagName, "v"):
		return Result{}, notRelease("it has no tag of the form vMAJOR.MINOR.PATCH")
	case rel.Draft:
		return Result{}, notRelease("it is a draft")
	case rel.Prerelease:
		return Result{}, notRelease("it is a prerelease")
	}

	res := Result{
		Current:    o.Current,
		Latest:     rel.TagName,
		ReleaseURL: rel.HTMLURL,
		CheckedAt:  o.Now().UTC(),
	}
	if res.ReleaseURL == "" {
		res.ReleaseURL = ReleasesURL + "/tag/" + rel.TagName
	}
	if t, err := time.Parse(time.RFC3339, rel.PublishedAt); err == nil {
		res.PublishedAt = t.UTC()
	}
	switch c, err := CompareVersions(o.Current, rel.TagName); {
	case err != nil:
		res.State = StateUnknown
	case c < 0:
		res.State = StateUpdateAvailable
	case c == 0:
		res.State = StateUpToDate
	default:
		res.State = StateAhead
	}
	if name := AssetName(rel.TagName, o.Flavor, o.GOOS, o.GOARCH); name != "" {
		for _, a := range rel.Assets {
			if a.Name == name {
				// The link reaches the clipboard, so only a github.com
				// address counts; anything else leaves Asset nil.
				if strings.HasPrefix(a.URL, assetURLPrefix) {
					res.Asset = &Asset{Name: a.Name, URL: a.URL, SizeBytes: a.Size}
				}
				break
			}
		}
	}
	return res, nil
}

// rateLimited builds the 403 or 429 error: the sentinel's sentence, GitHub's
// own message when its JSON has one, and when to try again when a header
// says.
func rateLimited(h http.Header, body []byte, tooLarge bool) error {
	var parts []string
	var answer struct {
		Message string `json:"message"`
	}
	if !tooLarge && json.Unmarshal(body, &answer) == nil {
		if m := printable(answer.Message); m != "" {
			parts = append(parts, "GitHub says: "+m)
		}
	}
	if when := retryWhen(h); when != "" {
		parts = append(parts, when)
	}
	msg := ErrRateLimited.Error()
	if len(parts) > 0 {
		msg += " (" + strings.Join(parts, "; ") + ")"
	}
	return fail(ErrRateLimited, msg)
}

// retryWhen turns Retry-After (seconds or an HTTP date) or
// X-RateLimit-Reset (Unix seconds) into plain words, or "" when neither is
// present and valid.
func retryWhen(h http.Header) string {
	if v := strings.TrimSpace(h.Get("Retry-After")); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil && secs >= 0 && secs < 1<<31 {
			return "try again in " + plainDuration(time.Duration(secs)*time.Second)
		}
		if t, err := http.ParseTime(v); err == nil {
			return "try again after " + t.UTC().Format("2006-01-02 15:04 UTC")
		}
	}
	if v := strings.TrimSpace(h.Get("X-RateLimit-Reset")); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil && secs > 0 && secs < 1<<40 {
			return "try again after " + time.Unix(secs, 0).UTC().Format("2006-01-02 15:04 UTC")
		}
	}
	return ""
}

// plainDuration says d in whole seconds under a minute, otherwise in
// minutes rounded up.
func plainDuration(d time.Duration) string {
	if d < time.Minute {
		n := int(d / time.Second)
		if n == 1 {
			return "1 second"
		}
		return strconv.Itoa(n) + " seconds"
	}
	n := int((d + time.Minute - 1) / time.Minute)
	if n == 1 {
		return "1 minute"
	}
	return strconv.Itoa(n) + " minutes"
}

// printable keeps GitHub's message safe to print in a terminal or show in
// the app: control characters (escape sequences included) are dropped,
// whitespace is collapsed, and the text is cut at 200 characters.
func printable(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "..."
	}
	return s
}
