# Changelog

All notable changes to `notebooklm-go` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project adheres to [Semantic Versioning](https://semver.org/).
Pre-1.0 versions reflect the inherent instability of reverse-engineered
protocols — minor bumps may include breaking method-ID updates when
Google ships frontend changes.

## [Unreleased]

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
  `share-status`, `gen data-table`). `notebooklm help` shows the
  full surface with 🧪 markers.
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
- `cmd/notebooklm` CLI wrapper (binary): subcommands `login`, `list`,
  `create`, `add`, `gen`, `download`. Lets shell / Python / JS / cron
  / Makefile users drive NotebookLM without writing Go.
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
