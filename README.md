# notebooklm-go

> **Unofficial pure-Go client for Google [NotebookLM](https://notebooklm.google.com)'s internal RPC.**
> Reverse-engineered from the browser bundle.
> Use at your own risk. Not affiliated with or endorsed by Google.

`notebooklm-go` lets Go programs (and via the bundled `notebooklm` CLI,
shell / Python / JS / cron / Makefile users too) drive NotebookLM
programmatically — list notebooks, add sources (PDF / URL / text), trigger
Audio Overviews / Reports / Mind Maps / Slide Decks / Deep Research, and
download generated artifacts. No browser automation, no Selenium —
direct HTTPS calls to the same `batchexecute` RPC that the official
JS bundle uses.

```bash
go install github.com/LocalKinAI/notebooklm-go/cmd/notebooklm-go@latest
notebooklm-go login       # paste Google session cookie
notebooklm-go list        # list your notebooks
```

> **Why the binary is `notebooklm-go`, not `notebooklm`** — the binary
> name `notebooklm` is already taken by the npm package
> [`notebooklm`](https://www.npmjs.com/package/notebooklm) (Node.js
> port) and the PyPI package [`notebooklm-py`](https://github.com/teng-lin/notebooklm-py)
> (Python port). Suffixing `-go` lets all three coexist on the same
> machine without `$PATH` collisions. The Go *package* import stays
> `notebooklm` (`import notebooklm "github.com/LocalKinAI/notebooklm-go"`).

## Why this exists

NotebookLM is one of the highest-quality consumer LLM products Google
ships: feed it PDFs/URLs, get back a 10–15 min conversational audio
overview, a mind map, a slide deck, a deep research report. The model
runs free for logged-in users (no API, no metering, no token bill).

There is, deliberately, **no public API**. Google's product economics
require it stay UI-only — opening an API would commoditize the most
expensive feature in their consumer line.

Yet programmatic access has obvious uses: bulk-processing a research
library, archiving your own workflow, plugging NotebookLM into an
agent system, building content pipelines.

The official client (the JS bundle) talks to `notebooklm.google.com/
_/LabsTailwindUi/data/batchexecute` over plain HTTPS. This library
talks to the same endpoint, using your existing session cookie.

## ⚠️ Disclaimer

- **Reverse-engineered.** Endpoints and RPC method IDs may change
  without notice. The library will break when Google changes them;
  pin to a release tag, not `master`.
- **Personal use only.** High-volume / commercial scraping likely
  violates Google's Terms of Service. We don't recommend running
  this at fleet scale against accounts you don't own.
- **Not endorsed by Google.** No warranty. No support contract.
  No promises about availability. If Google decides this protocol
  isn't reachable from non-browser clients, the library breaks.
- **Bring your own session.** This library uses *your* logged-in
  Google session cookie. Anything you can do in the UI, the library
  can do — including delete your notebooks. Use a dedicated account
  if you're nervous.

## Install

### As a Go library

```bash
go get github.com/LocalKinAI/notebooklm-go@latest
```

```go
package main

import (
    "fmt"
    "log"

    notebooklm "github.com/LocalKinAI/notebooklm-go"
)

func main() {
    // Reads cookies from ~/.config/notebooklm-go/auth.json by default.
    client, err := notebooklm.NewClient(notebooklm.DefaultStoragePath())
    if err != nil {
        log.Fatal(err)
    }

    notebooks, err := client.ListNotebooks()
    if err != nil {
        log.Fatal(err)
    }
    for _, nb := range notebooks {
        fmt.Printf("%s\t%s\n", nb.ID, nb.Title)
    }
}
```

See [`examples/`](examples/) for runnable variants: `list_notebooks`,
`add_pdf_source`, `generate_audio_overview`.

### As a CLI

```bash
go install github.com/LocalKinAI/notebooklm-go/cmd/notebooklm-go@latest

notebooklm-go login                                # opens browser for one-time OAuth
notebooklm-go list                                 # list notebooks
notebooklm-go create "Paper Reading List"          # create a new notebook
notebooklm-go add <id> https://arxiv.org/pdf/X.pdf # add a URL source
notebooklm-go gen audio <id>                       # request Audio Overview
notebooklm-go download <id> <artifact-id> -o out.mp3
notebooklm-go help                                 # full subcommand list
```

The CLI also covers (v0.1.1, 🧪 experimental): `rename` / `info` /
`summarize` / `source-{get,refresh,rename,delete,guide}` / `conv-{last,turns}`
/ `mindmap-legacy` / `artifact-{export,delete}` / `notes-{list,add}` /
`research-{poll,import}` / `share` / `share-status` / `settings`.

## Authentication

NotebookLM uses standard Google session cookies. v0.2.1 ships two
flows; **the `--attach` flow is strongly recommended** because Google's
anti-automation detection now blocks sign-in on chromedp-launched
Chromes ("Browser not secure").

### 1. ✅ Recommended: `--attach` to your existing Chrome (v0.2.1+)

Snapshot cookies from a normal, already-signed-in Chrome session via
the Chrome DevTools Protocol. Google can't tell the difference between
"user using Chrome" and "Go program reading cookies out of that
Chrome's running process", because that's literally what's happening.

**One-time setup** (then re-usable indefinitely):

```bash
# 1. Fully quit Chrome — Cmd+Q on every window. The --remote-debugging-port
#    flag is silently ignored if any Chrome process is already running.

# 2. Relaunch Chrome with the debug port:
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 &

# 3. Sign into Google in that Chrome and visit https://notebooklm.google.com

# 4. Snapshot cookies:
notebooklm-go login --attach
```

Your normal Chrome profile (history, bookmarks, logged-in sessions)
is preserved. The debug port is the only thing new. Once snapshotted,
`auth.json` is good until the cookies expire (typically months).

### 2. Fallback: chromedp launch (often blocked)

If you don't have Chrome installed yet, or you're on a fresh machine
where Google hasn't seen automation yet:

```bash
notebooklm-go login    # launches a fresh chromedp Chrome
```

The Go library equivalent:

```go
path := notebooklm.DefaultStoragePath()
if err := notebooklm.Login(path); err != nil {  // chromedp launch
    log.Fatal(err)
}
// or, recommended:
if err := notebooklm.LoginAttach(path, 9222); err != nil {  // CDP attach
    log.Fatal(err)
}
client, err := notebooklm.NewClient(path)
```

If `Login` returns "Browser not secure" or similar, switch to
`LoginAttach`.

## RPC method coverage

`rpc.go` declares the known method IDs. All of them are reachable
through the typed `Client` API and the CLI.

Methods are marked **🟢 stable** (battle-tested in LocalKin production)
or **🧪 experimental** (added in v0.1.1 with best-guess param shapes,
not yet verified against a live NotebookLM session — see "EXPERIMENTAL
methods" below).

| Surface       | Methods                                                     | Status |
|---------------|-------------------------------------------------------------|--------|
| Notebooks     | List / Create / Delete                                      | 🟢 stable |
| Notebooks     | Get / Rename                                                | 🧪 experimental |
| Sources       | Add (URL / PDF / text / YouTube)                            | 🟢 stable |
| Sources       | Delete / Get / Refresh / Update                             | 🧪 experimental |
| Summaries     | Get source guide                                            | 🟢 stable |
| Summaries     | Get summary                                                 | 🧪 experimental |
| Artifacts     | Generate (Audio: Deep Dive / Brief / Critique / Debate;     | 🟢 stable |
|               | Report, Video, Quiz, Mind Map, Infographic, Slide Deck,     |        |
|               | Data Table) / List / Download (audio/video/infographic)     |        |
| Artifacts     | Delete / Export (Docs/Sheets)                               | 🧪 experimental |
| Conversations | Chat                                                        | 🟢 stable |
| Conversations | Last conv ID / Conv turns                                   | 🧪 experimental |
| Notes         | Generate Mind Map (legacy RPC) / Create note / Get notes    | 🧪 experimental |
| Research      | Start Fast / Start Deep                                     | 🟢 stable |
| Research      | Poll / Import                                               | 🧪 experimental |
| Sharing       | Get share status                                            | 🟢 stable |
| Sharing       | Share notebook (write)                                      | 🧪 experimental |
| Settings      | Get user settings                                           | 🧪 experimental |
| Auth          | Check auth                                                  | 🟢 stable |

### EXPERIMENTAL methods

The 🧪 methods live in `client_extra.go`. Their RPC method IDs are
known (declared in `rpc.go`) but the param shape — what JSON the
batchexecute call wants in its `params` field — has not been verified
against a live NotebookLM account at release time. The shapes are
best-guesses inferred from neighbouring stable methods and from
notebooklm-py's wire conventions.

If you hit:

```
RPC error for s0tc2d: "..."
```

the param shape is wrong. To debug + fix:

```bash
LOCALKIN_NB_DEBUG=1 ./notebooklm <subcommand> ...
```

This dumps the failing request body and Google's response. File an
issue with the redacted payload and the fix is usually a 1-line edit
to the params slice in `client_extra.go`. PRs welcome — most of
these will graduate to 🟢 with a single round of real-world testing.

## The batchexecute protocol (≈1 paragraph)

Google's frontends talk to backends via a universal envelope called
`batchexecute`. Every "tool call" the JS bundle makes maps to an RPC
identified by a 6-character method ID (e.g. `wXbhsf` for list-notebooks).
A request is a triple-nested JSON: `[[[rpc_id, params_json_string, null,
"generic"]]]`. The response is `)]}'`-prefixed JSON the body of which
is also stringly-encoded inner JSON. Authentication is via cookies
(`SAPISIDHASH` derivation) plus a 32-character `at` token grabbed from
the page HTML. This library handles all of that — you call typed Go
methods, it handles the encoding/decoding.

If you want to add a new RPC: open NotebookLM in Chrome, DevTools →
Network → filter `batchexecute`, take whatever action you want to
reverse-engineer, copy the request payload from the request body, and
add a constant + typed wrapper to `rpc.go` / `client.go`. PRs welcome.

## Why pure Go and not Python?

The existing Python equivalent is [notebooklm-py](https://github.com/
teng-lin/notebooklm-py) — same protocol, different language. We picked
Go because:

- **One static binary.** No `pip install`, no virtualenv, no missing
  system libraries. Cross-compile to Linux ARM64 → drop on a Raspberry
  Pi and it just runs.
- **Goroutines for concurrent scraping.** Batch-add 50 sources, batch-
  generate 50 audio overviews, in parallel without thread-safety
  worries.
- **CLI binary is built-in.** No `python -m notebooklm_py.cli`. Just
  `notebooklm` in your $PATH.
- **Embeddable.** Other Go projects can vendor this without dragging
  in a Python runtime.

We're standing on `notebooklm-py`'s shoulders — the protocol they
reverse-engineered is exactly the protocol we use. Independent Go
re-implementation, no shared code.

## Production usage

This library is the audio backbone of [LocalKin](https://localkin.dev)'s
content publishing pipeline. As of v2.6 of `content_director.soul`,
NotebookLM audio generation is pure-Go via this library (we dropped
Playwright + Chromium and saved ~500 MB RAM per pipeline run).

If you're building a similar pipeline, the [LocalKin papers](https://
localkin.dev/papers) cover the architecture — particularly papers
#2 (Heart cron) and #8 (Swarm Genesis).

## Versioning

Pre-1.0 — the RPC method IDs can change with Google's frontend bundle
updates. We tag releases when we hit breakage in production and have
patched it. Pin to a release tag:

```go
// go.mod
require github.com/LocalKinAI/notebooklm-go v0.1.0
```

If you hit a breakage, file an issue with the failing request payload
(redact your `at` token + cookies first).

## Contributing

PRs welcome — especially for:

- New RPC methods (Mind Map node manipulation, custom artifact prompts, etc.)
- Better headless auth flows
- Test coverage (currently `rpc_test.go` covers envelope encoding only)
- CLI command coverage
- Documentation in `examples/`

See [`examples/`](examples/) for working programs.

## Acknowledgments

- [notebooklm-py](https://github.com/teng-lin/notebooklm-py) — the
  Python project that first reverse-engineered the batchexecute
  protocol for NotebookLM. This Go port re-implements independently
  but borrows the protocol understanding.
- [chromedp](https://github.com/chromedp/chromedp) — the headless
  Chrome library used for the interactive OAuth path.

## License

Apache License 2.0. See [LICENSE](LICENSE).

## Author

[LocalKinAI](https://localkin.dev) — building AI agents that run on
local hardware. See also [11 research papers on Zenodo](https://
localkin.dev/papers) for the architectural thesis.
