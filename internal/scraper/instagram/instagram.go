package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/scraper"
	"github.com/janderland/scraper/internal/storage"
)

// Scraper implements Instagram scraping functionality
type Scraper struct {
	*scraper.BaseScraper
	sessionID string
}

// NewScraper creates a new Instagram scraper
func NewScraper(db *storage.Database, fs *storage.FileStorage, vpn scraper.VPNManager, sessionID string) *Scraper {
	return &Scraper{
		BaseScraper: scraper.NewBaseScraper(db, fs, vpn),
		sessionID:   sessionID,
	}
}

// GetPlatform returns the platform
func (s *Scraper) GetPlatform() models.Platform {
	return models.PlatformInstagram
}

// Scrape performs Instagram scraping
func (s *Scraper) Scrape(ctx context.Context, filterInterface interface{}) error {
	filter, ok := filterInterface.(*models.InstagramFilter)
	if !ok {
		return fmt.Errorf("invalid filter type for Instagram scraper")
	}

	return s.scrapeUserProfile(ctx, filter)
}

// scrapeUserProfile scrapes media from a user's profile
func (s *Scraper) scrapeUserProfile(ctx context.Context, filter *models.InstagramFilter) error {
	client, err := s.BaseScraper.GetHTTPClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get HTTP client: %w", err)
	}

	// Get user ID first
	userID, err := s.getUserID(client, filter.Username)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	// Fetch posts
	posts, err := s.fetchUserPosts(client, userID, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch user posts: %w", err)
	}

	// Sort posts based on filter
	if filter.OldestFirst {
		// Posts are typically returned newest first, so reverse
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

// getUserID retrieves the user ID for a given username
func (s *Scraper) getUserID(client scraper.HTTPClient, username string) (string, error) {
	// Instagram web profile URL
	profileURL := fmt.Sprintf("https://www.instagram.com/%s/", username)

	data, err := client.Get(profileURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch profile page: %w", err)
	}

	// Parse HTML to extract shared data
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Look for script tag containing user data
	var userID string
	doc.Find("script[type='text/javascript']").EachWithBreak(func(i int, s *goquery.Selection) bool {
		content := s.Text()
		if strings.Contains(content, "window._sharedData") {
			// Extract JSON data
			re := regexp.MustCompile(`window._sharedData = ({.*?});`)
			matches := re.FindStringSubmatch(content)
			if len(matches) > 1 {
				var sharedData map[string]interface{}
				if err := json.Unmarshal([]byte(matches[1]), &sharedData); err == nil {
					// Navigate through the JSON structure to find user ID
					if entryData, ok := sharedData["entry_data"].(map[string]interface{}); ok {
						if profilePage, ok := entryData["ProfilePage"].([]interface{}); ok && len(profilePage) > 0 {
							if page, ok := profilePage[0].(map[string]interface{}); ok {
								if graphql, ok := page["graphql"].(map[string]interface{}); ok {
									if user, ok := graphql["user"].(map[string]interface{}); ok {
										if id, ok := user["id"].(string); ok {
											userID = id
											return false // Break
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	if userID == "" {
		return "", fmt.Errorf("could not extract user ID from profile page")
	}

	return userID, nil
}

// fetchUserPosts fetches posts from a user's profile
func (s *Scraper) fetchUserPosts(client scraper.HTTPClient, userID string, filter *models.InstagramFilter) ([]*InstagramPost, error) {
	var posts []*InstagramPost

	// This is a simplified implementation
	// In reality, Instagram requires proper authentication and uses GraphQL queries
	// This example shows the structure, but would need proper API implementation

	queryHash := "9dcf6e1a98bc7f6e92953d5a61027b98" // Example query hash (changes frequently)
	variables := map[string]interface{}{
		"id":    userID,
		"first": 50,
	}

	maxPosts := filter.MaxPosts
	if maxPosts == 0 {
		maxPosts = 1000
	}

	hasNextPage := true
	endCursor := ""

	for hasNextPage && len(posts) < maxPosts {
		if endCursor != "" {
			variables["after"] = endCursor
		}

		variablesJSON, _ := json.Marshal(variables)
		queryURL := fmt.Sprintf("https://www.instagram.com/graphql/query/?query_hash=%s&variables=%s",
			queryHash, string(variablesJSON))

		data, err := client.Get(queryURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch posts: %w", err)
		}

		var response InstagramGraphQLResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Extract posts from response
		if response.Data.User.EdgeOwnerToTimelineMedia.Edges != nil {
			for _, edge := range response.Data.User.EdgeOwnerToTimelineMedia.Edges {
				posts = append(posts, &edge.Node)
			}
		}

		// Check for next page
		pageInfo := response.Data.User.EdgeOwnerToTimelineMedia.PageInfo
		hasNextPage = pageInfo.HasNextPage
		endCursor = pageInfo.EndCursor

		time.Sleep(2 * time.Second) // Rate limiting
	}

	return posts, nil
}

// processPost downloads media from an Instagram post
func (s *Scraper) processPost(ctx context.Context, post *InstagramPost) error {
	// Extract media URLs
	mediaURLs := s.extractMediaURLs(post)

	for _, mediaURL := range mediaURLs {
		media := &models.Media{
			Platform:     models.PlatformInstagram,
			URL:          mediaURL,
			Title:        post.Shortcode,
			Description:  s.extractCaption(post),
			Author:       post.Owner.Username,
			SourceURL:    fmt.Sprintf("https://www.instagram.com/p/%s/", post.Shortcode),
			Upvotes:      post.EdgeMediaPreviewLike.Count,
			PostedAt:     time.Unix(post.TakenAtTimestamp, 0),
			DownloadedAt: time.Now(),
			Metadata: map[string]interface{}{
				"post_id":       post.ID,
				"shortcode":     post.Shortcode,
				"is_video":      post.IsVideo,
				"comment_count": post.EdgeMediaToComment.Count,
			},
		}

		// Determine media type
		if post.IsVideo {
			media.MediaType = models.MediaTypeVideo
		} else {
			media.MediaType = models.MediaTypeImage
		}

		// Generate hash from URL
		media.Hash = fmt.Sprintf("%x", []byte(mediaURL))

		if err := s.ProcessMedia(ctx, media); err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
	}

	return nil
}

// extractMediaURLs extracts media URLs from a post
func (s *Scraper) extractMediaURLs(post *InstagramPost) []string {
	var urls []string

	// Single media post
	if post.DisplayURL != "" {
		urls = append(urls, post.DisplayURL)
	}

	// Video
	if post.IsVideo && post.VideoURL != "" {
		urls = []string{post.VideoURL} // Replace image with video
	}

	// Carousel (multiple images/videos)
	if post.EdgeSidecarToChildren.Edges != nil {
		urls = []string{} // Clear single media
		for _, edge := range post.EdgeSidecarToChildren.Edges {
			if edge.Node.IsVideo && edge.Node.VideoURL != "" {
				urls = append(urls, edge.Node.VideoURL)
			} else if edge.Node.DisplayURL != "" {
				urls = append(urls, edge.Node.DisplayURL)
			}
		}
	}

	return urls
}

// extractCaption extracts caption text from a post
func (s *Scraper) extractCaption(post *InstagramPost) string {
	if post.EdgeMediaToCaption.Edges != nil && len(post.EdgeMediaToCaption.Edges) > 0 {
		return post.EdgeMediaToCaption.Edges[0].Node.Text
	}
	return ""
}

// InstagramGraphQLResponse represents Instagram's GraphQL API response
type InstagramGraphQLResponse struct {
	Data struct {
		User struct {
			EdgeOwnerToTimelineMedia struct {
				Edges []struct {
					Node InstagramPost `json:"node"`
				} `json:"edges"`
				PageInfo struct {
					HasNextPage bool   `json:"has_next_page"`
					EndCursor   string `json:"end_cursor"`
				} `json:"page_info"`
			} `json:"edge_owner_to_timeline_media"`
		} `json:"user"`
	} `json:"data"`
}

// InstagramPost represents an Instagram post
type InstagramPost struct {
	ID                 string `json:"id"`
	Shortcode          string `json:"shortcode"`
	DisplayURL         string `json:"display_url"`
	IsVideo            bool   `json:"is_video"`
	VideoURL           string `json:"video_url"`
	TakenAtTimestamp   int64  `json:"taken_at_timestamp"`
	EdgeMediaPreviewLike struct {
		Count int `json:"count"`
	} `json:"edge_media_preview_like"`
	EdgeMediaToComment struct {
		Count int `json:"count"`
	} `json:"edge_media_to_comment"`
	EdgeMediaToCaption struct {
		Edges []struct {
			Node struct {
				Text string `json:"text"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"edge_media_to_caption"`
	Owner struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"owner"`
	EdgeSidecarToChildren struct {
		Edges []struct {
			Node struct {
				DisplayURL string `json:"display_url"`
				IsVideo    bool   `json:"is_video"`
				VideoURL   string `json:"video_url"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"edge_sidecar_to_children"`
}
