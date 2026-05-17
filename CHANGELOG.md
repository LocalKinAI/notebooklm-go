# Changelog

All notable changes to `notebooklm-go` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project adheres to [Semantic Versioning](https://semver.org/).
Pre-1.0 versions reflect the inherent instability of reverse-engineered
protocols — minor bumps may include breaking method-ID updates when
Google ships frontend changes.

## [Unreleased]

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
