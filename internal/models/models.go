package models

import (
	"time"
)

// Platform represents the source platform
type Platform string

const (
	PlatformReddit    Platform = "reddit"
	PlatformInstagram Platform = "instagram"
	PlatformFacebook  Platform = "facebook"
)

// MediaType represents the type of media
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypeGIF   MediaType = "gif"
)

// Media represents a downloaded media item
type Media struct {
	ID          int64
	Platform    Platform
	MediaType   MediaType
	URL         string
	LocalPath   string
	Hash        string // For deduplication
	Title       string
	Description string
	Author      string
	SourceURL   string // Original post URL
	Upvotes     int
	PostedAt    time.Time
	DownloadedAt time.Time
	Metadata    map[string]interface{} // Additional platform-specific metadata
}

// Tag represents a tag that can be applied to media
type Tag struct {
	ID        int64
	Name      string
	RegexRule string // Regex pattern to auto-apply tag
	CreatedAt time.Time
}

// MediaTag represents the many-to-many relationship between media and tags
type MediaTag struct {
	MediaID int64
	TagID   int64
	Auto    bool // Whether this was auto-applied via regex
}

// RedditFilter contains filtering options for Reddit scraping
type RedditFilter struct {
	Source      string    // "saved" or subreddit name
	Username    string    // For saved posts
	MaxPosts    int       // 0 = unlimited
	MinUpvotes  int
	StartDate   time.Time
	EndDate     time.Time
}

// InstagramFilter contains filtering options for Instagram scraping
type InstagramFilter struct {
	Username  string
	MaxPosts  int  // 0 = unlimited
	OldestFirst bool // true = oldest to newest, false = newest to oldest
}

// FacebookFilter contains filtering options for Facebook scraping
type FacebookFilter struct {
	Username    string
	MaxPosts    int       // 0 = unlimited
	StartDate   time.Time
	EndDate     time.Time
	OldestFirst bool      // true = oldest to newest, false = newest to oldest
}

// VPNConfig represents a VPN configuration
type VPNConfig struct {
	ID           int64
	Name         string
	Host         string
	Port         int
	Protocol     string // "openvpn", "wireguard", etc.
	Username     string
	Password     string
	ConfigPath   string
	MaxBandwidth int64 // bytes per second, 0 = unlimited
	Active       bool
	FailureCount int
	LastUsed     time.Time
}

// DownloadStats tracks download statistics for VPN optimization
type DownloadStats struct {
	VPNID         int64
	BytesTransferred int64
	RequestCount  int
	FailureCount  int
	AvgLatency    time.Duration
	LastUpdated   time.Time
}

// ScraperJob represents a scraping task
type ScraperJob struct {
	ID        int64
	Platform  Platform
	Filter    interface{} // Platform-specific filter
	Status    string      // "pending", "running", "completed", "failed"
	Progress  int         // Percentage
	StartedAt time.Time
	CompletedAt time.Time
	Error     string
}
