package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/scraper"
	"github.com/janderland/scraper/internal/storage"
)

// Scraper implements Reddit scraping functionality
type Scraper struct {
	*scraper.BaseScraper
}

// NewScraper creates a new Reddit scraper
func NewScraper(db *storage.Database, fs *storage.FileStorage, vpn scraper.VPNManager) *Scraper {
	return &Scraper{
		BaseScraper: scraper.NewBaseScraper(db, fs, vpn),
	}
}

// GetPlatform returns the platform
func (s *Scraper) GetPlatform() models.Platform {
	return models.PlatformReddit
}

// Scrape performs Reddit scraping
func (s *Scraper) Scrape(ctx context.Context, filterInterface interface{}) error {
	filter, ok := filterInterface.(*models.RedditFilter)
	if !ok {
		return fmt.Errorf("invalid filter type for Reddit scraper")
	}

	if filter.Source == "saved" {
		return s.scrapeSaved(ctx, filter)
	}
	return s.scrapeSubreddit(ctx, filter)
}

// scrapeSaved scrapes a user's saved posts
func (s *Scraper) scrapeSaved(ctx context.Context, filter *models.RedditFilter) error {
	// Reddit requires OAuth for saved posts
	// This is a simplified implementation
	client, err := s.BaseScraper.GetHTTPClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get HTTP client: %w", err)
	}

	after := ""
	count := 0
	maxPosts := filter.MaxPosts
	if maxPosts == 0 {
		maxPosts = 1000 // Default maximum
	}

	for count < maxPosts && !s.IsStopped() {
		// Build URL for Reddit API
		apiURL := fmt.Sprintf("https://oauth.reddit.com/user/%s/saved.json?limit=100", filter.Username)
		if after != "" {
			apiURL += "&after=" + after
		}

		data, err := client.Get(apiURL)
		if err != nil {
			return fmt.Errorf("failed to fetch saved posts: %w", err)
		}

		listing, err := parseRedditListing(data)
		if err != nil {
			return fmt.Errorf("failed to parse listing: %w", err)
		}

		if len(listing.Data.Children) == 0 {
			break // No more posts
		}

		for _, child := range listing.Data.Children {
			if count >= maxPosts {
				break
			}

			post := child.Data
			if !s.shouldProcessPost(post, filter) {
				continue
			}

			if err := s.processPost(ctx, post); err != nil {
				fmt.Printf("Error processing post %s: %v\n", post.ID, err)
				continue
			}

			count++
			s.SetProgress(int(float64(count) / float64(maxPosts) * 100))
		}

		after = listing.Data.After
		if after == "" {
			break
		}

		// Rate limiting
		time.Sleep(2 * time.Second)
	}

	s.SetProgress(100)
	return nil
}

// scrapeSubreddit scrapes posts from a subreddit
func (s *Scraper) scrapeSubreddit(ctx context.Context, filter *models.RedditFilter) error {
	client, err := s.BaseScraper.GetHTTPClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get HTTP client: %w", err)
	}

	after := ""
	count := 0
	maxPosts := filter.MaxPosts
	if maxPosts == 0 {
		maxPosts = 1000
	}

	for count < maxPosts && !s.IsStopped() {
		// Build URL for Reddit JSON API (no auth required)
		apiURL := fmt.Sprintf("https://www.reddit.com/r/%s/hot.json?limit=100", filter.Source)
		if after != "" {
			apiURL += "&after=" + after
		}

		data, err := client.Get(apiURL)
		if err != nil {
			return fmt.Errorf("failed to fetch subreddit posts: %w", err)
		}

		listing, err := parseRedditListing(data)
		if err != nil {
			return fmt.Errorf("failed to parse listing: %w", err)
		}

		if len(listing.Data.Children) == 0 {
			break
		}

		for _, child := range listing.Data.Children {
			if count >= maxPosts {
				break
			}

			post := child.Data
			if !s.shouldProcessPost(post, filter) {
				continue
			}

			if err := s.processPost(ctx, post); err != nil {
				fmt.Printf("Error processing post %s: %v\n", post.ID, err)
				continue
			}

			count++
			s.SetProgress(int(float64(count) / float64(maxPosts) * 100))
		}

		after = listing.Data.After
		if after == "" {
			break
		}

		time.Sleep(2 * time.Second)
	}

	s.SetProgress(100)
	return nil
}

// shouldProcessPost checks if a post should be processed based on filters
func (s *Scraper) shouldProcessPost(post *RedditPost, filter *models.RedditFilter) bool {
	// Check upvotes
	if post.Score < filter.MinUpvotes {
		return false
	}

	// Check date range
	postTime := time.Unix(int64(post.CreatedUTC), 0)
	if !filter.StartDate.IsZero() && postTime.Before(filter.StartDate) {
		return false
	}
	if !filter.EndDate.IsZero() && postTime.After(filter.EndDate) {
		return false
	}

	// Check if post has media
	if !hasMedia(post) {
		return false
	}

	return true
}

// hasMedia checks if a post contains downloadable media
func hasMedia(post *RedditPost) bool {
	// Check for direct image/video links
	if post.URL != "" {
		lower := strings.ToLower(post.URL)
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".gif") ||
			strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") {
			return true
		}
	}

	// Check for Reddit hosted media
	if post.IsVideo || (post.PostHint == "image") {
		return true
	}

	// Check for gallery
	if post.IsGallery {
		return true
	}

	return false
}

// processPost downloads media from a Reddit post
func (s *Scraper) processPost(ctx context.Context, post *RedditPost) error {
	mediaURLs := extractMediaURLs(post)

	for _, mediaURL := range mediaURLs {
		media := &models.Media{
			Platform:     models.PlatformReddit,
			URL:          mediaURL,
			Title:        post.Title,
			Description:  post.Selftext,
			Author:       post.Author,
			SourceURL:    fmt.Sprintf("https://reddit.com%s", post.Permalink),
			Upvotes:      post.Score,
			PostedAt:     time.Unix(int64(post.CreatedUTC), 0),
			DownloadedAt: time.Now(),
			Metadata: map[string]interface{}{
				"subreddit":    post.Subreddit,
				"post_id":      post.ID,
				"num_comments": post.NumComments,
			},
		}

		// Determine media type
		media.MediaType = determineMediaType(mediaURL)

		// Generate hash from URL (will be recalculated from actual data during save)
		media.Hash = fmt.Sprintf("%x", []byte(mediaURL))

		if err := s.ProcessMedia(ctx, media); err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
	}

	return nil
}

// extractMediaURLs extracts all media URLs from a post
func extractMediaURLs(post *RedditPost) []string {
	var urls []string

	// Direct image/video link
	if post.URL != "" && hasMedia(post) {
		// Handle Reddit video
		if post.IsVideo && post.Media.RedditVideo != nil {
			urls = append(urls, post.Media.RedditVideo.FallbackURL)
		} else {
			urls = append(urls, post.URL)
		}
	}

	// Gallery
	if post.IsGallery && post.MediaMetadata != nil {
		for _, item := range post.MediaMetadata {
			if item.S.U != "" {
				// Decode HTML entities in URL
				decodedURL := strings.ReplaceAll(item.S.U, "&amp;", "&")
				urls = append(urls, decodedURL)
			}
		}
	}

	return urls
}

// determineMediaType determines media type from URL
func determineMediaType(mediaURL string) models.MediaType {
	lower := strings.ToLower(mediaURL)
	if strings.Contains(lower, ".gif") || strings.Contains(lower, "gifv") {
		return models.MediaTypeGIF
	}
	if strings.Contains(lower, ".mp4") || strings.Contains(lower, ".webm") || strings.Contains(lower, "v.redd.it") {
		return models.MediaTypeVideo
	}
	return models.MediaTypeImage
}

// parseRedditListing parses Reddit JSON listing
func parseRedditListing(data []byte) (*RedditListing, error) {
	var listing RedditListing
	if err := json.Unmarshal(data, &listing); err != nil {
		return nil, err
	}
	return &listing, nil
}

// RedditListing represents a Reddit API listing response
type RedditListing struct {
	Data struct {
		Children []struct {
			Data *RedditPost `json:"data"`
		} `json:"children"`
		After string `json:"after"`
	} `json:"data"`
}

// RedditPost represents a Reddit post
type RedditPost struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Selftext    string  `json:"selftext"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
	Permalink   string  `json:"permalink"`
	URL         string  `json:"url"`
	IsVideo     bool    `json:"is_video"`
	IsGallery   bool    `json:"is_gallery"`
	PostHint    string  `json:"post_hint"`
	Media       struct {
		RedditVideo *struct {
			FallbackURL string `json:"fallback_url"`
		} `json:"reddit_video"`
	} `json:"media"`
	MediaMetadata map[string]struct {
		S struct {
			U string `json:"u"`
		} `json:"s"`
	} `json:"media_metadata"`
}
