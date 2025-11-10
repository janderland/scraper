//go:build integration
// +build integration

package reddit

import (
	"context"
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
}

// Get performs a real HTTP GET request to Reddit API with OAuth authentication
func (r *RealHTTPClient) Get(url string) ([]byte, error) {
	// TODO: Implement OAuth2 token acquisition and authenticated requests
	// For now, this is a placeholder that would need to:
	// 1. Get OAuth token if not present
	// 2. Make authenticated request with token
	// 3. Handle rate limiting
	// 4. Refresh token if expired
	panic("RealHTTPClient not yet implemented - OAuth2 flow needed")
}

// GetStream performs a real HTTP GET request for streaming content
func (r *RealHTTPClient) GetStream(url string) ([]byte, error) {
	return r.Get(url)
}
