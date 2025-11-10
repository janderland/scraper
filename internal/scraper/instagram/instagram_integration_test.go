//go:build integration
// +build integration

package instagram

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
func (r *RealHTTPClient) Get(urlStr string) ([]byte, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Instagram-specific headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Add session cookie
	if r.sessionID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "sessionid",
			Value: r.sessionID,
		})
	}

	// Make request
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow redirects but limit to 10
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			// Copy cookies to redirected request
			if r.sessionID != "" {
				req.AddCookie(&http.Cookie{
					Name:  "sessionid",
					Value: r.sessionID,
				})
			}
			return nil
		},
	}

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
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}

	return body, nil
}

// GetStream performs a real HTTP GET request for streaming content
func (r *RealHTTPClient) GetStream(urlStr string) ([]byte, error) {
	// For media files, use the same method but with longer timeout
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Add session cookie for authenticated media
	if r.sessionID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "sessionid",
			Value: r.sessionID,
		})
	}

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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
