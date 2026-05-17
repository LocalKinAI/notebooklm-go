// generate_audio_overview — request a "Deep Dive" audio overview for a
// notebook, poll until ready, download the MP3.
//
// Usage:
//
//	cd examples/generate_audio_overview
//	go run . <notebook-id> [output-file.mp3]
//
// Defaults the output path to "./overview.mp3". Polls every 10 s for up to
// 6 minutes — typical generation takes 30–90 s but Google's queue can spike
// during peak hours.
//
// Audio format constants live in `rpc.go`:
//
//	AudioDeepDive  = 1  // ~12 min conversational, two-host (this example)
//	AudioBrief     = 2  // ~3 min single-host summary
//	AudioCritique  = 3  // adversarial / steelman
//	AudioDebate    = 4  // two-host disagreement
//
// Swap the format on the GenerateAudio call below to try the others.
//
// Expects credentials at ~/.config/notebooklm-go/auth.json (run
// `notebooklm-go login` first if you haven't). Override with NOTEBOOKLM_AUTH.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	notebooklm "github.com/LocalKinAI/notebooklm-go"
)

const (
	pollInterval = 10 * time.Second
	pollTimeout  = 6 * time.Minute
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintln(os.Stderr, "usage: generate_audio_overview <notebook-id> [output.mp3]")
		os.Exit(2)
	}
	notebookID := os.Args[1]
	output := "overview.mp3"
	if len(os.Args) == 3 {
		output = os.Args[2]
	}

	authPath := os.Getenv("NOTEBOOKLM_AUTH")
	if authPath == "" {
		authPath = notebooklm.DefaultStoragePath()
	}

	client, err := notebooklm.NewClient(authPath)
	if err != nil {
		log.Fatalf("init: %v (try: notebooklm-go login)", err)
	}

	fmt.Printf("requesting Deep Dive audio for notebook %s...\n", notebookID)
	artifactID, err := client.GenerateAudio(notebookID, notebooklm.AudioDeepDive)
	if err != nil {
		log.Fatalf("generate: %v", err)
	}
	if artifactID == "" {
		// Google sometimes accepts the request without returning the ID
		// immediately (the artifact appears in ListArtifacts a few seconds
		// later). Fall through; the poll loop will discover it.
		fmt.Println("request accepted; polling for the new artifact...")
	} else {
		fmt.Printf("artifact id: %s — polling until ready...\n", artifactID)
	}

	// Poll: list artifacts, find ours (by ID if known, else most-recent
	// audio), try DownloadArtifact. If the URL isn't ready yet, the
	// download surfaces a "could not extract download URL" error — we
	// treat that as "still cooking" and retry.
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		id, err := resolveArtifactID(client, notebookID, artifactID)
		if err != nil {
			log.Fatalf("list artifacts: %v", err)
		}
		if id == "" {
			fmt.Println("  ...not visible yet, waiting")
			time.Sleep(pollInterval)
			continue
		}

		err = client.DownloadArtifact(notebookID, id, output)
		if err == nil {
			fmt.Printf("done: %s\n", output)
			return
		}
		if strings.Contains(err.Error(), "could not extract download URL") {
			fmt.Println("  ...still generating, waiting")
			time.Sleep(pollInterval)
			continue
		}
		log.Fatalf("download: %v", err)
	}
	log.Fatalf("timed out after %s waiting for audio to finish", pollTimeout)
}

// resolveArtifactID returns the artifactID we should try to download.
// If GenerateAudio handed us an ID, use it. Otherwise pick the most-recent
// audio artifact (by CreatedAt) — that's the one we just requested.
func resolveArtifactID(client *notebooklm.Client, notebookID, known string) (string, error) {
	if known != "" {
		return known, nil
	}
	artifacts, err := client.ListArtifacts(notebookID)
	if err != nil {
		return "", err
	}
	var best notebooklm.Artifact
	for _, a := range artifacts {
		if a.TypeCode != notebooklm.ArtifactAudio {
			continue
		}
		if a.CreatedAt > best.CreatedAt {
			best = a
		}
	}
	return best.ID, nil
}
