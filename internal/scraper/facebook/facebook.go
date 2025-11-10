package facebook

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/scraper"
	"github.com/janderland/scraper/internal/storage"
)

// Scraper implements Facebook scraping functionality
type Scraper struct {
	*scraper.BaseScraper
	accessToken string
}

// NewScraper creates a new Facebook scraper
func NewScraper(db *storage.Database, fs *storage.FileStorage, vpn scraper.VPNManager, accessToken string) *Scraper {
	return &Scraper{
		BaseScraper: scraper.NewBaseScraper(db, fs, vpn),
		accessToken: accessToken,
	}
}

// GetPlatform returns the platform
func (s *Scraper) GetPlatform() models.Platform {
	return models.PlatformFacebook
}

// Scrape performs Facebook scraping
func (s *Scraper) Scrape(ctx context.Context, filterInterface interface{}) error {
	filter, ok := filterInterface.(*models.FacebookFilter)
	if !ok {
		return fmt.Errorf("invalid filter type for Facebook scraper")
	}

	return s.scrapeUserPhotos(ctx, filter)
}

// scrapeUserPhotos scrapes photos/videos from a user's profile
func (s *Scraper) scrapeUserPhotos(ctx context.Context, filter *models.FacebookFilter) error {
	client, err := s.BaseScraper.GetHTTPClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get HTTP client: %w", err)
	}

	// Get user ID if username is provided
	userID := filter.Username

	// Query Facebook Graph API for user's photos
	posts, err := s.fetchUserPosts(client, userID, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch user posts: %w", err)
	}

	// Sort posts based on filter
	if filter.OldestFirst {
		// Reverse the order
		for i, j := 0, len(posts)-1; i < j; i, j = i+1, j-1 {
			posts[i], posts[j] = posts[j], posts[i]
		}
	}

	// Process posts
	maxPosts := filter.MaxPosts
	if maxPosts == 0 || maxPosts > len(posts) {
		maxPosts = len(posts)
	}

	for i := 0; i < maxPosts && !s.IsStopped(); i++ {
		post := posts[i]

		// Apply date filters
		if !s.shouldProcessPost(post, filter) {
			continue
		}

		if err := s.processPost(ctx, post); err != nil {
			fmt.Printf("Error processing post %s: %v\n", post.ID, err)
			continue
		}

		s.SetProgress(int(float64(i+1) / float64(maxPosts) * 100))
		time.Sleep(1 * time.Second) // Rate limiting
	}

	s.SetProgress(100)
	return nil
}

// GetFilterSuggestions queries Facebook to provide filter suggestions
func (s *Scraper) GetFilterSuggestions(ctx context.Context, username string) (*FilterSuggestions, error) {
	client, err := s.BaseScraper.GetHTTPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP client: %w", err)
	}

	// Fetch a sample of posts to generate suggestions
	sampleFilter := &models.FacebookFilter{
		Username: username,
		MaxPosts: 100,
	}

	posts, err := s.fetchUserPosts(client, username, sampleFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts for suggestions: %w", err)
	}

	suggestions := &FilterSuggestions{
		TotalPosts: len(posts),
	}

	if len(posts) > 0 {
		// Find date range
		oldestDate := posts[0].CreatedTime
		newestDate := posts[0].CreatedTime

		for _, post := range posts {
			if post.CreatedTime.Before(oldestDate) {
				oldestDate = post.CreatedTime
			}
			if post.CreatedTime.After(newestDate) {
				newestDate = post.CreatedTime
			}
		}

		suggestions.DateRange = DateRange{
			Oldest: oldestDate,
			Newest: newestDate,
		}

		// Suggest common ranges
		now := time.Now()
		suggestions.SuggestedRanges = []string{
			"Last 7 days",
			"Last 30 days",
			"Last 3 months",
			"Last year",
			"All time",
		}

		// Count posts by year
		yearCounts := make(map[int]int)
		for _, post := range posts {
			year := post.CreatedTime.Year()
			yearCounts[year]++
		}
		suggestions.PostsByYear = yearCounts
	}

	return suggestions, nil
}

// fetchUserPosts fetches posts from Facebook Graph API
func (s *Scraper) fetchUserPosts(client scraper.HTTPClient, userID string, filter *models.FacebookFilter) ([]*FacebookPost, error) {
	var posts []*FacebookPost

	// Facebook Graph API endpoint
	// Fields: id,created_time,message,full_picture,attachments,likes.summary(true)
	baseURL := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/photos/uploaded", userID)
	fields := "id,created_time,name,images,likes.summary(true),comments.summary(true)"

	maxPosts := filter.MaxPosts
	if maxPosts == 0 {
		maxPosts = 1000
	}

	nextURL := fmt.Sprintf("%s?fields=%s&access_token=%s&limit=100", baseURL, fields, s.accessToken)

	for nextURL != "" && len(posts) < maxPosts {
		data, err := client.Get(nextURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch posts: %w", err)
		}

		var response FacebookPhotosResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		posts = append(posts, response.Data...)

		// Check for next page
		if response.Paging.Next != "" {
			nextURL = response.Paging.Next
		} else {
			nextURL = ""
		}

		time.Sleep(1 * time.Second) // Rate limiting
	}

	return posts, nil
}

// shouldProcessPost checks if a post should be processed based on filters
func (s *Scraper) shouldProcessPost(post *FacebookPost, filter *models.FacebookFilter) bool {
	// Check date range
	if !filter.StartDate.IsZero() && post.CreatedTime.Before(filter.StartDate) {
		return false
	}
	if !filter.EndDate.IsZero() && post.CreatedTime.After(filter.EndDate) {
		return false
	}

	return true
}

// processPost downloads media from a Facebook post
func (s *Scraper) processPost(ctx context.Context, post *FacebookPost) error {
	// Get highest resolution image
	var imageURL string
	maxWidth := 0

	for _, img := range post.Images {
		if img.Width > maxWidth {
			maxWidth = img.Width
			imageURL = img.Source
		}
	}

	if imageURL == "" {
		return fmt.Errorf("no image URL found for post %s", post.ID)
	}

	media := &models.Media{
		Platform:     models.PlatformFacebook,
		MediaType:    models.MediaTypeImage,
		URL:          imageURL,
		Title:        post.ID,
		Description:  post.Name,
		Author:       "user", // Would need to fetch from user object
		SourceURL:    fmt.Sprintf("https://www.facebook.com/photo.php?fbid=%s", post.ID),
		Upvotes:      post.Likes.Summary.TotalCount,
		PostedAt:     post.CreatedTime,
		DownloadedAt: time.Now(),
		Metadata: map[string]interface{}{
			"post_id":       post.ID,
			"comment_count": post.Comments.Summary.TotalCount,
		},
	}

	// Generate hash from URL
	media.Hash = fmt.Sprintf("%x", []byte(imageURL))

	if err := s.ProcessMedia(ctx, media); err != nil {
		return fmt.Errorf("failed to process media: %w", err)
	}

	return nil
}

// FacebookPhotosResponse represents Facebook Graph API photos response
type FacebookPhotosResponse struct {
	Data []FacebookPost `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}

// FacebookPost represents a Facebook photo post
type FacebookPost struct {
	ID          string    `json:"id"`
	CreatedTime time.Time `json:"created_time"`
	Name        string    `json:"name"`
	Images      []struct {
		Height int    `json:"height"`
		Width  int    `json:"width"`
		Source string `json:"source"`
	} `json:"images"`
	Likes struct {
		Summary struct {
			TotalCount int `json:"total_count"`
		} `json:"summary"`
	} `json:"likes"`
	Comments struct {
		Summary struct {
			TotalCount int `json:"total_count"`
		} `json:"summary"`
	} `json:"comments"`
}

// FilterSuggestions provides suggestions for filtering options
type FilterSuggestions struct {
	TotalPosts      int
	DateRange       DateRange
	SuggestedRanges []string
	PostsByYear     map[int]int
}

// DateRange represents a date range
type DateRange struct {
	Oldest time.Time
	Newest time.Time
}
