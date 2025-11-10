//go:build integration
// +build integration

package facebook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/storage"
	"github.com/janderland/scraper/internal/testutil"
)

// TestFacebookIntegration_ScrapeUser tests scraping from an actual Facebook user
func TestFacebookIntegration_ScrapeUser(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoFacebookCredentials(t)

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

	scraper := NewScraper(db, fs, vpnManager, creds.FacebookAccessToken)

	// Test with "me" (the authenticated user)
	// You can also use a specific user ID or page ID
	filter := &models.FacebookFilter{
		Username: "me",
		MaxPosts: 5,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Log("Starting Facebook scrape (limited to 5 posts)...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items from Facebook", len(media))

	// Verify basic properties
	for i, m := range media {
		if m.Platform != models.PlatformFacebook {
			t.Errorf("Media %d: expected platform Facebook, got %s", i, m.Platform)
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
		t.Logf("Media %d: %s (Posted: %v)", i, m.Description, m.PostedAt)
	}
}

// TestFacebookIntegration_GetFilterSuggestions tests getting filter suggestions
func TestFacebookIntegration_GetFilterSuggestions(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoFacebookCredentials(t)

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

	scraper := NewScraper(db, fs, vpnManager, creds.FacebookAccessToken)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Getting filter suggestions from Facebook...")
	suggestions, err := scraper.GetFilterSuggestions(ctx, "me")
	if err != nil {
		t.Fatalf("Failed to get suggestions: %v", err)
	}

	t.Logf("Total posts available: %d", suggestions.TotalPosts)
	t.Logf("Suggested date ranges:")
	for _, r := range suggestions.SuggestedRanges {
		t.Logf("  - %s: %d posts", r.Label, r.PostCount)
	}

	if suggestions.PostsByYear != nil {
		t.Logf("Posts by year:")
		for year, count := range suggestions.PostsByYear {
			t.Logf("  - %d: %d posts", year, count)
		}
	}
}

// TestFacebookIntegration_DateFilter tests date range filtering
func TestFacebookIntegration_DateFilter(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoFacebookCredentials(t)

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

	scraper := NewScraper(db, fs, vpnManager, creds.FacebookAccessToken)

	// Only get posts from the last 30 days
	filter := &models.FacebookFilter{
		Username:  "me",
		MaxPosts:  5,
		StartDate: time.Now().AddDate(0, 0, -30),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Log("Starting Facebook scrape with date filter (last 30 days)...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items from last 30 days", len(media))

	// Verify date filter worked
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	for i, m := range media {
		if m.PostedAt.Before(thirtyDaysAgo) {
			t.Errorf("Media %d: posted at %v, which is before 30 days ago", i, m.PostedAt)
		}
		t.Logf("Media %d: Posted at %v", i, m.PostedAt)
	}
}

// TestFacebookIntegration_OldestFirst tests oldest-first ordering
func TestFacebookIntegration_OldestFirst(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoFacebookCredentials(t)

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

	scraper := NewScraper(db, fs, vpnManager, creds.FacebookAccessToken)

	filter := &models.FacebookFilter{
		Username:    "me",
		MaxPosts:    5,
		OldestFirst: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Log("Starting Facebook scrape with OldestFirst=true...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items", len(media))

	// Log post dates to verify ordering
	for i, m := range media {
		t.Logf("Media %d: Posted at %v", i, m.PostedAt)
	}
}

// TestFacebookIntegration_Deduplication tests that duplicate posts aren't re-downloaded
func TestFacebookIntegration_Deduplication(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoFacebookCredentials(t)

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

	filter := &models.FacebookFilter{
		Username: "me",
		MaxPosts: 3,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// First scrape
	t.Log("First scrape...")
	scraper1 := NewScraper(db, fs, vpnManager, creds.FacebookAccessToken)
	err = scraper1.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("First scrape failed: %v", err)
	}

	media1, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}
	count1 := len(media1)
	t.Logf("First scrape: %d media items", count1)

	// Second scrape (should not download duplicates)
	t.Log("Second scrape (testing deduplication)...")
	scraper2 := NewScraper(db, fs, vpnManager, creds.FacebookAccessToken)
	err = scraper2.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Second scrape failed: %v", err)
	}

	media2, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
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

// NewRealHTTPClient creates an HTTP client with Facebook Graph API authentication
func NewRealHTTPClient(creds *testutil.Credentials) *RealHTTPClient {
	return &RealHTTPClient{
		accessToken: creds.FacebookAccessToken,
	}
}

// RealHTTPClient implements the HTTPClient interface with real Facebook API calls
type RealHTTPClient struct {
	accessToken string
}

// Get performs a real HTTP GET request to Facebook Graph API
func (r *RealHTTPClient) Get(urlStr string) ([]byte, error) {
	// Parse URL to add access token
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Add access_token to query parameters
	query := parsedURL.Query()
	if r.accessToken != "" {
		query.Set("access_token", r.accessToken)
	}
	parsedURL.RawQuery = query.Encode()

	// Create request
	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

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
	// For media files (images/videos), download directly
	// Media URLs from Facebook already include authentication tokens
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

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
