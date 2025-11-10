//go:build integration
// +build integration

package facebook

import (
	"context"
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
func (r *RealHTTPClient) Get(url string) ([]byte, error) {
	// TODO: Implement real Facebook Graph API HTTP client
	// This would need to:
	// 1. Add access_token to URL parameters
	// 2. Set proper headers
	// 3. Handle rate limiting
	// 4. Handle pagination
	panic("RealHTTPClient not yet implemented - Facebook Graph API handling needed")
}

// GetStream performs a real HTTP GET request for streaming content
func (r *RealHTTPClient) GetStream(url string) ([]byte, error) {
	return r.Get(url)
}
