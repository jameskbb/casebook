# 0010: Build environment and CI matrix

- Status: Accepted
- Date: 2026-10-01

## Context

The development machine is WSL without gcc or webkit, so Linux desktop builds are not possible there. End users must never need Go or Node. Signing credentials are not yet available.

## Decision

- Go code, the CLI, and the MCP server build and test natively in WSL.
- The Windows desktop app is cross-compiled from WSL (Wails v2 on Windows is CGO-free) and smoke-tested on the Windows host.
- macOS desktop builds run only on GitHub Actions macOS runners.
- CI release matrix: windows-amd64, windows-arm64, darwin-arm64, darwin-amd64 (or a universal macOS build).
- Windows code signing, macOS Developer ID signing, and notarization steps are gated on secrets and skipped cleanly when absent.

## Consequences

- Linux desktop packaging is deferred.
- macOS behavior is verified in CI rather than locally.
- Adding certificates later requires only repository secrets, not workflow rewrites.
