# Changelog

All notable changes to `notebooklm-go` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project adheres to [Semantic Versioning](https://semver.org/).
Pre-1.0 versions reflect the inherent instability of reverse-engineered
protocols — minor bumps may include breaking method-ID updates when
Google ships frontend changes.

## [Unreleased]

## [0.2.2] - 2026-05-16

### Added

- **`notebooklm-go version`** (aliases: `-v`, `--version`) — prints
  the module version, git commit, build date, Go version, and module
  path. Uses `runtime/debug.ReadBuildInfo` so installs via
  `go install ...@vX.Y.Z` self-report their tag correctly.

### Fixed

- Two error messages still prefixed `notebooklm:` instead of
  `notebooklm-go:` (regression from the v0.2.0 rename — the sed
  pattern only matched `notebooklm <space>`, missing `notebooklm:`).
  Now consistently `notebooklm-go: ...`.

## [0.2.1] - 2026-05-16

### Added

- **`notebooklm-go login --attach`** — snapshot Google cookies from an
  already-running Chrome via the Chrome DevTools Protocol. Bypasses
  Google's "Browser not secure" block that hits chromedp-launched
  Chromes. Recommended path for v0.2.1+.
- New library function `notebooklm.LoginAttach(storagePath, port)`.
- `--port` flag for `login` (defaults to 9222).

### Why

Google's anti-automation detection started flagging chromedp-launched
Chromes during the sign-in flow, surfacing as a "Browser not secure"
banner where the password field used to be. Existing `notebooklm-go login`
(the chromedp path) now fails for most users. `--attach` connects to
a real Chrome the user is already running with `--remote-debugging-port=9222`
and just snapshots its cookies — no automation flags on the live session,
no detection trigger.

### Setup

```bash
# Quit Chrome fully (Cmd+Q on every window)
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 &
# Sign in (or verify already signed in) at https://notebooklm.google.com
notebooklm-go login --attach
```

The original chromedp-launch path is preserved as fallback (`notebooklm-go login`
without `--attach`) for the rare case where it still works (fresh
machines where Google hasn't seen automation yet).

## [0.2.0] - 2026-05-16

### Changed (BREAKING — CLI install URL)

- **CLI binary renamed `notebooklm` → `notebooklm-go`** to coexist with
  the npm package [`notebooklm`](https://www.npmjs.com/package/notebooklm)
  (Node.js CLI) and [`notebooklm-py`](https://github.com/teng-lin/notebooklm-py)
  (Python CLI). All three projects had claimed the same binary name,
  causing silent `$PATH` shadowing depending on `~/.nvm` / `~/.pyenv` /
  `~/go/bin` ordering. Suffixing `-go` lets all three live on the same
  machine cleanly.
- **Install URL changed**: was `go install ...notebooklm-go/cmd/notebooklm@v0.1.x`,
  now `go install ...notebooklm-go/cmd/notebooklm-go@v0.2.0`.
- All CLI examples in the README, CHANGELOG, and `examples/*/main.go`
  doc comments updated to invoke `notebooklm-go ...` instead of
  `notebooklm ...`.

### Not changed

- The Go *library* import path is **unchanged**:
  `import notebooklm "github.com/LocalKinAI/notebooklm-go"`. Only the
  CLI binary name changed. Library users see no diff.
- All `Client` methods, the EXPERIMENTAL surface from v0.1.1, the
  RPC method IDs in `rpc.go` — all untouched.

### Migration

If you ran `go install .../cmd/notebooklm@v0.1.x`:

```bash
# (optional) remove the old binary
rm $(go env GOPATH)/bin/notebooklm

# install the new one
go install github.com/LocalKinAI/notebooklm-go/cmd/notebooklm-go@v0.2.0

# old: notebooklm list
# new:
notebooklm-go list
```

## [0.1.1] - 2026-05-16

### Added

- **17 new typed `Client` methods** wrapping every RPC method ID
  declared in `rpc.go` that previously had no Go wrapper. All marked
  🧪 EXPERIMENTAL in godoc: the param shape is the best-guess inferred
  from neighbouring stable methods and notebooklm-py's wire conventions.
  Run with `LOCALKIN_NB_DEBUG=1` to capture failing payloads; file
  issues for 1-line param fixes.
  - Notebooks: `RenameNotebook`, `GetNotebook`
  - Sources: `DeleteSource`, `GetSource`, `RefreshSource`, `UpdateSource`
  - Summaries: `Summarize`
  - Artifacts: `DeleteArtifact`, `ExportArtifact`
  - Conversations: `GetLastConvID`, `GetConvTurns`
  - Notes & legacy mindmap: `GenerateMindMap`, `CreateNote`, `GetNotes`
  - Research: `PollResearch`, `ImportResearch`
  - Sharing: `ShareNotebook`
  - Settings: `GetUserSettings`
- **20 new CLI subcommands** wiring the new typed methods + the 4
  stable lib methods that v0.1.0 missed (`info`, `source-guide`,
  `share-status`, `gen data-table`). `notebooklm-go help` shows the
  full surface with 🧪 markers. (At v0.1.1 the binary was still named
  `notebooklm`; renamed to `notebooklm-go` in v0.2.0.)
- `client_extra.go` to keep the EXPERIMENTAL additions visually
  separated from the production-validated code in `client.go`.

### Changed

- README's "RPC method coverage" table now distinguishes 🟢 stable
  vs 🧪 experimental — v0.1.0's table overclaimed by listing
  not-yet-implemented methods as "covered".
- CHANGELOG v0.1.0 entry's RPC coverage list amended (same reason).

### Notes

No breaking changes. The v0.1.0 stable surface is untouched —
`client.go` was not modified.

## [0.1.0] - 2026-05-16

### Added

- Initial public release. Reverse-engineered Go client for Google
  NotebookLM's internal `batchexecute` RPC.
- ~1,500 LoC of pure Go (zero cgo). Builds to a single static binary.
- Authentication via session-cookie paste (headless) or chromedp
  OAuth flow (interactive Chrome popup).
- Typed `Client` API covering the full known RPC surface:
  - **Notebooks**: list / create / get / rename / delete
  - **Sources**: add (URL/PDF/text) / delete / get / refresh / update
  - **Summarize**: get summary / get source guide
  - **Artifacts**: create / list / delete / export
  - **Audio overview formats**: Deep Dive / Brief / Critique / Debate
  - **Artifact types**: Audio / Report / Video / Quiz / Mind Map /
    Infographic / Slide Deck / Data Table
  - **Conversations**: get last ID / get turns
  - **Notes**: generate mind map / create note / get notes
  - **Research**: start Fast / start Deep / poll / import
  - **Sharing**: share notebook / get share status
  - **Settings**: get user settings
- `cmd/notebooklm` CLI wrapper (binary, renamed to `cmd/notebooklm-go`
  in v0.2.0): subcommands `login`, `list`, `create`, `add`, `gen`,
  `download`. Lets shell / Python / JS / cron / Makefile users drive
  NotebookLM without writing Go.
- Apache 2.0 license.

### Origin

Extracted from `LocalKinAI/localkin-core` (private repo) where this
package has been running in production since April 2026 as the audio
backbone of LocalKin's content publishing pipeline. The extraction
involved zero source changes — the package was already standalone
(no `localkin-core` imports). Tests pass identically before and
after the move.

The protocol understanding is owed to the
[notebooklm-py](https://github.com/teng-lin/notebooklm-py) project
(Python implementation, also reverse-engineered). This Go port is
independent code; no source is shared.

### Known limitations

- RPC method IDs are observed-via-DevTools and not stable across all
  versions of Google's frontend bundle. If Google ships a major
  refactor (rare but possible), method IDs in `rpc.go` will need
  re-discovery.
- The CLI's `login` flow paste-cookies UX is functional but not
  polished — a future minor will add per-cookie prompts with
  inline validation.
- No retry / backoff logic in the HTTP layer yet. Rate-limit
  errors (HTTP 429) surface to the caller as-is.
- No streaming support for long-running operations (audio
  generation typically takes 30–90s). Poll the artifact endpoint
  until `status == ready`.
