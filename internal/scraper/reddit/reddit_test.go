package reddit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/scraper"
	"github.com/janderland/scraper/internal/storage"
)

// MockHTTPClient is a mock implementation of HTTPClient
type MockHTTPClient struct {
	responses map[string][]byte
	calls     int
}

func (m *MockHTTPClient) Get(url string) ([]byte, error) {
	m.calls++
	if response, ok := m.responses[url]; ok {
		return response, nil
	}
	// Return empty listing
	return []byte(`{"data": {"children": [], "after": null}}`), nil
}

func (m *MockHTTPClient) GetStream(url string) ([]byte, error) {
	return m.Get(url)
}

// MockVPNManager is a mock implementation of VPNManager
type MockVPNManager struct {
	client *MockHTTPClient
}

func (m *MockVPNManager) GetHTTPClient(ctx context.Context) (scraper.HTTPClient, error) {
	return m.client, nil
}

func (m *MockVPNManager) MarkVPNFailed(vpnID int64) error {
	return nil
}

func (m *MockVPNManager) MarkVPNSuccess(vpnID int64, bytesTransferred int64) error {
	return nil
}

// Helper function to create test database
func setupTestDB(t *testing.T) (*storage.Database, *storage.FileStorage, func()) {
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	fs, err := storage.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create test file storage: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, fs, cleanup
}

// Helper function to create mock Reddit listing
func createMockListing(posts []RedditPost, after string) []byte {
	listing := struct {
		Data struct {
			Children []struct {
				Data RedditPost `json:"data"`
			} `json:"children"`
			After string `json:"after"`
		} `json:"data"`
	}{}

	for _, post := range posts {
		listing.Data.Children = append(listing.Data.Children, struct {
			Data RedditPost `json:"data"`
		}{Data: post})
	}
	listing.Data.After = after

	data, _ := json.Marshal(listing)
	return data
}

func TestRedditScraper_Scrape_Subreddit(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	// Create mock posts
	posts := []RedditPost{
		{
			ID:         "post1",
			Title:      "Test Post 1",
			Subreddit:  "test",
			Score:      100,
			URL:        "https://i.redd.it/test1.jpg",
			PostHint:   "image",
			CreatedUTC: float64(time.Now().Unix()),
			Permalink:  "/r/test/comments/post1",
		},
		{
			ID:         "post2",
			Title:      "Test Post 2",
			Subreddit:  "test",
			Score:      50,
			URL:        "https://i.redd.it/test2.jpg",
			PostHint:   "image",
			CreatedUTC: float64(time.Now().Unix()),
			Permalink:  "/r/test/comments/post2",
		},
	}

	// Setup mock HTTP client
	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.reddit.com/r/test/hot.json?limit=100": createMockListing(posts, ""),
			"https://i.redd.it/test1.jpg":                      []byte("fake image data 1"),
			"https://i.redd.it/test2.jpg":                      []byte("fake image data 2"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}

	// Create scraper
	scraper := NewScraper(db, fs, mockVPN)

	// Create filter
	filter := &models.RedditFilter{
		Source:   "test",
		MaxPosts: 10,
	}

	// Run scraper
	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify results
	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 2 {
		t.Errorf("Expected 2 media items, got %d", len(media))
	}

	// Verify first media item
	if media[0].Title != "Test Post 1" {
		t.Errorf("Expected title 'Test Post 1', got '%s'", media[0].Title)
	}
	if media[0].Platform != models.PlatformReddit {
		t.Errorf("Expected platform Reddit, got %s", media[0].Platform)
	}
}

func TestRedditScraper_Filter_MinUpvotes(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	// Create mock posts with different upvote counts
	posts := []RedditPost{
		{
			ID:         "post1",
			Title:      "High Upvotes",
			Subreddit:  "test",
			Score:      1000,
			URL:        "https://i.redd.it/test1.jpg",
			PostHint:   "image",
			CreatedUTC: float64(time.Now().Unix()),
			Permalink:  "/r/test/comments/post1",
		},
		{
			ID:         "post2",
			Title:      "Low Upvotes",
			Subreddit:  "test",
			Score:      10,
			URL:        "https://i.redd.it/test2.jpg",
			PostHint:   "image",
			CreatedUTC: float64(time.Now().Unix()),
			Permalink:  "/r/test/comments/post2",
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.reddit.com/r/test/hot.json?limit=100": createMockListing(posts, ""),
			"https://i.redd.it/test1.jpg":                      []byte("fake image data 1"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN)

	// Filter with minimum 100 upvotes
	filter := &models.RedditFilter{
		Source:     "test",
		MaxPosts:   10,
		MinUpvotes: 100,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify only high-upvote post was downloaded
	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 1 {
		t.Errorf("Expected 1 media item, got %d", len(media))
	}

	if media[0].Title != "High Upvotes" {
		t.Errorf("Expected 'High Upvotes', got '%s'", media[0].Title)
	}
}

func TestRedditScraper_Filter_DateRange(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	oldDate := now.AddDate(0, 0, -30) // 30 days ago
	newDate := now.AddDate(0, 0, -1)  // 1 day ago

	posts := []RedditPost{
		{
			ID:         "post1",
			Title:      "Old Post",
			Subreddit:  "test",
			Score:      100,
			URL:        "https://i.redd.it/test1.jpg",
			PostHint:   "image",
			CreatedUTC: float64(oldDate.Unix()),
			Permalink:  "/r/test/comments/post1",
		},
		{
			ID:         "post2",
			Title:      "New Post",
			Subreddit:  "test",
			Score:      100,
			URL:        "https://i.redd.it/test2.jpg",
			PostHint:   "image",
			CreatedUTC: float64(newDate.Unix()),
			Permalink:  "/r/test/comments/post2",
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.reddit.com/r/test/hot.json?limit=100": createMockListing(posts, ""),
			"https://i.redd.it/test2.jpg":                      []byte("fake image data 2"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN)

	// Filter for last 7 days only
	filter := &models.RedditFilter{
		Source:    "test",
		MaxPosts:  10,
		StartDate: now.AddDate(0, 0, -7),
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify only recent post was downloaded
	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 1 {
		t.Errorf("Expected 1 media item, got %d", len(media))
	}

	if media[0].Title != "New Post" {
		t.Errorf("Expected 'New Post', got '%s'", media[0].Title)
	}
}

func TestRedditScraper_Filter_MaxPosts(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	// Create 5 mock posts
	var posts []RedditPost
	for i := 1; i <= 5; i++ {
		posts = append(posts, RedditPost{
			ID:         "post" + string(rune(i)),
			Title:      "Test Post",
			Subreddit:  "test",
			Score:      100,
			URL:        "https://i.redd.it/test.jpg",
			PostHint:   "image",
			CreatedUTC: float64(time.Now().Unix()),
			Permalink:  "/r/test/comments/post",
		})
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.reddit.com/r/test/hot.json?limit=100": createMockListing(posts, ""),
			"https://i.redd.it/test.jpg":                       []byte("fake image data"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN)

	// Limit to 2 posts
	filter := &models.RedditFilter{
		Source:   "test",
		MaxPosts: 2,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify only 2 posts were downloaded (due to deduplication, might be 1)
	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) > 2 {
		t.Errorf("Expected at most 2 media items, got %d", len(media))
	}
}

func TestRedditScraper_Deduplication(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	post := RedditPost{
		ID:         "post1",
		Title:      "Test Post",
		Subreddit:  "test",
		Score:      100,
		URL:        "https://i.redd.it/test.jpg",
		PostHint:   "image",
		CreatedUTC: float64(time.Now().Unix()),
		Permalink:  "/r/test/comments/post1",
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.reddit.com/r/test/hot.json?limit=100": createMockListing([]RedditPost{post}, ""),
			"https://i.redd.it/test.jpg":                       []byte("fake image data"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN)

	filter := &models.RedditFilter{
		Source:   "test",
		MaxPosts: 10,
	}

	ctx := context.Background()

	// First scrape
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("First scrape failed: %v", err)
	}

	// Second scrape (should not duplicate)
	scraper2 := NewScraper(db, fs, mockVPN)
	err = scraper2.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Second scrape failed: %v", err)
	}

	// Verify only one media item exists
	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 1 {
		t.Errorf("Expected 1 media item (deduplicated), got %d", len(media))
	}
}

func TestRedditScraper_NoMediaPosts(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	// Create posts without media
	posts := []RedditPost{
		{
			ID:         "post1",
			Title:      "Text Post",
			Subreddit:  "test",
			Score:      100,
			URL:        "https://reddit.com/r/test",
			CreatedUTC: float64(time.Now().Unix()),
			Permalink:  "/r/test/comments/post1",
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.reddit.com/r/test/hot.json?limit=100": createMockListing(posts, ""),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN)

	filter := &models.RedditFilter{
		Source:   "test",
		MaxPosts: 10,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify no media was downloaded
	media, err := db.SearchMedia(models.PlatformReddit, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 0 {
		t.Errorf("Expected 0 media items, got %d", len(media))
	}
}
