# Web Scraper - Multi-Platform Media Downloader

[![CI](https://github.com/janderland/scraper/workflows/CI/badge.svg)](https://github.com/janderland/scraper/actions/workflows/ci.yml)
[![Quick Test](https://github.com/janderland/scraper/workflows/Quick%20Test/badge.svg)](https://github.com/janderland/scraper/actions/workflows/quick-test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/janderland/scraper)](https://goreportcard.com/report/github.com/janderland/scraper)
[![codecov](https://codecov.io/gh/janderland/scraper/branch/main/graph/badge.svg)](https://codecov.io/gh/janderland/scraper)
[![License](https://img.shields.io/badge/license-Educational-blue.svg)](LICENSE)

A comprehensive web scraper written in Go that supports downloading media from Reddit, Instagram, and Facebook with advanced filtering, VPN routing, and a GUI for browsing downloaded content.

## Features

### Supported Platforms

- **Reddit**: Download from subreddits and user saved posts
- **Instagram**: Download from user profiles
- **Facebook**: Download from user photo albums

### Core Capabilities

- **Advanced Filtering**: Filter by date range, upvotes, post count, and more
- **VPN Routing**: Automatic VPN selection and failover for optimal throughput
- **Deduplication**: Automatically detects and skips already-downloaded media
- **SQLite Database**: Stores metadata for all downloaded media
- **Tagging System**: Auto-tag media based on regex rules matched against metadata
- **GUI Browser**: Browse, search, and view downloaded media with a user-friendly interface
- **Fuzzy Search**: Search media using fuzzy text matching

## Installation

### Prerequisites

- Go 1.21 or later
- SQLite3
- GCC (for CGo compilation of SQLite driver)
- **Optional**: OpenVPN, WireGuard, or SOCKS5 proxy (for VPN support)

### Build from Source

```bash
git clone https://github.com/janderland/scraper.git
cd scraper
go mod download
go build -o scraper ./cmd/scraper-v2
go build -o scraper-gui ./cmd/gui
```

### Quick Start

1. Copy example configuration:
   ```bash
   cp config/vpn.example.yaml config/vpn.yaml
   ```

2. Edit `config/vpn.yaml` with your settings

3. Run the scraper:
   ```bash
   ./scraper --config config/vpn.yaml -platform reddit -source pics -max 50
   ```

## Usage

### Command Line Interface

The CLI tool supports scraping from all platforms with various filtering options.

#### Reddit

Scrape a subreddit:
```bash
./scraper -platform reddit -source funny -max 100 -upvotes 100
```

Scrape with date range:
```bash
./scraper -platform reddit -source pics -max 50 -start-date 2024-01-01 -end-date 2024-12-31
```

Scrape user's saved posts:
```bash
./scraper -platform reddit -source saved -max 200
```

#### Instagram

Scrape a user's profile (newest first):
```bash
./scraper -platform instagram -source username -max 100
```

Scrape oldest posts first:
```bash
./scraper -platform instagram -source username -max 100 -oldest-first
```

#### Facebook

Scrape a user's photos:
```bash
./scraper -platform facebook -source username -max 100
```

Scrape with filter suggestions:
```bash
SHOW_SUGGESTIONS=true ./scraper -platform facebook -source username -max 10
```

Scrape with date range and ordering:
```bash
./scraper -platform facebook -source username -max 50 -start-date 2024-01-01 -oldest-first
```

### GUI Browser

Launch the GUI to browse downloaded media:

```bash
./scraper-gui -db ./scraper.db
```

#### GUI Features

- **Grid View**: Browse media in a thumbnail grid
- **Fullscreen View**: View images in fullscreen mode
- **Metadata Display**: See all metadata including title, author, upvotes, etc.
- **Search**: Fuzzy text search across all metadata fields
- **Filtering**: Filter by platform, tags, and search terms
- **Tag Management**: Create tags with regex rules for automatic tagging
- **Pagination**: Navigate through large collections

## Configuration

### Directory Structure

```
scraper/
├── scraper.db          # SQLite database
├── downloads/          # Downloaded media organized by platform/year/month
│   ├── reddit/
│   │   └── 2024/
│   │       └── 01/
│   ├── instagram/
│   └── facebook/
└── config/             # Configuration files
    ├── instagram_session.txt
    └── facebook_token.txt
```

### Authentication

#### Instagram

Instagram requires a session ID for scraping. To obtain one:

1. Log into Instagram in your browser
2. Open Developer Tools (F12)
3. Go to Application/Storage → Cookies
4. Copy the value of the `sessionid` cookie
5. Save it to `config/instagram_session.txt`

#### Facebook

Facebook requires an access token:

1. Go to [Facebook Graph API Explorer](https://developers.facebook.com/tools/explorer/)
2. Get a User Access Token with `user_photos` permission
3. Save it to `config/facebook_token.txt`

### VPN Configuration

VPN support allows routing requests through multiple VPNs with automatic failover.

To add VPN configurations, you'll need to add entries to the database:

```sql
INSERT INTO vpn_configs (name, host, port, protocol, username, password, active)
VALUES ('VPN1', 'vpn1.example.com', 1194, 'openvpn', 'user', 'pass', 1);
```

The scraper will automatically:
- Select the best VPN based on failure rate and last usage
- Fail over to other VPNs if one gets rate-limited
- Use multiple VPNs concurrently for optimal throughput

## Filtering Options

### Reddit Filters

- `source`: Subreddit name or "saved"
- `max`: Maximum number of posts to scrape
- `upvotes`: Minimum upvote threshold
- `start-date`: Start date (YYYY-MM-DD)
- `end-date`: End date (YYYY-MM-DD)

### Instagram Filters

- `source`: Username to scrape
- `max`: Maximum number of posts to scrape
- `oldest-first`: Scrape from oldest to newest

### Facebook Filters

- `source`: Username to scrape
- `max`: Maximum number of posts to scrape
- `start-date`: Start date (YYYY-MM-DD)
- `end-date`: End date (YYYY-MM-DD)
- `oldest-first`: Scrape from oldest to newest

## Tagging System

The scraper includes an automatic tagging system that can apply tags to media based on regex rules.

### Creating Tags

Tags can be created through the GUI or directly in the database:

```sql
INSERT INTO tags (name, regex_rule, created_at)
VALUES ('nsfw', '(?i)(nsfw|nude)', datetime('now'));
```

### Auto-Tagging

When media is downloaded, the scraper automatically:
1. Builds searchable text from title, description, author, and metadata
2. Tests each tag's regex rule against the text
3. Applies matching tags to the media

### Manual Tagging

You can also manually add tags to media through the GUI.

## Database Schema

The scraper uses SQLite with the following main tables:

- `media`: Stores all downloaded media with metadata
- `tags`: Stores tag definitions with optional regex rules
- `media_tags`: Many-to-many relationship between media and tags
- `vpn_configs`: VPN configuration entries
- `download_stats`: Statistics for VPN performance tracking

## Testing

The project includes comprehensive tests for all scrapers:

```bash
# Run all tests
go test ./...

# Run specific platform tests
go test ./internal/scraper/reddit/
go test ./internal/scraper/instagram/
go test ./internal/scraper/facebook/

# Run with verbose output
go test -v ./internal/scraper/reddit/
```

### Test Coverage

Tests include:
- Basic scraping functionality
- All filtering options (date range, max posts, upvotes, etc.)
- Ordering options (oldest first, newest first)
- Deduplication
- Error handling (no media posts, network failures)
- Filter suggestions (Facebook)

## Architecture

### Project Structure

```
scraper/
├── cmd/
│   ├── scraper/        # CLI application
│   └── gui/            # GUI application
├── internal/
│   ├── models/         # Data models
│   ├── storage/        # Database and file storage
│   ├── scraper/        # Scraper implementations
│   │   ├── reddit/
│   │   ├── instagram/
│   │   └── facebook/
│   ├── vpn/            # VPN management
│   ├── tags/           # Tagging system
│   └── gui/            # GUI implementation
└── pkg/
    └── http/           # HTTP utilities
```

### Key Components

1. **Base Scraper**: Common functionality for all platform scrapers
2. **VPN Manager**: Handles VPN selection, failover, and load balancing
3. **File Storage**: Manages file system operations with deduplication
4. **Database**: SQLite storage for metadata and relationships
5. **Tagger**: Automatic and manual tagging system
6. **GUI**: Fyne-based graphical interface

## Performance Considerations

- **Rate Limiting**: Built-in delays between requests to respect API limits
- **Concurrent VPNs**: Multiple VPNs can be used simultaneously
- **Deduplication**: Hash-based deduplication prevents re-downloading
- **Batch Processing**: Efficient database operations with batching
- **Pagination**: Memory-efficient processing of large result sets

## Legal and Ethical Considerations

**Important**: This tool is for educational purposes. When using it:

- Respect each platform's Terms of Service
- Respect rate limits and don't overload servers
- Only download content you have permission to access
- Don't use for commercial purposes without proper authorization
- Be aware of copyright and privacy laws in your jurisdiction

## Troubleshooting

### Common Issues

1. **Database locked**: Close other applications using the database
2. **VPN connection failed**: Check VPN credentials and connectivity
3. **Rate limited**: Reduce request rate or add more VPNs
4. **Authentication failed**: Verify session IDs and access tokens are valid

### Debug Mode

Enable verbose logging:
```bash
export DEBUG=1
./scraper -platform reddit -source pics -max 10
```

## CI/CD Pipeline

The project uses GitHub Actions for continuous integration and deployment:

### CI Workflows

1. **Main CI** (`.github/workflows/ci.yml`)
   - Runs on every push and pull request
   - Tests with Go 1.21 and 1.22
   - Builds CLI and GUI applications
   - Runs full test suite with race detection
   - Generates code coverage reports
   - Performs linting with golangci-lint
   - Builds for multiple platforms (Linux, macOS, Windows)
   - Runs security scans with gosec

2. **Quick Test** (`.github/workflows/quick-test.yml`)
   - Fast feedback on every commit
   - Runs short tests only
   - Checks code formatting
   - Runs go vet

3. **Release** (`.github/workflows/release.yml`)
   - Triggers on version tags (v*)
   - Builds binaries for all platforms
   - Creates GitHub releases with artifacts
   - Generates changelog automatically

### Running CI Locally

To run the same checks locally before pushing:

```bash
# Run tests
go test -v -race ./...

# Run linter
golangci-lint run

# Check formatting
gofmt -s -l .

# Run vet
go vet ./...

# Build all targets
go build ./...
```

### Test Coverage

Test coverage reports are automatically generated and uploaded to Codecov. View coverage at:
https://codecov.io/gh/janderland/scraper

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass (`go test ./...`)
5. Ensure linting passes (`golangci-lint run`)
6. Submit a pull request

All pull requests must pass CI checks before merging.

## License

This project is provided as-is for educational purposes.

## Changelog

### Version 1.0.0

- Initial release
- Support for Reddit, Instagram, and Facebook
- VPN routing with automatic failover
- SQLite database with comprehensive metadata
- GUI for browsing and searching
- Automatic tagging system
- Comprehensive test suite

## Future Enhancements

- [ ] Support for additional platforms (Twitter, TikTok, etc.)
- [ ] Video transcoding and thumbnail generation
- [ ] Cloud storage integration (S3, Google Drive)
- [ ] Web interface for remote access
- [ ] Machine learning-based content classification
- [ ] Scheduled scraping jobs
- [ ] Export functionality (ZIP archives, etc.)
- [ ] Advanced search with boolean operators
- [ ] Duplicate detection across platforms

## Support

For issues, questions, or contributions, please open an issue on GitHub.
