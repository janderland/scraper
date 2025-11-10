package testutil

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// Credentials holds all platform credentials for integration testing
type Credentials struct {
	// Reddit OAuth credentials
	RedditClientID     string
	RedditClientSecret string
	RedditUsername     string
	RedditPassword     string

	// Instagram session credentials
	InstagramSessionID string

	// Facebook Graph API credentials
	FacebookAccessToken string
}

// LoadCredentials loads credentials from environment variables
// and optionally from a .env.test file (if it exists)
func LoadCredentials(t *testing.T) *Credentials {
	t.Helper()

	// Try to load from .env.test file first (for local development)
	loadEnvFile(".env.test")

	creds := &Credentials{
		RedditClientID:      os.Getenv("REDDIT_CLIENT_ID"),
		RedditClientSecret:  os.Getenv("REDDIT_CLIENT_SECRET"),
		RedditUsername:      os.Getenv("REDDIT_USERNAME"),
		RedditPassword:      os.Getenv("REDDIT_PASSWORD"),
		InstagramSessionID:  os.Getenv("INSTAGRAM_SESSION_ID"),
		FacebookAccessToken: os.Getenv("FACEBOOK_ACCESS_TOKEN"),
	}

	return creds
}

// HasRedditCredentials returns true if Reddit credentials are configured
func (c *Credentials) HasRedditCredentials() bool {
	return c.RedditClientID != "" &&
		c.RedditClientSecret != "" &&
		c.RedditUsername != "" &&
		c.RedditPassword != ""
}

// HasInstagramCredentials returns true if Instagram credentials are configured
func (c *Credentials) HasInstagramCredentials() bool {
	return c.InstagramSessionID != ""
}

// HasFacebookCredentials returns true if Facebook credentials are configured
func (c *Credentials) HasFacebookCredentials() bool {
	return c.FacebookAccessToken != ""
}

// SkipIfNoRedditCredentials skips the test if Reddit credentials are not configured
func (c *Credentials) SkipIfNoRedditCredentials(t *testing.T) {
	t.Helper()
	if !c.HasRedditCredentials() {
		t.Skip("Skipping Reddit integration test: credentials not configured. Set REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, REDDIT_USERNAME, REDDIT_PASSWORD environment variables.")
	}
}

// SkipIfNoInstagramCredentials skips the test if Instagram credentials are not configured
func (c *Credentials) SkipIfNoInstagramCredentials(t *testing.T) {
	t.Helper()
	if !c.HasInstagramCredentials() {
		t.Skip("Skipping Instagram integration test: credentials not configured. Set INSTAGRAM_SESSION_ID environment variable.")
	}
}

// SkipIfNoFacebookCredentials skips the test if Facebook credentials are not configured
func (c *Credentials) SkipIfNoFacebookCredentials(t *testing.T) {
	t.Helper()
	if !c.HasFacebookCredentials() {
		t.Skip("Skipping Facebook integration test: credentials not configured. Set FACEBOOK_ACCESS_TOKEN environment variable.")
	}
}

// loadEnvFile loads environment variables from a .env file if it exists
// This is a simple implementation that doesn't handle complex cases
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// File doesn't exist, that's fine
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		// Only set if not already in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
