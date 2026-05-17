// list_notebooks — the simplest possible notebooklm-go program.
//
// Reads your saved credentials, calls ListNotebooks, prints title + ID
// for each. Use this as the "is auth working?" smoke test before
// writing more elaborate pipelines.
//
//	cd examples/list_notebooks && go run .
//
// Expects credentials at ~/.config/notebooklm-go/auth.json (run
// `notebooklm login` first if you haven't).
package main

import (
	"fmt"
	"log"
	"os"

	notebooklm "github.com/LocalKinAI/notebooklm-go"
)

func main() {
	authPath := os.Getenv("NOTEBOOKLM_AUTH")
	if authPath == "" {
		authPath = notebooklm.DefaultStoragePath()
	}

	client, err := notebooklm.NewClient(authPath)
	if err != nil {
		log.Fatalf("init: %v (try: notebooklm login)", err)
	}

	notebooks, err := client.ListNotebooks()
	if err != nil {
		log.Fatalf("list: %v", err)
	}

	if len(notebooks) == 0 {
		fmt.Println("no notebooks yet — create one in the NotebookLM UI or via the CLI")
		return
	}
	for _, nb := range notebooks {
		fmt.Printf("%s\t%s\n", nb.ID, nb.Title)
	}
}
