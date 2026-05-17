// notebooklm — CLI wrapper for github.com/LocalKinAI/notebooklm-go.
//
// Lets shell / Python / JS / cron / Makefile users drive Google NotebookLM
// without writing Go. The same library powers the typed Go API (see the
// repo README and examples/), but this binary covers the most common
// pipeline use cases:
//
//	notebooklm login                                    one-time setup
//	notebooklm list                                     list notebooks
//	notebooklm create "Paper Reading List"              new notebook
//	notebooklm add <id> https://arxiv.org/pdf/X.pdf     add URL source
//	notebooklm add <id> /path/to/local.pdf              add PDF source
//	notebooklm add <id> --text "raw text body"         add text source
//	notebooklm chat <id> "What does paper 11 say?"     ask a question
//	notebooklm gen audio <id> --format deepdive       request Audio Overview
//	notebooklm gen mindmap <id>                        request Mind Map
//	notebooklm list-artifacts <id>                     list generated artifacts
//	notebooklm download <id> <artifact-id> -o out.mp3 download artifact
//	notebooklm research <id> "compare LLM agents" --deep
//
// All subcommands respect $NOTEBOOKLM_AUTH for the credentials path
// override; default is ~/.config/notebooklm-go/auth.json.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	notebooklm "github.com/LocalKinAI/notebooklm-go"
)

const usage = `notebooklm — Google NotebookLM CLI (unofficial, reverse-engineered)

USAGE:
  notebooklm <command> [args...]

COMMANDS:
  login                              One-time: paste session cookies
  list                               List all your notebooks
  create <title>                     Create a new notebook
  delete <id>                        Delete a notebook
  add <id> <url|path>                Add a source (auto-detect URL/PDF/text-file)
  add <id> --text "raw text"         Add a raw text source
  chat <id> <question>               Ask the notebook a question
  gen <kind> <id> [--format X]       Generate artifact: audio | mindmap | report | video
  list-artifacts <id>                List generated artifacts for a notebook
  download <id> <artifact-id> [-o]   Download generated artifact to file
  research <id> <query> [--deep]     Start research (fast unless --deep)
  whoami                             Show authenticated identity

OPTIONS:
  --auth PATH                        Credentials file (default $HOME/.config/notebooklm-go/auth.json
                                     or $NOTEBOOKLM_AUTH)
  -o, --output FILE                  Output file for 'download'
  -h, --help                         Show this help

EXAMPLES:
  notebooklm login
  notebooklm list
  notebooklm create "Reading List"
  notebooklm add abc123 https://arxiv.org/pdf/2501.12345v2
  notebooklm add abc123 ~/Downloads/paper.pdf
  notebooklm gen audio abc123 --format deepdive
  notebooklm list-artifacts abc123
  notebooklm download abc123 art-xyz -o overview.mp3

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
	default:
		fmt.Fprintf(os.Stderr, "notebooklm: unknown command %q\n\n%s", cmd, usage)
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
	fmt.Fprintln(os.Stderr, "notebooklm: "+msg)
	os.Exit(1)
}

func client(auth string) *notebooklm.Client {
	c, err := notebooklm.NewClient(auth)
	if err != nil {
		die(fmt.Sprintf("init client: %v\n  (try: notebooklm login)", err))
	}
	return c
}

// MARK: - login

func runLogin(args []string) {
	auth, _ := authPath(args)
	// notebooklm.Login() opens chromedp + paste-cookies fallback. The
	// underlying lib handles UX; we just forward stdout.
	if err := notebooklm.Login(auth); err != nil {
		die(fmt.Sprintf("login failed: %v", err))
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
		fmt.Fprintln(os.Stderr, "(no notebooks — create one with `notebooklm create <title>`)")
	}
}

// MARK: - create

func runCreate(args []string) {
	auth, rest := authPath(args)
	if len(rest) < 1 {
		die("create requires a title: notebooklm create <title>")
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
		die("delete requires a notebook ID: notebooklm delete <id>")
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
		die("add requires notebook ID and source: notebooklm add <id> <url|file|--text \"...\">")
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
		die("chat requires notebook ID and question: notebooklm chat <id> \"<question>\"")
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
		die("gen requires kind + notebook ID: notebooklm gen audio <id>")
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
	default:
		die(fmt.Sprintf("unknown gen kind %q (audio/mindmap/report/video/slides/infographic/quiz)", kind))
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
		die("download requires notebook ID and artifact ID: notebooklm download <id> <artifact-id> [-o file]")
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
		die("research requires notebook ID and query: notebooklm research <id> \"<query>\" [--deep]")
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
		die(fmt.Sprintf("auth check: %v\n  (try: notebooklm login)", err))
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
