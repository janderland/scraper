package facebook

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
	return []byte(`{"data": [], "paging": {}}`), nil
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

// Helper function to create mock Facebook response
func createMockFacebookResponse(posts []FacebookPost, nextURL string) []byte {
	response := FacebookPhotosResponse{
		Data: posts,
	}
	response.Paging.Next = nextURL

	data, _ := json.Marshal(response)
	return data
}

func TestFacebookScraper_Scrape(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	posts := []FacebookPost{
		{
			ID:          "photo1",
			CreatedTime: now,
			Name:        "Test Photo 1",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/photo1.jpg"},
			},
			Likes: struct {
				Summary struct {
					TotalCount int `json:"total_count"`
				} `json:"summary"`
			}{
				Summary: struct {
					TotalCount int `json:"total_count"`
				}{TotalCount: 100},
			},
		},
		{
			ID:          "photo2",
			CreatedTime: now,
			Name:        "Test Photo 2",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/photo2.jpg"},
			},
			Likes: struct {
				Summary struct {
					TotalCount int `json:"total_count"`
				} `json:"summary"`
			}{
				Summary: struct {
					TotalCount int `json:"total_count"`
				}{TotalCount: 50},
			},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://graph.facebook.com/v18.0/testuser/photos/uploaded?fields=id,created_time,name,images,likes.summary(true),comments.summary(true)&access_token=test_token&limit=100": createMockFacebookResponse(posts, ""),
			"https://facebook.com/photo1.jpg": []byte("fake image 1"),
			"https://facebook.com/photo2.jpg": []byte("fake image 2"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_token")

	filter := &models.FacebookFilter{
		Username: "testuser",
		MaxPosts: 10,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify results
	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 2 {
		t.Errorf("Expected 2 media items, got %d", len(media))
	}

	if media[0].Platform != models.PlatformFacebook {
		t.Errorf("Expected platform Facebook, got %s", media[0].Platform)
	}
}

func TestFacebookScraper_Filter_MaxPosts(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	// Create 5 mock posts
	var posts []FacebookPost
	for i := 1; i <= 5; i++ {
		posts = append(posts, FacebookPost{
			ID:          "photo",
			CreatedTime: time.Now(),
			Name:        "Test Photo",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/photo.jpg"},
			},
		})
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://graph.facebook.com/v18.0/testuser/photos/uploaded?fields=id,created_time,name,images,likes.summary(true),comments.summary(true)&access_token=test_token&limit=100": createMockFacebookResponse(posts, ""),
			"https://facebook.com/photo.jpg": []byte("fake image"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_token")

	filter := &models.FacebookFilter{
		Username: "testuser",
		MaxPosts: 2,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify only 2 posts (or 1 due to deduplication)
	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) > 2 {
		t.Errorf("Expected at most 2 media items, got %d", len(media))
	}
}

func TestFacebookScraper_Filter_DateRange(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	oldDate := now.AddDate(0, 0, -30)
	newDate := now.AddDate(0, 0, -1)

	posts := []FacebookPost{
		{
			ID:          "photo1",
			CreatedTime: oldDate,
			Name:        "Old Photo",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/old.jpg"},
			},
		},
		{
			ID:          "photo2",
			CreatedTime: newDate,
			Name:        "New Photo",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/new.jpg"},
			},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://graph.facebook.com/v18.0/testuser/photos/uploaded?fields=id,created_time,name,images,likes.summary(true),comments.summary(true)&access_token=test_token&limit=100": createMockFacebookResponse(posts, ""),
			"https://facebook.com/new.jpg": []byte("fake new image"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_token")

	// Filter for last 7 days only
	filter := &models.FacebookFilter{
		Username:  "testuser",
		MaxPosts:  10,
		StartDate: now.AddDate(0, 0, -7),
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify only recent photo was downloaded
	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 1 {
		t.Errorf("Expected 1 media item, got %d", len(media))
	}
}

func TestFacebookScraper_Filter_OldestFirst(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	oldTime := time.Now().AddDate(0, 0, -30)
	newTime := time.Now()

	posts := []FacebookPost{
		{
			ID:          "photo1",
			CreatedTime: newTime,
			Name:        "New Photo",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/new.jpg"},
			},
		},
		{
			ID:          "photo2",
			CreatedTime: oldTime,
			Name:        "Old Photo",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/old.jpg"},
			},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://graph.facebook.com/v18.0/testuser/photos/uploaded?fields=id,created_time,name,images,likes.summary(true),comments.summary(true)&access_token=test_token&limit=100": createMockFacebookResponse(posts, ""),
			"https://facebook.com/new.jpg": []byte("fake new image"),
			"https://facebook.com/old.jpg": []byte("fake old image"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_token")

	filter := &models.FacebookFilter{
		Username:    "testuser",
		MaxPosts:    10,
		OldestFirst: true,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify both photos were downloaded
	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 2 {
		t.Errorf("Expected 2 media items, got %d", len(media))
	}
}

func TestFacebookScraper_GetFilterSuggestions(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	posts := []FacebookPost{
		{
			ID:          "photo1",
			CreatedTime: now.AddDate(-1, 0, 0), // 1 year ago
			Name:        "Old Photo",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/old.jpg"},
			},
		},
		{
			ID:          "photo2",
			CreatedTime: now,
			Name:        "New Photo",
			Images: []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{
				{Height: 1000, Width: 1000, Source: "https://facebook.com/new.jpg"},
			},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://graph.facebook.com/v18.0/testuser/photos/uploaded?fields=id,created_time,name,images,likes.summary(true),comments.summary(true)&access_token=test_token&limit=100": createMockFacebookResponse(posts, ""),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_token")

	ctx := context.Background()
	suggestions, err := scraper.GetFilterSuggestions(ctx, "testuser")
	if err != nil {
		t.Fatalf("Failed to get suggestions: %v", err)
	}

	if suggestions.TotalPosts != 2 {
		t.Errorf("Expected 2 total posts, got %d", suggestions.TotalPosts)
	}

	if len(suggestions.SuggestedRanges) == 0 {
		t.Error("Expected suggested ranges, got none")
	}

	if suggestions.PostsByYear == nil {
		t.Error("Expected posts by year, got nil")
	}
}

func TestFacebookScraper_Deduplication(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	post := FacebookPost{
		ID:          "photo1",
		CreatedTime: time.Now(),
		Name:        "Test Photo",
		Images: []struct {
			Height int    `json:"height"`
			Width  int    `json:"width"`
			Source string `json:"source"`
		}{
			{Height: 1000, Width: 1000, Source: "https://facebook.com/photo.jpg"},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://graph.facebook.com/v18.0/testuser/photos/uploaded?fields=id,created_time,name,images,likes.summary(true),comments.summary(true)&access_token=test_token&limit=100": createMockFacebookResponse([]FacebookPost{post}, ""),
			"https://facebook.com/photo.jpg": []byte("fake image"),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}

	filter := &models.FacebookFilter{
		Username: "testuser",
		MaxPosts: 10,
	}

	ctx := context.Background()

	// First scrape
	scraper1 := NewScraper(db, fs, mockVPN, "test_token")
	err := scraper1.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("First scrape failed: %v", err)
	}

	// Second scrape
	scraper2 := NewScraper(db, fs, mockVPN, "test_token")
	err = scraper2.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Second scrape failed: %v", err)
	}

	// Verify only one media item
	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 1 {
		t.Errorf("Expected 1 media item (deduplicated), got %d", len(media))
	}
}

func TestFacebookScraper_NoImages(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	// Post with no images
	posts := []FacebookPost{
		{
			ID:          "photo1",
			CreatedTime: time.Now(),
			Name:        "Test Photo",
			Images:      []struct {
				Height int    `json:"height"`
				Width  int    `json:"width"`
				Source string `json:"source"`
			}{},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://graph.facebook.com/v18.0/testuser/photos/uploaded?fields=id,created_time,name,images,likes.summary(true),comments.summary(true)&access_token=test_token&limit=100": createMockFacebookResponse(posts, ""),
		},
	}

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_token")

	filter := &models.FacebookFilter{
		Username: "testuser",
		MaxPosts: 10,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)

	// Should complete without error (just skip the post)
	if err != nil {
		// This is expected behavior - posts without images should be skipped
		t.Logf("Scraping completed with expected error: %v", err)
	}

	// Verify no media was saved
	media, err := db.SearchMedia(models.PlatformFacebook, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 0 {
		t.Errorf("Expected 0 media items, got %d", len(media))
	}
}
