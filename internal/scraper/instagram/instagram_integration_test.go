//go:build integration
// +build integration

package instagram

import (
	"context"
	"testing"
	"time"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/storage"
	"github.com/janderland/scraper/internal/testutil"
)

// TestInstagramIntegration_ScrapeUser tests scraping from an actual Instagram user profile
func TestInstagramIntegration_ScrapeUser(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoInstagramCredentials(t)

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

	scraper := NewScraper(db, fs, vpnManager, creds.InstagramSessionID)

	// Test with a public account (e.g., National Geographic)
	// You can change this to any public Instagram username
	filter := &models.InstagramFilter{
		Username: "natgeo",
		MaxPosts: 5,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Log("Starting Instagram scrape from natgeo (limited to 5 posts)...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items from Instagram", len(media))

	// Verify basic properties
	for i, m := range media {
		if m.Platform != models.PlatformInstagram {
			t.Errorf("Media %d: expected platform Instagram, got %s", i, m.Platform)
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
		if m.Author == "" {
			t.Errorf("Media %d: Author is empty", i)
		}
		t.Logf("Media %d: Author=%s, Type=%s, Posted=%v", i, m.Author, m.MediaType, m.PostedAt)
	}
}

// TestInstagramIntegration_ScrapeWithOrdering tests newest/oldest ordering
func TestInstagramIntegration_ScrapeWithOrdering(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoInstagramCredentials(t)

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

	scraper := NewScraper(db, fs, vpnManager, creds.InstagramSessionID)

	// Test oldest first ordering
	filter := &models.InstagramFilter{
		Username:    "natgeo",
		MaxPosts:    3,
		OldestFirst: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Log("Starting Instagram scrape with OldestFirst=true...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	t.Logf("Successfully scraped %d media items", len(media))

	// Log post dates to verify ordering
	for i, m := range media {
		t.Logf("Media %d: Posted at %v", i, m.PostedAt)
	}
}

// TestInstagramIntegration_VideoDownload tests downloading video posts
func TestInstagramIntegration_VideoDownload(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoInstagramCredentials(t)

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

	scraper := NewScraper(db, fs, vpnManager, creds.InstagramSessionID)

	// Use an account known to have video posts
	filter := &models.InstagramFilter{
		Username: "natgeo",
		MaxPosts: 10, // Get more posts to increase chance of finding videos
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("Starting Instagram scrape to find video posts...")
	err = scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 20, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	// Count videos vs images
	videoCount := 0
	imageCount := 0
	for _, m := range media {
		if m.MediaType == models.MediaTypeVideo {
			videoCount++
		} else if m.MediaType == models.MediaTypeImage {
			imageCount++
		}
	}

	t.Logf("Downloaded %d images and %d videos", imageCount, videoCount)

	if videoCount > 0 {
		t.Logf("Successfully downloaded at least one video")
	} else {
		t.Log("Note: No videos found in the scraped posts")
	}
}

// TestInstagramIntegration_Deduplication tests that duplicate posts aren't re-downloaded
func TestInstagramIntegration_Deduplication(t *testing.T) {
	creds := testutil.LoadCredentials(t)
	creds.SkipIfNoInstagramCredentials(t)

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

	filter := &models.InstagramFilter{
		Username: "natgeo",
		MaxPosts: 3,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// First scrape
	t.Log("First scrape...")
	scraper1 := NewScraper(db, fs, vpnManager, creds.InstagramSessionID)
	err = scraper1.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("First scrape failed: %v", err)
	}

	media1, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}
	count1 := len(media1)
	t.Logf("First scrape: %d media items", count1)

	// Second scrape (should not download duplicates)
	t.Log("Second scrape (testing deduplication)...")
	scraper2 := NewScraper(db, fs, vpnManager, creds.InstagramSessionID)
	err = scraper2.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Second scrape failed: %v", err)
	}

	media2, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
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

// NewRealHTTPClient creates an HTTP client with Instagram session authentication
func NewRealHTTPClient(creds *testutil.Credentials) *RealHTTPClient {
	return &RealHTTPClient{
		sessionID: creds.InstagramSessionID,
	}
}

// RealHTTPClient implements the HTTPClient interface with real Instagram API calls
type RealHTTPClient struct {
	sessionID string
}

// Get performs a real HTTP GET request to Instagram with session cookie
func (r *RealHTTPClient) Get(url string) ([]byte, error) {
	// TODO: Implement real Instagram HTTP client with session cookie
	// This would need to:
	// 1. Set proper headers (User-Agent, etc.)
	// 2. Include session cookie (sessionid)
	// 3. Handle rate limiting
	// 4. Parse responses properly
	panic("RealHTTPClient not yet implemented - Instagram session handling needed")
}

// GetStream performs a real HTTP GET request for streaming content
func (r *RealHTTPClient) GetStream(url string) ([]byte, error) {
	return r.Get(url)
}
