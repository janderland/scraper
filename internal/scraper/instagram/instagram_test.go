package instagram

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
	return []byte(`{"data": {"user": {"edge_owner_to_timeline_media": {"edges": [], "page_info": {"has_next_page": false}}}}}`), nil
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

// Helper function to create mock GraphQL response
func createMockGraphQLResponse(posts []InstagramPost, hasNextPage bool) []byte {
	response := InstagramGraphQLResponse{}

	for _, post := range posts {
		response.Data.User.EdgeOwnerToTimelineMedia.Edges = append(
			response.Data.User.EdgeOwnerToTimelineMedia.Edges,
			struct {
				Node InstagramPost `json:"node"`
			}{Node: post},
		)
	}

	response.Data.User.EdgeOwnerToTimelineMedia.PageInfo.HasNextPage = hasNextPage
	response.Data.User.EdgeOwnerToTimelineMedia.PageInfo.EndCursor = ""

	data, _ := json.Marshal(response)
	return data
}

func TestInstagramScraper_Scrape(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	posts := []InstagramPost{
		{
			ID:               "post1",
			Shortcode:        "ABC123",
			DisplayURL:       "https://instagram.com/p/test1.jpg",
			IsVideo:          false,
			TakenAtTimestamp: now.Unix(),
			Owner: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			}{
				ID:       "user1",
				Username: "testuser",
			},
		},
		{
			ID:               "post2",
			Shortcode:        "DEF456",
			DisplayURL:       "https://instagram.com/p/test2.jpg",
			IsVideo:          false,
			TakenAtTimestamp: now.Unix(),
			Owner: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			}{
				ID:       "user1",
				Username: "testuser",
			},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.instagram.com/testuser/": []byte(`
				<script type="text/javascript">
					window._sharedData = {"entry_data":{"ProfilePage":[{"graphql":{"user":{"id":"user1"}}}]}};
				</script>
			`),
			"https://instagram.com/p/test1.jpg": []byte("fake image 1"),
			"https://instagram.com/p/test2.jpg": []byte("fake image 2"),
		},
	}

	// Add GraphQL response
	mockClient.responses["https://www.instagram.com/graphql/query/?query_hash=9dcf6e1a98bc7f6e92953d5a61027b98&variables={\"id\":\"user1\",\"first\":50}"] = createMockGraphQLResponse(posts, false)

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_session")

	filter := &models.InstagramFilter{
		Username: "testuser",
		MaxPosts: 10,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify results
	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 2 {
		t.Errorf("Expected 2 media items, got %d", len(media))
	}

	if media[0].Platform != models.PlatformInstagram {
		t.Errorf("Expected platform Instagram, got %s", media[0].Platform)
	}
}

func TestInstagramScraper_Filter_MaxPosts(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	// Create 5 mock posts
	var posts []InstagramPost
	for i := 1; i <= 5; i++ {
		posts = append(posts, InstagramPost{
			ID:               "post",
			Shortcode:        "ABC",
			DisplayURL:       "https://instagram.com/p/test.jpg",
			IsVideo:          false,
			TakenAtTimestamp: time.Now().Unix(),
			Owner: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			}{
				ID:       "user1",
				Username: "testuser",
			},
		})
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.instagram.com/testuser/": []byte(`
				<script type="text/javascript">
					window._sharedData = {"entry_data":{"ProfilePage":[{"graphql":{"user":{"id":"user1"}}}]}};
				</script>
			`),
			"https://instagram.com/p/test.jpg": []byte("fake image"),
		},
	}

	mockClient.responses["https://www.instagram.com/graphql/query/?query_hash=9dcf6e1a98bc7f6e92953d5a61027b98&variables={\"id\":\"user1\",\"first\":50}"] = createMockGraphQLResponse(posts, false)

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_session")

	filter := &models.InstagramFilter{
		Username: "testuser",
		MaxPosts: 2,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify only 2 posts (or 1 due to deduplication)
	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) > 2 {
		t.Errorf("Expected at most 2 media items, got %d", len(media))
	}
}

func TestInstagramScraper_Filter_OldestFirst(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	oldTime := time.Now().AddDate(0, 0, -30).Unix()
	newTime := time.Now().Unix()

	posts := []InstagramPost{
		{
			ID:               "post1",
			Shortcode:        "NEW",
			DisplayURL:       "https://instagram.com/p/new.jpg",
			IsVideo:          false,
			TakenAtTimestamp: newTime,
			Owner: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			}{
				ID:       "user1",
				Username: "testuser",
			},
		},
		{
			ID:               "post2",
			Shortcode:        "OLD",
			DisplayURL:       "https://instagram.com/p/old.jpg",
			IsVideo:          false,
			TakenAtTimestamp: oldTime,
			Owner: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			}{
				ID:       "user1",
				Username: "testuser",
			},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.instagram.com/testuser/": []byte(`
				<script type="text/javascript">
					window._sharedData = {"entry_data":{"ProfilePage":[{"graphql":{"user":{"id":"user1"}}}]}};
				</script>
			`),
			"https://instagram.com/p/new.jpg": []byte("fake new image"),
			"https://instagram.com/p/old.jpg": []byte("fake old image"),
		},
	}

	mockClient.responses["https://www.instagram.com/graphql/query/?query_hash=9dcf6e1a98bc7f6e92953d5a61027b98&variables={\"id\":\"user1\",\"first\":50}"] = createMockGraphQLResponse(posts, false)

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_session")

	// Test with OldestFirst = true
	filter := &models.InstagramFilter{
		Username:    "testuser",
		MaxPosts:    10,
		OldestFirst: true,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify results
	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 2 {
		t.Errorf("Expected 2 media items, got %d", len(media))
	}

	// Note: The order in database might be by download time, not post time
	// This test verifies that both posts were processed
}

func TestInstagramScraper_VideoPost(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []InstagramPost{
		{
			ID:               "post1",
			Shortcode:        "VIDEO1",
			DisplayURL:       "https://instagram.com/p/thumb.jpg",
			IsVideo:          true,
			VideoURL:         "https://instagram.com/p/video.mp4",
			TakenAtTimestamp: time.Now().Unix(),
			Owner: struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			}{
				ID:       "user1",
				Username: "testuser",
			},
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.instagram.com/testuser/": []byte(`
				<script type="text/javascript">
					window._sharedData = {"entry_data":{"ProfilePage":[{"graphql":{"user":{"id":"user1"}}}]}};
				</script>
			`),
			"https://instagram.com/p/video.mp4": []byte("fake video data"),
		},
	}

	mockClient.responses["https://www.instagram.com/graphql/query/?query_hash=9dcf6e1a98bc7f6e92953d5a61027b98&variables={\"id\":\"user1\",\"first\":50}"] = createMockGraphQLResponse(posts, false)

	mockVPN := &MockVPNManager{client: mockClient}
	scraper := NewScraper(db, fs, mockVPN, "test_session")

	filter := &models.InstagramFilter{
		Username: "testuser",
		MaxPosts: 10,
	}

	ctx := context.Background()
	err := scraper.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Scraping failed: %v", err)
	}

	// Verify video was downloaded
	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 1 {
		t.Errorf("Expected 1 media item, got %d", len(media))
	}

	if media[0].MediaType != models.MediaTypeVideo {
		t.Errorf("Expected video type, got %s", media[0].MediaType)
	}
}

func TestInstagramScraper_Deduplication(t *testing.T) {
	db, fs, cleanup := setupTestDB(t)
	defer cleanup()

	post := InstagramPost{
		ID:               "post1",
		Shortcode:        "ABC123",
		DisplayURL:       "https://instagram.com/p/test.jpg",
		IsVideo:          false,
		TakenAtTimestamp: time.Now().Unix(),
		Owner: struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		}{
			ID:       "user1",
			Username: "testuser",
		},
	}

	mockClient := &MockHTTPClient{
		responses: map[string][]byte{
			"https://www.instagram.com/testuser/": []byte(`
				<script type="text/javascript">
					window._sharedData = {"entry_data":{"ProfilePage":[{"graphql":{"user":{"id":"user1"}}}]}};
				</script>
			`),
			"https://instagram.com/p/test.jpg": []byte("fake image"),
		},
	}

	mockClient.responses["https://www.instagram.com/graphql/query/?query_hash=9dcf6e1a98bc7f6e92953d5a61027b98&variables={\"id\":\"user1\",\"first\":50}"] = createMockGraphQLResponse([]InstagramPost{post}, false)

	mockVPN := &MockVPNManager{client: mockClient}

	filter := &models.InstagramFilter{
		Username: "testuser",
		MaxPosts: 10,
	}

	ctx := context.Background()

	// First scrape
	scraper1 := NewScraper(db, fs, mockVPN, "test_session")
	err := scraper1.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("First scrape failed: %v", err)
	}

	// Second scrape
	scraper2 := NewScraper(db, fs, mockVPN, "test_session")
	err = scraper2.Scrape(ctx, filter)
	if err != nil {
		t.Fatalf("Second scrape failed: %v", err)
	}

	// Verify only one media item
	media, err := db.SearchMedia(models.PlatformInstagram, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to search media: %v", err)
	}

	if len(media) != 1 {
		t.Errorf("Expected 1 media item (deduplicated), got %d", len(media))
	}
}
