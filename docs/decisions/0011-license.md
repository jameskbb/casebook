# 0011: License recommendation: MIT

- Status: Accepted (2026-10-06)
- Date: 2026-10-02

This record is a recommendation. The maintainer makes the decision and adds the license file. It is not legal advice.

## Context

The brief asks for a license recommendation and forbids inventing ownership or legal claims. The repository may become public, so contributors and users will need to know on what terms they may use, change and share Casebook.

Casebook is a local tool aimed at broad adoption: individuals, small teams and people using it at work. It ships as a desktop app plus a command line tool and links its Go dependencies into both executables.

The direct dependencies and their licenses, confirmed on 2026-10-02 by reading each module's license file in the Go module cache:

| Module | Version | License found |
|---|---|---|
| `github.com/wailsapp/wails/v2` | v2.14.0 | MIT (`LICENSE`) |
| `modernc.org/sqlite` | v1.60.1 | BSD-style three-clause (`LICENSE`); the bundled SQLite code is public domain (`LICENSE-SQLITE`); the module also ships `LICENSE-3RD-PARTY.md` and `LICENSE-SQLITE_VEC` |
| `github.com/spf13/cobra` | v1.10.2 | Apache-2.0 (`LICENSE.txt`) |
| `github.com/pelletier/go-toml/v2` | v2.4.3 | MIT (`LICENSE`) |
| `github.com/modelcontextprotocol/go-sdk` | v1.8.0 | Apache-2.0 for new code, with some earlier contributions still under MIT; its `LICENSE` explains the transition (documentation contributions are CC-BY-4.0) |

The frontend's npm dependencies are added by the desktop track and should be reviewed the same way when they land.

## Decision (recommendation)

License Casebook under the MIT License.

- It is short, permissive and widely understood, so people can read it in a minute.
- Individuals and companies can use, embed and adapt Casebook with little friction, which suits a tool meant to be adopted at work.
- It matches the Wails ecosystem the desktop app is built on.
- It is compatible with every current dependency license listed above (MIT, BSD three-clause, Apache-2.0 and public domain).

## Alternatives considered

- Apache-2.0: also permissive, with an explicit patent grant. It brings more process (NOTICE files and change notices), which is more than a small local tool needs. A reasonable second choice if a patent grant matters to the maintainer.
- GPL-3.0 or AGPL-3.0: copyleft. They would require anyone who distributes a modified Casebook to share their changes under the same license, which adds friction for workplace use. Not recommended for a local tool aimed at broad adoption.

## Consequences

- This phase adds no license file. When the maintainer decides, they add the license file with their own notice.
- Until then, the code has no license grant: people may read it, but the default copyright rules apply to everything else.
- Once a license is chosen, the binary releases should include the third-party license texts (for example for the Apache-2.0 dependencies, and any notices they carry). This is a follow-up for the release workflow.
- The README's License section points here and says the license is still to be finalized.

## Outcome

2026-10-06: the maintainer accepted this recommendation and added [LICENSE](../../LICENSE) (MIT, Copyright (c) 2026 James Krape). The consequences above that say no license is granted describe the state before that date. Release assets carry `LICENSE` and `THIRD_PARTY_NOTICES.txt`; the remaining gap (the Windows installer) is in [docs/follow-ups.md](../follow-ups.md).
