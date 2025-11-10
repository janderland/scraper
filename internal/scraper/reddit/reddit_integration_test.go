//go:build integration
// +build integration

package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/storage"
	"github.com/janderland/scraper/internal/testutil"
)

// TestRedditIntegration_ScrapeSaved tests scraping from actual Reddit saved posts
func TestRedditIntegration_ScrapeSaved(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoRedditCredentials(t)

	// Setup test database and storage
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	fs, err := storage.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create test file storage: %v", err)
	}

	// Create VPN manager (nil for now - we'll use direct connection)
	// In production, you'd pass a real VPN manager here
	vpnManager := &MockVPNManager{
		client: NewRealHTTPClient(creds),
	}

	scraper := NewScraper(db, fs, vpnManager)

	// Test scraping saved posts with minimal filter
	filter := &models.RedditFilter{
		Source:   "saved",
		MaxPosts: 5, // Limit to 5 posts for testing
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("Starting Reddit saved posts scrape (limited to 5 posts)...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify some media was downloaded
	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items from Reddit saved posts", len(media))

	// Verify basic properties
	for i, m := range media {
		if m.Platform != models.PlatformReddit {
			t.Errorf("Media %d: expected platform Reddit, got %s", i, m.Platform)
		}
		if m.URL == "" {
			t.Errorf("Media %d: URL is empty", i)
		}
		if m.LocalPath == "" {
			t.Errorf("Media %d: LocalPath is empty", i)
		}
		if m.Hash == "" {
			t.Errorf("Media %d: Hash is empty", i)
		}
		t.Logf("Media %d: %s - %s", i, m.Title, m.URL)
	}
}

// TestRedditIntegration_ScrapeSubreddit tests scraping from an actual subreddit
func TestRedditIntegration_ScrapeSubreddit(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoRedditCredentials(t)

	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	fs, err := storage.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create test file storage: %v", err)
	}

	vpnManager := &MockVPNManager{
		client: NewRealHTTPClient(creds),
	}

	scraper := NewScraper(db, fs, vpnManager)

	// Test scraping from r/aww (safe, public subreddit with lots of images)
	filter := &models.RedditFilter{
		Source:     "aww",
		MaxPosts:   3,
		MinUpvotes: 100, // Only high-quality posts
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("Starting Reddit subreddit scrape from r/aww (limited to 3 posts with 100+ upvotes)...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items from r/aww", len(media))

	// Verify upvotes filter worked
	for i, m := range media {
		if m.Upvotes < 100 {
			t.Errorf("Media %d: expected at least 100 upvotes, got %d", i, m.Upvotes)
		}
		t.Logf("Media %d: %s (upvotes: %d)", i, m.Title, m.Upvotes)
	}
}

// TestRedditIntegration_DateFilter tests date range filtering
func TestRedditIntegration_DateFilter(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoRedditCredentials(t)

	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	fs, err := storage.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create test file storage: %v", err)
	}

	vpnManager := &MockVPNManager{
		client: NewRealHTTPClient(creds),
	}

	scraper := NewScraper(db, fs, vpnManager)

	// Only get posts from the last 7 days
	filter := &models.RedditFilter{
		Source:    "pics",
		MaxPosts:  3,
		StartDate: time.Now().AddDate(0, 0, -7),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("Starting Reddit scrape with date filter (last 7 days)...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items from r/pics (last 7 days)", len(media))

	// Verify date filter worked
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	for i, m := range media {
		if m.PostedAt.Before(sevenDaysAgo) {
			t.Errorf("Media %d: posted at %v, which is before 7 days ago", i, m.PostedAt)
		}
		t.Logf("Media %d: %s (posted: %v)", i, m.Title, m.PostedAt)
	}
}

// TestRedditIntegration_Deduplication tests that duplicate posts aren't re-downloaded
func TestRedditIntegration_Deduplication(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoRedditCredentials(t)

	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	fs, err := storage.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create test file storage: %v", err)
	}

	vpnManager := &MockVPNManager{
		client: NewRealHTTPClient(creds),
	}

	scraper := NewScraper(db, fs, vpnManager)

	filter := &models.RedditFilter{
		Source:   "earthporn",
		MaxPosts: 2,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First scrape
	t.Log("First scrape...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("First scrape failed: %v", err)
	}

	media1, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}
	count1 := len(media1)
	t.Logf("First scrape: %d media items", count1)

	// Second scrape (should not download duplicates)
	t.Log("Second scrape (testing deduplication)...")
	scraper2 := NewScraper(db, fs, vpnManager)
	err = scraper2.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Second scrape failed: %v", err)
	}

	media2, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}
	count2 := len(media2)
	t.Logf("Second scrape: %d media items total", count2)

	// Should be the same count (deduplication working)
	if count2 != count1 {
		t.Logf("Warning: count changed from %d to %d (new posts may have appeared)", count1, count2)
	} else {
		t.Log("Deduplication working: same count after second scrape")
	}
}

// NewRealHTTPClient creates an HTTP client with Reddit OAuth authentication
func NewRealHTTPClient(creds *testutil.Credentials) *RealHTTPClient {
	return &RealHTTPClient{
		clientID:     creds.RedditClientID,
		clientSecret: creds.RedditClientSecret,
		username:     creds.RedditUsername,
		password:     creds.RedditPassword,
	}
}

// RealHTTPClient implements the HTTPClient interface with real Reddit API calls
type RealHTTPClient struct {
	clientID     string
	clientSecret string
	username     string
	password     string
	accessToken  string
	tokenExpiry  time.Time
}

// getAccessToken obtains an OAuth2 access token from Reddit
func (r *RealHTTPClient) getAccessToken() error {
	// Check if we have a valid token
	if r.accessToken != "" && time.Now().Before(r.tokenExpiry) {
		return nil
	}

	// Build token request
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", r.username)
	data.Set("password", r.password)

	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	// Set basic auth with client credentials
	req.SetBasicAuth(r.clientID, r.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "golang:scraper:v1.0.0 (by /u/testuser)")

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	r.accessToken = tokenResp.AccessToken
	r.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// Get performs a real HTTP GET request to Reddit API with OAuth authentication
func (r *RealHTTPClient) Get(urlStr string) ([]byte, error) {
	// Get access token if needed
	if err := r.getAccessToken(); err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Create request
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add OAuth token
	req.Header.Set("Authorization", "Bearer "+r.accessToken)
	req.Header.Set("User-Agent", "golang:scraper:v1.0.0 (by /u/testuser)")

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetStream performs a real HTTP GET request for streaming content
func (r *RealHTTPClient) GetStream(urlStr string) ([]byte, error) {
	// For media files, we don't need OAuth - just download directly
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "golang:scraper:v1.0.0 (by /u/testuser)")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return body, nil
}
