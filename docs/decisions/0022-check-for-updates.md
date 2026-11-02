# 0022: Check for updates is the one network request, and a person starts it

- Status: Accepted
- Date: 2026-10-14
- Amends: [ADR 0020](0020-evernote-through-client-mcp.md): its "no network connections, with no exception" becomes "no network connections except a check for updates the person starts"

## Context

Casebook promises that its programs make no network connections ([Privacy and local data](../privacy.md)). [ADR 0020](0020-evernote-through-client-mcp.md) made that promise unconditional again by taking Evernote's network code out, and through release 0.4.0 no HTTP client, telemetry, analytics or update check existed in the Go code or the desktop page, as the no-network audits in [the acceptance record](../acceptance.md#no-network-audit) recorded.

The promise has a cost for the people Casebook is built for. Someone who downloaded the desktop app from the Releases page has no other way to learn that a newer release exists: no app store tells them, no installer checks, and nobody watches a GitHub repository for a program they opened once. A fix or a schema change can sit unreleased on their machine for months. They should be able to ask "is there a newer Casebook?" from inside the app, and get the answer with the link to download it.

The repository was private when this decision was made. GitHub's releases API answers 404 for a private repository to a request without credentials, so until the repository became public every check reported that GitHub lists no release.

## Decision

Casebook gets a manual check for updates: **Check for updates...** in the desktop app's **Data** menu, and `casebook update` on the command line. It is the one network request Casebook makes, and only a person starts it.

- **Manual only.** Nothing runs it at startup, on a timer, after an action or in the background. The desktop item opens a small dialog that says what will be sent, and the request goes out only when the person chooses **Check**. The CLI command runs it once and exits.
- **One GET.** The check asks `https://api.github.com/repos/jameskbb/casebook/releases/latest` for GitHub's latest published release (not a draft, not a prerelease), reads at most 1 MiB of the answer, and follows nothing else.
- **What is sent.** A fixed User-Agent, `Casebook-update-check (+https://github.com/jameskbb/casebook)`, which GitHub requires. Nothing else: no version, no identifier, no operating system, no record and no setting. The version comparison happens on the person's machine after the answer arrives. GitHub sees the IP address, as it would for any visit to a web page.
- **Confined to one package.** The request lives in `internal/update`, the only Casebook package that opens a network connection. Only `internal/cli` and `desktop` may import it, and `TestPackageBoundaries` enforces that shape. The engine, the MCP server (`casebook mcp`), the hooks and doctor import nothing that can reach the network, so the program an AI client starts has no network code path at all. Like `internal/evernote`, this is a documented exception to "every surface goes through `internal/app`": a check for updates involves no records.
- **Never downloads or installs.** The answer names the newer version, its release page and the download that matches the running program (the desktop package for this system, or the `casebook-cli` archive for the CLI). Downloading, checking the file and replacing the program stay the person's steps, described in [Updating Casebook](../updating.md).
- **The desktop page stays locked.** The page's Content-Security-Policy keeps `connect-src 'self'`. The Go side of the desktop app makes the request and hands the page the result.
- **What the person sees.**
  - Up to date: `Casebook v0.4.0 is the latest release.`
  - A newer release: `A newer Casebook is available: v0.5.0 (you have v0.4.0).`, with the release page, the download link when the release has one for this system, and a link to [Updating Casebook](../updating.md). The desktop dialog offers **Open release page** and **Copy download link**.
  - Newer than the latest release (a build from source or a prerelease of the next version): `You have Casebook v0.5.0-rc.1, which is newer than the latest release, v0.4.0.`
  - A build that is not a release version (`dev`): it says so and names the latest release.
  - A failure prints one plain sentence and changes nothing: `could not reach GitHub` with no network, a rate limit message when GitHub limits the address, and, when GitHub answers 404 (a private repository, or no published release), `GitHub lists no release for jameskbb/casebook`. `casebook update` exits 1 for each.

## Alternatives considered

- **An automatic check at startup** (or once a day). Rejected: it breaks the promise that Casebook connects to nothing unless asked, it surprises people who read [Privacy and local data](../privacy.md), and it would put a request on every launch of a program people may run offline on purpose.
- **Self-update: download the new release, or install it.** Rejected: release builds are not code signed yet, so Casebook cannot prove to the operating system or to the person that what it fetched is genuine, and replacing program files that AI clients are running is a step the person takes deliberately, with the checks in [Updating Casebook](../updating.md).
- **Checking from the desktop page with `fetch`.** Rejected: it would loosen the page's Content-Security-Policy from `connect-src 'self'` to a GitHub origin, and the page is the part of the app that displays text an AI wrote.

## Consequences

- [Privacy and local data](../privacy.md), the README, [Updating Casebook](../updating.md) and the boundary test change: the promise becomes "no network connections except a check for updates you start", stated with what is sent, and the test enforces that only the CLI and the desktop app import `internal/update` instead of banning network packages outright.
- While the repository was private, every check reported `GitHub lists no release for jameskbb/casebook`. The feature works unchanged on a public repository; nothing in Casebook carries credentials to read a private one.
- [The acceptance record](../acceptance.md)'s no-network audit paragraphs stay as they are, as the record of what was true on their dates. The next audit must also run `casebook update` in a namespace with no network and expect the `could not reach GitHub` error, alongside `casebook mcp`, the hooks and doctor.
- A person who never chooses the check gets exactly the behavior of 0.4.0.
