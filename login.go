package notebooklm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Login opens a browser window for the user to sign into Google,
// then captures cookies and saves them as a Playwright-compatible storage state.
func Login(storagePath string) error {
	if storagePath == "" {
		storagePath = DefaultStoragePath()
	}

	// Ensure directory exists
	dir := filepath.Dir(storagePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating storage directory: %w", err)
	}

	fmt.Println("  Opening browser for Google sign-in...")
	fmt.Println("  Sign in to your Google account, then navigate to NotebookLM.")
	fmt.Println("  The browser will close automatically once authenticated.")
	fmt.Println()

	// Launch visible (non-headless) Chrome
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.WindowSize(1200, 800),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Navigate to NotebookLM — will redirect to Google sign-in
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://notebooklm.google.com"),
	); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}

	// Poll until we detect we're on notebooklm.google.com (authenticated)
	fmt.Println("  Waiting for authentication...")
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	authenticated := false
	for !authenticated {
		select {
		case <-timeout:
			return fmt.Errorf("login timed out after 5 minutes")
		case <-ticker.C:
			var currentURL string
			if err := chromedp.Run(ctx,
				chromedp.Location(&currentURL),
			); err != nil {
				continue
			}
			// Check if we've landed on NotebookLM (not a Google sign-in page)
			if currentURL == "https://notebooklm.google.com/" ||
				currentURL == "https://notebooklm.google.com" {
				authenticated = true
			}
		}
	}

	// Small delay to ensure cookies are fully set
	time.Sleep(2 * time.Second)

	// Extract cookies from browser via CDP
	fmt.Println("  Authenticated! Extracting cookies...")
	state := storageState{}
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			for _, c := range cookies {
				if !isGoogleDomain(c.Domain) {
					continue
				}
				sc := storageCookie{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   c.Domain,
					Path:     c.Path,
					Secure:   c.Secure,
					HTTPOnly: c.HTTPOnly,
					SameSite: string(c.SameSite),
				}
				if c.Expires < 0 {
					sc.Expires = -1
				} else {
					sc.Expires = float64(c.Expires)
				}
				state.Cookies = append(state.Cookies, sc)
			}
			return nil
		}),
	); err != nil {
		return fmt.Errorf("extracting cookies: %w", err)
	}

	// Save storage state
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling storage state: %w", err)
	}
	if err := os.WriteFile(storagePath, data, 0600); err != nil {
		return fmt.Errorf("writing storage state: %w", err)
	}

	fmt.Printf("\n  Login successful! Cookies saved to: %s\n", storagePath)
	fmt.Printf("  Found %d Google cookies\n\n", len(state.Cookies))

	return nil
}

// LoginAttach connects to an already-running Chrome via the Chrome
// DevTools Protocol (CDP) and snapshots its Google cookies into the
// Playwright-compatible storage state at storagePath.
//
// Why this exists: Google's anti-bot detection blocks sign-in on Chromes
// launched by chromedp ("Browser not secure"). Attaching to the user's
// real, already-signed-in Chrome bypasses the detection entirely — we
// just read cookies out of a normal browsing session.
//
// Prerequisites:
//
//  1. Fully quit Chrome (Cmd+Q on macOS — must close ALL windows, otherwise
//     the new launch attaches to the existing process and silently drops
//     the --remote-debugging-port flag).
//
//  2. Relaunch Chrome with the remote debugging port. On macOS:
//
//     /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
//     --remote-debugging-port=9222 &
//
//     This preserves your default profile (cookies, login state) — only the
//     debug port is new.
//
//  3. In that Chrome, sign into Google and open https://notebooklm.google.com
//     (or verify you're already signed in).
//
//  4. Run `notebooklm-go login --attach`.
//
// port=0 defaults to 9222.
func LoginAttach(storagePath string, port int) error {
	if storagePath == "" {
		storagePath = DefaultStoragePath()
	}
	if port == 0 {
		port = 9222
	}

	// Ensure storage directory exists.
	if err := os.MkdirAll(filepath.Dir(storagePath), 0700); err != nil {
		return fmt.Errorf("creating storage directory: %w", err)
	}

	// Resolve the WebSocket debugger URL by hitting Chrome's
	// /json/version endpoint. This both verifies Chrome is up and gives
	// us the URL chromedp.NewRemoteAllocator needs.
	versionURL := fmt.Sprintf("http://localhost:%d/json/version", port)
	resp, err := http.Get(versionURL)
	if err != nil {
		return fmt.Errorf(
			"cannot reach Chrome's debug port on localhost:%d — %w\n\n"+
				"  Quit Chrome fully (Cmd+Q on every window), then relaunch with:\n"+
				"    /Applications/Google\\ Chrome.app/Contents/MacOS/Google\\ Chrome \\\n"+
				"      --remote-debugging-port=%d &\n"+
				"  Then sign into Google in that Chrome and re-run this command.",
			port, err, port)
	}
	defer resp.Body.Close()

	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return fmt.Errorf("parsing %s: %w", versionURL, err)
	}
	if v.WebSocketDebuggerURL == "" {
		return fmt.Errorf("%s returned no webSocketDebuggerUrl — is this really Chrome?", versionURL)
	}

	fmt.Printf("  Attached to Chrome on localhost:%d\n", port)
	fmt.Println("  Opening NotebookLM in a new tab to ensure cookies are loaded...")

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), v.WebSocketDebuggerURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Navigate to NotebookLM. If the user is signed in, the page loads
	// directly. If not, Google's sign-in flow takes over — but in the
	// USER'S real Chrome, which Google doesn't flag as automated. We
	// don't need to detect "signed in"; we just need cookies present.
	state := storageState{}
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://notebooklm.google.com"),
		chromedp.Sleep(2*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			for _, c := range cookies {
				if !isGoogleDomain(c.Domain) {
					continue
				}
				sc := storageCookie{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   c.Domain,
					Path:     c.Path,
					Secure:   c.Secure,
					HTTPOnly: c.HTTPOnly,
					SameSite: string(c.SameSite),
				}
				if c.Expires < 0 {
					sc.Expires = -1
				} else {
					sc.Expires = float64(c.Expires)
				}
				state.Cookies = append(state.Cookies, sc)
			}
			return nil
		}),
	); err != nil {
		return fmt.Errorf("snapshotting cookies via CDP: %w", err)
	}

	if len(state.Cookies) == 0 {
		return fmt.Errorf(
			"snapshot returned 0 Google cookies — are you signed into Google in the attached Chrome?\n" +
				"  Open https://notebooklm.google.com in that Chrome window, sign in if prompted, then retry.")
	}

	// Save.
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling storage state: %w", err)
	}
	if err := os.WriteFile(storagePath, data, 0600); err != nil {
		return fmt.Errorf("writing storage state: %w", err)
	}

	fmt.Printf("\n  Login successful! Cookies saved to: %s\n", storagePath)
	fmt.Printf("  Found %d Google cookies — you can close the new tab in Chrome now.\n\n", len(state.Cookies))
	return nil
}
