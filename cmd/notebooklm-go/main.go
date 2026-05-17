// notebooklm-go — CLI wrapper for github.com/LocalKinAI/notebooklm-go.
//
// Lets shell / Python / JS / cron / Makefile users drive Google NotebookLM
// without writing Go. The same library powers the typed Go API (see the
// repo README and examples/), but this binary covers the most common
// pipeline use cases:
//
//	notebooklm-go login                                    one-time setup
//	notebooklm-go list                                     list notebooks
//	notebooklm-go create "Paper Reading List"              new notebook
//	notebooklm-go add <id> https://arxiv.org/pdf/X.pdf     add URL source
//	notebooklm-go add <id> /path/to/local.pdf              add PDF source
//	notebooklm-go add <id> --text "raw text body"         add text source
//	notebooklm-go chat <id> "What does paper 11 say?"     ask a question
//	notebooklm-go gen audio <id> --format deepdive       request Audio Overview
//	notebooklm-go gen mindmap <id>                        request Mind Map
//	notebooklm-go list-artifacts <id>                     list generated artifacts
//	notebooklm-go download <id> <artifact-id> -o out.mp3 download artifact
//	notebooklm-go research <id> "compare LLM agents" --deep
//
// All subcommands respect $NOTEBOOKLM_AUTH for the credentials path
// override; default is ~/.config/notebooklm-go/auth.json.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	notebooklm "github.com/LocalKinAI/notebooklm-go"
)

const usage = `notebooklm-go — Google NotebookLM CLI (unofficial, reverse-engineered)

USAGE:
  notebooklm-go <command> [args...]

NOTEBOOKS:
  login [--attach] [--port N]          One-time auth. Default launches chromedp Chrome
                                       (often blocked by Google's anti-automation).
                                       --attach connects to a running Chrome via the
                                       Chrome DevTools Protocol — recommended.
  whoami                               Verify auth works
  list                                 List all your notebooks
  create <title>                       Create a new notebook
  rename <id> <new-title>              Rename a notebook                          [EXPERIMENTAL]
  info <id>                            Dump full notebook metadata (raw JSON)
  delete <id>                          Delete a notebook
  summarize <id>                       Get the auto-summary for a notebook        [EXPERIMENTAL]

SOURCES:
  add <id> <url|path>                  Add a source (auto-detect URL/PDF/text-file/YouTube)
  add <id> --text "raw text"           Add a raw text source
  source-get <id> <src-id>             Get a single source's record               [EXPERIMENTAL]
  source-refresh <id> <src-id>         Re-ingest a source                          [EXPERIMENTAL]
  source-rename <id> <src-id> <title>  Rename a source                             [EXPERIMENTAL]
  source-delete <id> <src-id>          Delete a source                             [EXPERIMENTAL]
  source-guide <id>                    Get the per-source summary

CHAT & CONVERSATIONS:
  chat <id> <question>                 Ask the notebook a question
  conv-last <id>                       Get the most-recent conversation ID         [EXPERIMENTAL]
  conv-turns <id> <conv-id>            Dump a conversation's message history       [EXPERIMENTAL]

ARTIFACTS:
  gen <kind> <id> [--format X]         Generate: audio | mindmap | report | video | slides
                                       | infographic | quiz | data-table
  mindmap-legacy <id>                  Mind map via legacy direct RPC              [EXPERIMENTAL]
  list-artifacts <id>                  List generated artifacts for a notebook
  download <id> <art-id> [-o FILE]     Download artifact (audio/video/infographic)
  artifact-export <id> <art-id>        Export report/data-table to Google Docs    [EXPERIMENTAL]
  artifact-delete <id> <art-id>        Delete an artifact                          [EXPERIMENTAL]

NOTES:
  notes-list <id>                      List notes in a notebook                    [EXPERIMENTAL]
  notes-add <id> <title> <content>     Add a note                                  [EXPERIMENTAL]

RESEARCH:
  research <id> <query> [--deep]       Start a research task (fast unless --deep)
  research-poll <id> <task-id>         Poll a research task's status               [EXPERIMENTAL]
  research-import <id> <task-id>       Import a finished research task as a source [EXPERIMENTAL]

SHARING & SETTINGS:
  share <id> --email E [--level L]     Share a notebook (L: view | edit)           [EXPERIMENTAL]
  share-status <id>                    Get sharing state
  settings                             Get current user settings                   [EXPERIMENTAL]

OPTIONS:
  --auth PATH                          Credentials file (default $HOME/.config/notebooklm-go/auth.json
                                       or $NOTEBOOKLM_AUTH)
  -o, --output FILE                    Output file for 'download'
  -h, --help                           Show this help
  -v, --version, version               Print version + commit + build info

EXAMPLES:
  notebooklm-go login
  notebooklm-go create "Reading List"
  notebooklm-go add abc123 https://arxiv.org/pdf/2501.12345v2
  notebooklm-go gen audio abc123 --format deepdive
  notebooklm-go download abc123 art-xyz -o overview.mp3
  notebooklm-go research abc123 "compare LLM agents" --deep
  notebooklm-go share abc123 --email teammate@example.com --level edit

[EXPERIMENTAL] = library wrapper added in v0.1.1 with best-guess param
shapes. If a call returns "RPC error for <id>", capture the failing
payload (LOCALKIN_NB_DEBUG=1) and file an issue.

LOGIN --attach SETUP (one-time, then re-usable indefinitely):
  1. Quit Chrome fully — Cmd+Q on EVERY Chrome window (the flag is
     ignored if a Chrome process is already running).
  2. Relaunch with the debug port. On macOS:
       /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
         --remote-debugging-port=9222 &
     Your normal Chrome profile is preserved; only the debug port is new.
  3. Sign into Google in that Chrome and open notebooklm.google.com.
  4. Run:  notebooklm-go login --attach

DOCS:
  https://github.com/LocalKinAI/notebooklm-go
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(usage)
	case "-v", "--version", "version":
		runVersion()
	// --- already-shipped v0.1.0 surface ---
	case "login":
		runLogin(args)
	case "list":
		runList(args)
	case "create":
		runCreate(args)
	case "delete":
		runDelete(args)
	case "add":
		runAdd(args)
	case "chat":
		runChat(args)
	case "gen":
		runGen(args)
	case "list-artifacts":
		runListArtifacts(args)
	case "download":
		runDownload(args)
	case "research":
		runResearch(args)
	case "whoami":
		runWhoami(args)
	// --- v0.1.1 surface expansion (each backed by an EXPERIMENTAL lib method) ---
	case "rename":
		runRename(args)
	case "info":
		runInfo(args)
	case "summarize":
		runSummarize(args)
	case "source-get":
		runSourceGet(args)
	case "source-refresh":
		runSourceRefresh(args)
	case "source-rename":
		runSourceRename(args)
	case "source-delete":
		runSourceDelete(args)
	case "source-guide":
		runSourceGuide(args)
	case "conv-last":
		runConvLast(args)
	case "conv-turns":
		runConvTurns(args)
	case "mindmap-legacy":
		runMindMapLegacy(args)
	case "artifact-export":
		runArtifactExport(args)
	case "artifact-delete":
		runArtifactDelete(args)
	case "notes-list":
		runNotesList(args)
	case "notes-add":
		runNotesAdd(args)
	case "research-poll":
		runResearchPoll(args)
	case "research-import":
		runResearchImport(args)
	case "share":
		runShare(args)
	case "share-status":
		runShareStatus(args)
	case "settings":
		runSettings(args)
	default:
		fmt.Fprintf(os.Stderr, "notebooklm-go: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

// authPath resolves the credentials file path from --auth flag, env var,
// or default. Subcommands consume any args remaining after this strip.
func authPath(args []string) (string, []string) {
	// Hand-rolled flag extraction so we keep positional args clean.
	// (flag.Parse would consume them in a way that confuses the CLI's
	// "auth flag works anywhere" UX.)
	out := []string{}
	path := os.Getenv("NOTEBOOKLM_AUTH")
	if path == "" {
		path = notebooklm.DefaultStoragePath()
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--auth":
			if i+1 >= len(args) {
				die("--auth requires a value")
			}
			path = args[i+1]
			i++
		default:
			out = append(out, args[i])
		}
	}
	return path, out
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, "notebooklm-go: "+msg)
	os.Exit(1)
}

func client(auth string) *notebooklm.Client {
	c, err := notebooklm.NewClient(auth)
	if err != nil {
		die(fmt.Sprintf("init client: %v\n  (try: notebooklm-go login)", err))
	}
	return c
}

// MARK: - login

func runLogin(args []string) {
	auth, rest := authPath(args)

	// Flag parsing — keep simple and explicit so it composes with
	// authPath's hand-rolled extractor.
	attach := false
	port := 9222
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--attach", "-a":
			attach = true
		case "--port":
			if i+1 < len(rest) {
				p, err := strconv.Atoi(rest[i+1])
				if err != nil {
					die(fmt.Sprintf("--port: not a number: %q", rest[i+1]))
				}
				port = p
				i++
			}
		}
	}

	if attach {
		// Snapshot cookies from a running Chrome via CDP. Use this when
		// Google blocks the chromedp launch (the default path) with
		// "Browser not secure" — they aggressively detect automation.
		if err := notebooklm.LoginAttach(auth, port); err != nil {
			die(fmt.Sprintf("login --attach failed: %v", err))
		}
		fmt.Printf("Auth saved to %s\n", auth)
		return
	}

	// Default: launch a fresh chromedp Chrome and let the user sign in.
	// Works when Google's detection is lenient — usually not — and
	// when no other Chrome is running.
	if err := notebooklm.Login(auth); err != nil {
		die(fmt.Sprintf("login failed: %v\n  (try: notebooklm-go login --attach — see `notebooklm-go help` for prerequisites)", err))
	}
	fmt.Printf("Auth saved to %s\n", auth)
}

// MARK: - list

func runList(args []string) {
	auth, _ := authPath(args)
	c := client(auth)
	notebooks, err := c.ListNotebooks()
	if err != nil {
		die(fmt.Sprintf("list: %v", err))
	}
	for _, nb := range notebooks {
		fmt.Printf("%s\t%s\n", nb.ID, nb.Title)
	}
	if len(notebooks) == 0 {
		fmt.Fprintln(os.Stderr, "(no notebooks — create one with `notebooklm-go create <title>`)")
	}
}

// MARK: - create

func runCreate(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("create requires a title: notebooklm-go create <title>")
	}
	title := strings.Join(rest, " ")
	c := client(auth)
	id, err := c.CreateNotebook(title)
	if err != nil {
		die(fmt.Sprintf("create: %v", err))
	}
	fmt.Println(id)
}

// MARK: - delete

func runDelete(args []string) {
	auth, rest := authPath(args)
	if len(rest) != 1 {
		die("delete requires a notebook ID: notebooklm-go delete <id>")
	}
	c := client(auth)
	if err := c.DeleteNotebook(rest[0]); err != nil {
		die(fmt.Sprintf("delete: %v", err))
	}
	fmt.Println("deleted")
}

// MARK: - add

func runAdd(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("add requires notebook ID and source: notebooklm-go add <id> <url|file|--text \"...\">")
	}
	id := rest[0]
	c := client(auth)

	// Three input modes: --text raw, http(s) URL, or local file path.
	if rest[1] == "--text" {
		if len(rest) < 3 {
			die("--text requires the raw text body")
		}
		body := strings.Join(rest[2:], " ")
		title := truncate(body, 60)
		if err := c.AddTextSource(id, title, body); err != nil {
			die(fmt.Sprintf("add text: %v", err))
		}
		fmt.Println("ok: added text source")
		return
	}

	src := rest[1]
	switch {
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		// YouTube specifically routes to a different RPC server-side;
		// detect by URL host so users don't need to know.
		if strings.Contains(src, "youtube.com") || strings.Contains(src, "youtu.be") {
			if err := c.AddYouTubeSource(id, src); err != nil {
				die(fmt.Sprintf("add YouTube: %v", err))
			}
			fmt.Println("ok: added YouTube source")
			return
		}
		if err := c.AddURLSource(id, src); err != nil {
			die(fmt.Sprintf("add URL: %v", err))
		}
		fmt.Println("ok: added URL source")
	default:
		// Treat as local file path. NotebookLM accepts PDFs primarily.
		// Read + add as text source — the LM-side parser handles binary
		// upload separately via a different endpoint we haven't wrapped
		// in v0.1.0 yet. Hint the user.
		data, err := os.ReadFile(src)
		if err != nil {
			die(fmt.Sprintf("read %s: %v", src, err))
		}
		title := src
		body := string(data)
		if err := c.AddTextSource(id, title, body); err != nil {
			die(fmt.Sprintf("add file: %v", err))
		}
		fmt.Fprintf(os.Stderr, "note: %q added as text source. Binary PDF upload coming in v0.2.\n", src)
		fmt.Println("ok: added file as text source")
	}
}

// MARK: - chat

func runChat(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("chat requires notebook ID and question: notebooklm-go chat <id> \"<question>\"")
	}
	id := rest[0]
	question := strings.Join(rest[1:], " ")
	c := client(auth)
	answer, err := c.Chat(id, question)
	if err != nil {
		die(fmt.Sprintf("chat: %v", err))
	}
	fmt.Println(answer)
}

// MARK: - gen (audio / mindmap / report / video / etc.)

func runGen(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("gen requires kind + notebook ID: notebooklm-go gen audio <id>")
	}
	kind := rest[0]
	id := rest[1]
	format := ""
	for i := 2; i < len(rest); i++ {
		if rest[i] == "--format" && i+1 < len(rest) {
			format = rest[i+1]
			i++
		}
	}
	c := client(auth)

	switch strings.ToLower(kind) {
	case "audio":
		formatCode := audioFormatCode(format)
		artifactID, err := c.GenerateAudio(id, formatCode)
		if err != nil {
			die(fmt.Sprintf("gen audio: %v", err))
		}
		fmt.Println(artifactID)
	case "mindmap", "mind-map", "mind_map":
		artifactID, err := c.GenerateArtifact(id, notebooklm.ArtifactMindMap)
		if err != nil {
			die(fmt.Sprintf("gen mindmap: %v", err))
		}
		fmt.Println(artifactID)
	case "report":
		artifactID, err := c.GenerateArtifact(id, notebooklm.ArtifactReport)
		if err != nil {
			die(fmt.Sprintf("gen report: %v", err))
		}
		fmt.Println(artifactID)
	case "video":
		artifactID, err := c.GenerateArtifact(id, notebooklm.ArtifactVideo)
		if err != nil {
			die(fmt.Sprintf("gen video: %v", err))
		}
		fmt.Println(artifactID)
	case "slidedeck", "slides", "slide-deck", "slide_deck":
		artifactID, err := c.GenerateArtifact(id, notebooklm.ArtifactSlideDeck)
		if err != nil {
			die(fmt.Sprintf("gen slides: %v", err))
		}
		fmt.Println(artifactID)
	case "infographic":
		artifactID, err := c.GenerateArtifact(id, notebooklm.ArtifactInfogr)
		if err != nil {
			die(fmt.Sprintf("gen infographic: %v", err))
		}
		fmt.Println(artifactID)
	case "quiz":
		artifactID, err := c.GenerateArtifact(id, notebooklm.ArtifactQuiz)
		if err != nil {
			die(fmt.Sprintf("gen quiz: %v", err))
		}
		fmt.Println(artifactID)
	case "datatable", "data-table", "data_table":
		artifactID, err := c.GenerateArtifact(id, notebooklm.ArtifactDataTable)
		if err != nil {
			die(fmt.Sprintf("gen data-table: %v", err))
		}
		fmt.Println(artifactID)
	default:
		die(fmt.Sprintf("unknown gen kind %q (audio/mindmap/report/video/slides/infographic/quiz/data-table)", kind))
	}
}

// audioFormatCode maps human-readable Audio Overview formats to the
// integer codes the batchexecute RPC expects. Defaults to Deep Dive
// (the original NotebookLM podcast experience) when format is empty.
func audioFormatCode(format string) int {
	switch strings.ToLower(strings.ReplaceAll(format, "-", "")) {
	case "", "deepdive", "deep_dive":
		return notebooklm.AudioDeepDive
	case "brief":
		return notebooklm.AudioBrief
	case "critique":
		return notebooklm.AudioCritique
	case "debate":
		return notebooklm.AudioDebate
	default:
		die(fmt.Sprintf("unknown audio format %q (deepdive/brief/critique/debate)", format))
		return 0
	}
}

// MARK: - list-artifacts

func runListArtifacts(args []string) {
	auth, rest := authPath(args)
	if len(rest) != 1 {
		die("list-artifacts requires notebook ID")
	}
	c := client(auth)
	arts, err := c.ListArtifacts(rest[0])
	if err != nil {
		die(fmt.Sprintf("list-artifacts: %v", err))
	}
	for _, a := range arts {
		fmt.Printf("%s\t%s\t%s\n", a.ID, artifactTypeName(a.TypeCode), a.Title)
	}
}

// artifactTypeName maps the integer type code back to a human-readable
// label. The codes themselves are stable across releases (defined in
// rpc.go); this helper is purely a UX layer.
func artifactTypeName(code int) string {
	switch code {
	case notebooklm.ArtifactAudio:
		return "audio"
	case notebooklm.ArtifactReport:
		return "report"
	case notebooklm.ArtifactVideo:
		return "video"
	case notebooklm.ArtifactQuiz:
		return "quiz"
	case notebooklm.ArtifactMindMap:
		return "mindmap"
	case notebooklm.ArtifactInfogr:
		return "infographic"
	case notebooklm.ArtifactSlideDeck:
		return "slides"
	case notebooklm.ArtifactDataTable:
		return "table"
	default:
		return fmt.Sprintf("type-%d", code)
	}
}

// MARK: - download

func runDownload(args []string) {
	// Custom flag parsing because we want positional `<id> <artifact-id>`
	// before the -o flag.
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	outPath := fs.String("o", "", "output file (default: artifact-id.ext)")
	fs.StringVar(outPath, "output", "", "output file (alias for -o)")
	authFlag := fs.String("auth", "", "credentials file path")

	// Strip --auth / -o early so positional parsing works.
	if err := fs.Parse(args); err != nil {
		die(err.Error())
	}
	positional := fs.Args()
	if len(positional) != 2 {
		die("download requires notebook ID and artifact ID: notebooklm-go download <id> <artifact-id> [-o file]")
	}
	auth := *authFlag
	if auth == "" {
		auth = os.Getenv("NOTEBOOKLM_AUTH")
		if auth == "" {
			auth = notebooklm.DefaultStoragePath()
		}
	}
	c := client(auth)

	dest := *outPath
	if dest == "" {
		// Fall back to artifact-id.bin — extension is unknown without
		// inspecting the artifact type, which would need a list-artifacts
		// roundtrip. v0.2 plans: auto-detect MP3/MP4/PDF/PPTX.
		dest = positional[1] + ".bin"
	}
	if err := c.DownloadArtifact(positional[0], positional[1], dest); err != nil {
		die(fmt.Sprintf("download: %v", err))
	}
	fmt.Printf("ok: wrote %s\n", dest)
}

// MARK: - research

func runResearch(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("research requires notebook ID and query: notebooklm-go research <id> \"<query>\" [--deep]")
	}
	id := rest[0]
	mode := "fast"
	queryParts := []string{}
	for _, a := range rest[1:] {
		if a == "--deep" {
			mode = "deep"
			continue
		}
		queryParts = append(queryParts, a)
	}
	query := strings.Join(queryParts, " ")

	c := client(auth)
	taskID, err := c.StartResearch(id, query, mode)
	if err != nil {
		die(fmt.Sprintf("research: %v", err))
	}
	fmt.Printf("research task: %s (poll via list-artifacts)\n", taskID)
}

// MARK: - whoami

func runWhoami(args []string) {
	auth, _ := authPath(args)
	c := client(auth)
	if err := c.CheckAuth(); err != nil {
		die(fmt.Sprintf("auth check: %v\n  (try: notebooklm-go login)", err))
	}
	fmt.Printf("ok: authenticated, credentials at %s\n", auth)
}

// MARK: - helpers

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// (kept here so future subcommands can `bufio.NewReader(os.Stdin)` for
// interactive prompts without re-importing.)
var _ = bufio.NewReader

// printRaw dumps a json.RawMessage to stdout, pretty-printed when possible.
// Used by the read-only EXPERIMENTAL subcommands that surface raw RPC
// responses — Google's protocol doesn't have a stable typed schema, so we
// hand the JSON back to the caller for downstream `jq` / scripting.
func printRaw(label string, raw []byte) {
	if len(raw) == 0 {
		fmt.Printf("%s: (empty)\n", label)
		return
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err == nil {
		fmt.Println(pretty.String())
		return
	}
	// Not valid JSON — print as string.
	fmt.Println(string(raw))
}

// =============================================================================
// v0.1.1 EXPERIMENTAL subcommands
// =============================================================================
//
// Every handler below is a thin wrapper around an EXPERIMENTAL Client method
// in client_extra.go. If a call returns `RPC error for <id>: ...`, the param
// shape needs fixing — file an issue with the failing payload (capture via
// LOCALKIN_NB_DEBUG=1 ./notebooklm-go ...).

// MARK: - rename / info / summarize

func runRename(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("rename requires notebook ID and new title: notebooklm-go rename <id> <new-title>")
	}
	c := client(auth)
	newTitle := strings.Join(rest[1:], " ")
	if err := c.RenameNotebook(rest[0], newTitle); err != nil {
		die(fmt.Sprintf("rename: %v", err))
	}
	fmt.Printf("ok: renamed %s → %q\n", rest[0], newTitle)
}

func runInfo(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("info requires notebook ID: notebooklm-go info <id>")
	}
	c := client(auth)
	raw, err := c.GetNotebookMetadata(rest[0])
	if err != nil {
		die(fmt.Sprintf("info: %v", err))
	}
	printRaw("info", raw)
}

func runSummarize(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("summarize requires notebook ID: notebooklm-go summarize <id>")
	}
	c := client(auth)
	raw, err := c.Summarize(rest[0])
	if err != nil {
		die(fmt.Sprintf("summarize: %v", err))
	}
	printRaw("summary", raw)
}

// MARK: - source-*

func runSourceGet(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("source-get requires notebook ID and source ID: notebooklm-go source-get <id> <src-id>")
	}
	c := client(auth)
	raw, err := c.GetSource(rest[0], rest[1])
	if err != nil {
		die(fmt.Sprintf("source-get: %v", err))
	}
	printRaw("source", raw)
}

func runSourceRefresh(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("source-refresh requires notebook ID and source ID")
	}
	c := client(auth)
	if err := c.RefreshSource(rest[0], rest[1]); err != nil {
		die(fmt.Sprintf("source-refresh: %v", err))
	}
	fmt.Printf("ok: refreshed source %s\n", rest[1])
}

func runSourceRename(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 3 {
		die("source-rename requires notebook ID, source ID, and new title")
	}
	c := client(auth)
	newTitle := strings.Join(rest[2:], " ")
	if err := c.UpdateSource(rest[0], rest[1], newTitle); err != nil {
		die(fmt.Sprintf("source-rename: %v", err))
	}
	fmt.Printf("ok: renamed source %s → %q\n", rest[1], newTitle)
}

func runSourceDelete(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("source-delete requires notebook ID and source ID")
	}
	c := client(auth)
	if err := c.DeleteSource(rest[0], rest[1]); err != nil {
		die(fmt.Sprintf("source-delete: %v", err))
	}
	fmt.Printf("ok: deleted source %s\n", rest[1])
}

func runSourceGuide(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("source-guide requires notebook ID")
	}
	c := client(auth)
	guide, err := c.GetSourceGuide(rest[0])
	if err != nil {
		die(fmt.Sprintf("source-guide: %v", err))
	}
	fmt.Println(guide)
}

// MARK: - conv-*

func runConvLast(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("conv-last requires notebook ID")
	}
	c := client(auth)
	id, err := c.GetLastConvID(rest[0])
	if err != nil {
		die(fmt.Sprintf("conv-last: %v", err))
	}
	fmt.Println(id)
}

func runConvTurns(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("conv-turns requires notebook ID and conv ID")
	}
	c := client(auth)
	raw, err := c.GetConvTurns(rest[0], rest[1])
	if err != nil {
		die(fmt.Sprintf("conv-turns: %v", err))
	}
	printRaw("turns", raw)
}

// MARK: - mindmap-legacy / artifact-export / artifact-delete

func runMindMapLegacy(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("mindmap-legacy requires notebook ID")
	}
	c := client(auth)
	raw, err := c.GenerateMindMap(rest[0])
	if err != nil {
		die(fmt.Sprintf("mindmap-legacy: %v", err))
	}
	printRaw("mindmap", raw)
}

func runArtifactExport(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("artifact-export requires notebook ID and artifact ID")
	}
	c := client(auth)
	raw, err := c.ExportArtifact(rest[0], rest[1])
	if err != nil {
		die(fmt.Sprintf("artifact-export: %v", err))
	}
	printRaw("export", raw)
}

func runArtifactDelete(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("artifact-delete requires notebook ID and artifact ID")
	}
	c := client(auth)
	if err := c.DeleteArtifact(rest[0], rest[1]); err != nil {
		die(fmt.Sprintf("artifact-delete: %v", err))
	}
	fmt.Printf("ok: deleted artifact %s\n", rest[1])
}

// MARK: - notes-list / notes-add

func runNotesList(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("notes-list requires notebook ID")
	}
	c := client(auth)
	raw, err := c.GetNotes(rest[0])
	if err != nil {
		die(fmt.Sprintf("notes-list: %v", err))
	}
	printRaw("notes", raw)
}

func runNotesAdd(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 3 {
		die("notes-add requires notebook ID, title, content: notebooklm-go notes-add <id> <title> <content>")
	}
	c := client(auth)
	title := rest[1]
	content := strings.Join(rest[2:], " ")
	if err := c.CreateNote(rest[0], title, content); err != nil {
		die(fmt.Sprintf("notes-add: %v", err))
	}
	fmt.Printf("ok: added note %q\n", title)
}

// MARK: - research-poll / research-import

func runResearchPoll(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("research-poll requires notebook ID and task ID")
	}
	c := client(auth)
	raw, err := c.PollResearch(rest[0], rest[1])
	if err != nil {
		die(fmt.Sprintf("research-poll: %v", err))
	}
	printRaw("poll", raw)
}

func runResearchImport(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 2 {
		die("research-import requires notebook ID and task ID")
	}
	c := client(auth)
	if err := c.ImportResearch(rest[0], rest[1]); err != nil {
		die(fmt.Sprintf("research-import: %v", err))
	}
	fmt.Printf("ok: imported research %s\n", rest[1])
}

// MARK: - share / share-status / settings

func runShare(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("share requires notebook ID: notebooklm-go share <id> --email <e> [--level view|edit]")
	}
	id := rest[0]
	var emails []string
	level := 1 // 1 = view, 2 = edit (best guess)
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case "--email":
			if i+1 < len(rest) {
				emails = append(emails, rest[i+1])
				i++
			}
		case "--level":
			if i+1 < len(rest) {
				switch strings.ToLower(rest[i+1]) {
				case "view", "viewer", "1":
					level = 1
				case "edit", "editor", "2":
					level = 2
				default:
					die(fmt.Sprintf("share: unknown --level %q (use view|edit)", rest[i+1]))
				}
				i++
			}
		}
	}
	if len(emails) == 0 {
		die("share requires at least one --email")
	}
	c := client(auth)
	if err := c.ShareNotebook(id, emails, level); err != nil {
		die(fmt.Sprintf("share: %v", err))
	}
	fmt.Printf("ok: shared %s with %d recipient(s) at level %d\n", id, len(emails), level)
}

func runShareStatus(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("share-status requires notebook ID")
	}
	c := client(auth)
	raw, err := c.GetShareStatus(rest[0])
	if err != nil {
		die(fmt.Sprintf("share-status: %v", err))
	}
	printRaw("share-status", raw)
}

func runSettings(args []string) {
	auth, _ := authPath(args)
	c := client(auth)
	raw, err := c.GetUserSettings()
	if err != nil {
		die(fmt.Sprintf("settings: %v", err))
	}
	printRaw("settings", raw)
}

// MARK: - version

// runVersion prints the binary's version + git commit + build date as
// reported by `runtime/debug.ReadBuildInfo`. When this binary was
// installed via `go install module@version`, the version is the module
// tag (e.g. "v0.2.2"); when built locally with `go build`, it's "(devel)"
// which we display as "dev". The vcs.* keys are populated automatically
// by the Go toolchain at build time as long as VCS info is available.
func runVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("notebooklm-go dev (no build info — built with -trimpath?)")
		return
	}
	version := info.Main.Version
	if version == "" || version == "(devel)" {
		version = "dev"
	}

	var commit, date, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.time":
			date = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = " (dirty)"
			}
		}
	}
	if len(commit) > 12 {
		commit = commit[:12]
	}

	fmt.Printf("notebooklm-go %s\n", version)
	if commit != "" {
		fmt.Printf("  commit:  %s%s\n", commit, dirty)
	}
	if date != "" {
		fmt.Printf("  built:   %s\n", date)
	}
	fmt.Printf("  go:      %s\n", info.GoVersion)
	fmt.Printf("  module:  %s\n", info.Main.Path)
}
