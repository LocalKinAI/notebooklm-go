package notebooklm

import (
	"context"
	"encoding/json"
	"fmt"
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
