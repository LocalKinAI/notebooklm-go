package notebooklm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// storageState represents Playwright's storage state format.
type storageState struct {
	Cookies []storageCookie `json:"cookies"`
}

type storageCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	SameSite string  `json:"sameSite"`
	Expires  float64 `json:"expires"`
}

// Tokens holds the CSRF and session tokens needed for RPC calls.
type Tokens struct {
	CSRF      string // SNlM0e value
	SessionID string // FdrFJe value
}

// LoadCookiesFromStorage reads a Playwright storage_state.json file
// and returns the Cookie header string for Google domains.
func LoadCookiesFromStorage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading storage state: %w", err)
	}

	var state storageState
	if err := json.Unmarshal(data, &state); err != nil {
		return "", fmt.Errorf("parsing storage state: %w", err)
	}

	// Filter to Google domains only
	var parts []string
	for _, c := range state.Cookies {
		if isGoogleDomain(c.Domain) {
			parts = append(parts, c.Name+"="+c.Value)
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no Google cookies found in storage state")
	}
	return strings.Join(parts, "; "), nil
}

// LoadCookieJarFromStorage builds a Go cookiejar from Playwright
// storage_state.json, preserving domain / path / secure / httpOnly info.
//
// This is REQUIRED for media downloads: NotebookLM hands out URLs on
// domains like `googleusercontent.com`, and a single cross-domain Cookie
// header (the shortcut used by RPC calls on notebooklm.google.com) won't
// be honored. A proper jar sends the right cookies per destination host
// as the client follows redirects.
func LoadCookieJarFromStorage(path string) (http.CookieJar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading storage state: %w", err)
	}
	var state storageState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing storage state: %w", err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	// Group cookies by the URL they should attach to, mirroring Playwright's
	// domain field semantics:
	//   - leading dot domain  (".google.com")  → attach to https://google.com and all subdomains
	//   - bare domain         ("google.com")   → attach to https://google.com only
	// net/http/cookiejar.SetCookies takes the URL it would have come from,
	// so synthesize that per cookie.
	hostCookies := map[string][]*http.Cookie{}
	for _, c := range state.Cookies {
		if !isGoogleDomain(c.Domain) {
			continue
		}
		// Strip leading dot for URL synthesis, but preserve it in the Cookie.Domain
		// field so the jar treats this as a domain-wide cookie.
		host := strings.TrimPrefix(c.Domain, ".")
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}
		hostCookies[host] = append(hostCookies[host], hc)
	}

	for host, cookies := range hostCookies {
		u := &url.URL{Scheme: "https", Host: host}
		jar.SetCookies(u, cookies)
	}

	return jar, nil
}

// FetchTokens makes an authenticated GET request to NotebookLM
// and extracts CSRF (SNlM0e) and session (FdrFJe) tokens from the page HTML.
func FetchTokens(cookieHeader string) (*Tokens, error) {
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookieHeader)

	client := &http.Client{
		// Don't follow redirects — detect auth failure
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching NotebookLM page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("authentication expired (HTTP %d) — re-login required", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	html := string(body)
	tokens := &Tokens{}

	// Extract SNlM0e (CSRF token)
	csrfRe := regexp.MustCompile(`"SNlM0e"\s*:\s*"([^"]+)"`)
	if m := csrfRe.FindStringSubmatch(html); len(m) > 1 {
		tokens.CSRF = m[1]
	} else {
		return nil, fmt.Errorf("CSRF token (SNlM0e) not found in page — auth may have expired")
	}

	// Extract FdrFJe (session ID) — optional
	sidRe := regexp.MustCompile(`"FdrFJe"\s*:\s*"([^"]+)"`)
	if m := sidRe.FindStringSubmatch(html); len(m) > 1 {
		tokens.SessionID = m[1]
	}

	return tokens, nil
}

// DefaultStoragePath returns the default path for the storage state file.
func DefaultStoragePath() string {
	if p := os.Getenv("NOTEBOOKLM_AUTH_JSON"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "storage_state.json"
	}
	return filepath.Join(home, ".notebooklm", "storage_state.json")
}

// isGoogleDomain checks if a cookie domain belongs to Google.
func isGoogleDomain(domain string) bool {
	d := strings.TrimPrefix(domain, ".")
	return d == "google.com" ||
		strings.HasSuffix(d, ".google.com") ||
		d == "googleusercontent.com" ||
		strings.HasSuffix(d, ".googleusercontent.com")
}
