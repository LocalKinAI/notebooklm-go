// add_pdf_source — add a remote PDF (or any URL NotebookLM accepts) to an
// existing notebook.
//
// Usage:
//
//	cd examples/add_pdf_source
//	go run . <notebook-id> <url>
//
// Example — drop the GPT-4 technical report into notebook abc123:
//
//	go run . abc123 https://arxiv.org/pdf/2303.08774.pdf
//
// NotebookLM ingests the PDF server-side (download → parse → embed) and the
// source becomes searchable + summarizable within ~30 seconds.
//
// Expects credentials at ~/.config/notebooklm-go/auth.json (run
// `notebooklm-go login` first if you haven't). Override with NOTEBOOKLM_AUTH.
package main

import (
	"fmt"
	"log"
	"os"

	notebooklm "github.com/LocalKinAI/notebooklm-go"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: add_pdf_source <notebook-id> <url>")
		os.Exit(2)
	}
	notebookID, sourceURL := os.Args[1], os.Args[2]

	authPath := os.Getenv("NOTEBOOKLM_AUTH")
	if authPath == "" {
		authPath = notebooklm.DefaultStoragePath()
	}

	client, err := notebooklm.NewClient(authPath)
	if err != nil {
		log.Fatalf("init: %v (try: notebooklm-go login)", err)
	}

	if err := client.AddURLSource(notebookID, sourceURL); err != nil {
		log.Fatalf("add: %v", err)
	}
	fmt.Printf("added %s to notebook %s\n", sourceURL, notebookID)
}
