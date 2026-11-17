# Security policy

## Supported versions

Security fixes go into the latest release and the `main` branch. Older releases are not patched; update to the latest one.

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub's private vulnerability reporting:

1. Open the repository's Security tab.
2. Choose Report a vulnerability.

Or go straight to https://github.com/jameskbb/casebook/security/advisories/new.

Please do not open public issues, pull requests or discussions for vulnerabilities. A public report exposes every user before a fix exists.

### What to include

- the Casebook version (or the commit, if you built from source) and how you installed it
- your operating system and its version
- the steps to reproduce, with a minimal example file or command if one helps
- what an attacker could do with it, and what they need first (for example local access, or a crafted import file)

Do not include real secrets or personal data in the report; a placeholder works just as well.

## What happens next

The report stays private between you and the maintainers in a GitHub security advisory. The maintainers acknowledge it there, ask follow-up questions, and coordinate the fix and its disclosure with you in the same advisory. This is a small project maintained on a best-effort basis, so there are no promised response or fix timelines. You are welcome to be credited in the published advisory.

## Scope

In scope:

- how Casebook stores local data, including file and folder permissions on the database, backups and exports
- importing untrusted files (Casebook import documents and anything else Casebook reads)
- input validation in the MCP server (`casebook mcp`) and its tools
- the setup commands that edit AI client configuration (`casebook setup claude`, `casebook setup codex`), including backups and removal
- the Claude Code hooks Casebook installs
- the Windows installer's optional PATH change and its removal on uninstall
- release integrity: the build workflow, code signing, notarization and `SHA256SUMS.txt`

Out of scope:

- how AI providers handle data that a user chooses to share with them through a connected AI client. Casebook sends nothing over the network itself, except the check for updates you start, which sends no data ([Updating Casebook](docs/updating.md#checking-for-a-newer-version)). A summary the person asks their AI client to save in another app travels through that client and the app's own tools, never through Casebook; [Privacy and local data](docs/privacy.md) explains what is stored and what leaves your machine.
- vulnerabilities in the AI clients themselves (report those to their vendors)
- attacks that already require full control of the user's account on the machine

## For maintainers

Keep the Private vulnerability reporting option enabled in the repository's security settings, so the link above keeps working.
