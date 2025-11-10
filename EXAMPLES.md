# Usage Examples

This document provides practical examples of using the web scraper.

## Basic Usage Examples

### Reddit: Download Top Posts from a Subreddit

Download the top 50 images from r/EarthPorn with at least 1000 upvotes:

```bash
./scraper \
  -platform reddit \
  -source EarthPorn \
  -max 50 \
  -upvotes 1000
```

### Reddit: Download from Multiple Date Ranges

Download posts from January 2024:

```bash
./scraper \
  -platform reddit \
  -source wallpapers \
  -max 200 \
  -start-date 2024-01-01 \
  -end-date 2024-01-31
```

### Reddit: Scrape User's Saved Posts

```bash
./scraper \
  -platform reddit \
  -source saved \
  -max 500 \
  -upvotes 10
```

### Instagram: Download User Profile

Download the 100 most recent posts from a user:

```bash
./scraper \
  -platform instagram \
  -source username \
  -max 100
```

### Instagram: Download Oldest Posts First

Useful for archiving accounts:

```bash
./scraper \
  -platform instagram \
  -source username \
  -max 1000 \
  -oldest-first
```

### Facebook: Download Photos with Filter Suggestions

First, get suggestions:

```bash
SHOW_SUGGESTIONS=true ./scraper \
  -platform facebook \
  -source username \
  -max 10
```

Output:
```
Filter Suggestions:
Total posts available: 456
Date range: 2020-01-15 to 2024-11-10
Suggested ranges: [Last 7 days Last 30 days Last 3 months Last year All time]
Posts by year: map[2020:45 2021:89 2022:134 2023:102 2024:86]
```

Then download specific year:

```bash
./scraper \
  -platform facebook \
  -source username \
  -max 200 \
  -start-date 2024-01-01 \
  -end-date 2024-12-31
```

## Advanced Usage Examples

### Batch Processing Multiple Sources

Create a script to download from multiple subreddits:

```bash
#!/bin/bash

SUBREDDITS=("wallpapers" "EarthPorn" "itookapicture" "pics")

for sub in "${SUBREDDITS[@]}"; do
  echo "Downloading from r/$sub..."
  ./scraper -platform reddit -source "$sub" -max 100 -upvotes 500
  sleep 60  # Wait between subreddits
done
```

### Instagram: Archive Multiple Accounts

```bash
#!/bin/bash

ACCOUNTS=("account1" "account2" "account3")

for account in "${ACCOUNTS[@]}"; do
  echo "Archiving $account..."
  ./scraper -platform instagram -source "$account" -max 1000 -oldest-first
  sleep 300  # 5 minute delay between accounts
done
```

### Date Range Sweeping

Download content month by month:

```bash
#!/bin/bash

YEAR=2024
for MONTH in {01..12}; do
  START_DATE="$YEAR-$MONTH-01"
  END_DATE="$YEAR-$MONTH-31"

  echo "Downloading for $YEAR-$MONTH..."
  ./scraper \
    -platform reddit \
    -source pics \
    -max 500 \
    -start-date "$START_DATE" \
    -end-date "$END_DATE" \
    -upvotes 1000

  sleep 120
done
```

## GUI Usage Examples

### Browse All Downloaded Media

```bash
./scraper-gui
```

### Open Specific Database

```bash
./scraper-gui -db /path/to/archive.db
```

### Search Examples in GUI

Once the GUI is open:

1. **Search by keyword**: Type "sunset" in search box
2. **Filter by platform**: Select "Reddit" from platform dropdown
3. **Filter by tag**: Select a tag from tag dropdown
4. **Combine filters**: Use search + platform + tag together

### Creating Tags in GUI

1. Click "Manage Tags"
2. Click "Add Tag"
3. Enter tag name: `landscape`
4. Enter regex rule: `(?i)(landscape|scenery|vista)`
5. Click "Create"

Now all media with those words will be auto-tagged.

## Database Queries

### Direct SQLite Queries

Open the database:
```bash
sqlite3 scraper.db
```

Find most upvoted Reddit posts:
```sql
SELECT title, upvotes, source_url
FROM media
WHERE platform = 'reddit'
ORDER BY upvotes DESC
LIMIT 10;
```

Count media by platform:
```sql
SELECT platform, COUNT(*) as count
FROM media
GROUP BY platform;
```

Find media by date:
```sql
SELECT title, posted_at, platform
FROM media
WHERE posted_at >= '2024-01-01'
AND posted_at <= '2024-12-31'
ORDER BY posted_at DESC;
```

Find tagged media:
```sql
SELECT m.title, t.name as tag
FROM media m
JOIN media_tags mt ON m.id = mt.media_id
JOIN tags t ON mt.tag_id = t.id
WHERE t.name = 'landscape';
```

Get storage statistics:
```sql
SELECT
  platform,
  COUNT(*) as files,
  SUM(LENGTH(hash)) as approx_size
FROM media
GROUP BY platform;
```

## Automation Examples

### Cron Job for Daily Scraping

Add to crontab (`crontab -e`):

```bash
# Download top posts from r/wallpapers daily at 2 AM
0 2 * * * cd /path/to/scraper && ./scraper -platform reddit -source wallpapers -max 50 -upvotes 1000

# Archive Instagram account weekly on Sunday at 3 AM
0 3 * * 0 cd /path/to/scraper && ./scraper -platform instagram -source myaccount -max 100
```

### Systemd Service

Create `/etc/systemd/system/scraper-daily.service`:

```ini
[Unit]
Description=Daily Web Scraper
After=network.target

[Service]
Type=oneshot
User=youruser
WorkingDirectory=/path/to/scraper
ExecStart=/path/to/scraper/scraper -platform reddit -source wallpapers -max 100
```

Create `/etc/systemd/system/scraper-daily.timer`:

```ini
[Unit]
Description=Run scraper daily

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

Enable and start:
```bash
sudo systemctl enable scraper-daily.timer
sudo systemctl start scraper-daily.timer
```

### Docker Example

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git gcc musl-dev sqlite-dev

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o scraper ./cmd/scraper

FROM alpine:latest
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app
COPY --from=builder /app/scraper .
COPY config config/

VOLUME ["/app/downloads", "/app/data"]

ENTRYPOINT ["./scraper"]
```

Build and run:
```bash
docker build -t web-scraper .

docker run -v $(pwd)/downloads:/app/downloads \
           -v $(pwd)/data:/app/data \
           -v $(pwd)/config:/app/config \
           web-scraper -platform reddit -source pics -max 100
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  scraper:
    build: .
    volumes:
      - ./downloads:/app/downloads
      - ./data:/app/data
      - ./config:/app/config
    environment:
      - DB_PATH=/app/data/scraper.db
      - DOWNLOAD_DIR=/app/downloads
    command: -platform reddit -source wallpapers -max 100
```

Run:
```bash
docker-compose up
```

## Error Handling Examples

### Retry Failed Downloads

```bash
#!/bin/bash

MAX_RETRIES=3
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  echo "Attempt $((RETRY_COUNT + 1))..."

  if ./scraper -platform reddit -source pics -max 100; then
    echo "Success!"
    break
  else
    echo "Failed, retrying in 60 seconds..."
    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 60
  fi
done
```

### Log Output

```bash
# Log to file with timestamps
./scraper -platform reddit -source pics -max 100 2>&1 | \
  while read line; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') $line"
  done >> scraper.log
```

### Email Notifications

```bash
#!/bin/bash

./scraper -platform reddit -source wallpapers -max 100

if [ $? -eq 0 ]; then
  echo "Scraping completed successfully" | mail -s "Scraper Success" user@example.com
else
  echo "Scraping failed" | mail -s "Scraper Failed" user@example.com
fi
```

## Performance Optimization

### Parallel Scraping

```bash
# Scrape multiple sources in parallel
./scraper -platform reddit -source pics -max 100 &
./scraper -platform reddit -source wallpapers -max 100 &
./scraper -platform reddit -source itookapicture -max 100 &
wait
```

### Rate-Limited Scraping

```bash
# Download with controlled rate
for i in {1..10}; do
  ./scraper -platform reddit -source pics -max 10 -upvotes $((i * 100))
  sleep 30  # 30 second delay between batches
done
```

## Export Examples

### Export to CSV

```bash
sqlite3 -header -csv scraper.db \
  "SELECT title, author, upvotes, posted_at, source_url FROM media WHERE platform='reddit'" \
  > reddit_export.csv
```

### Export Tagged Media

```bash
sqlite3 -header -csv scraper.db \
  "SELECT m.title, m.local_path, t.name as tag
   FROM media m
   JOIN media_tags mt ON m.id = mt.media_id
   JOIN tags t ON mt.tag_id = t.id" \
  > tagged_media.csv
```

### Create Archive

```bash
# Create ZIP archive of downloaded media
zip -r archive.zip downloads/ scraper.db
```

## Maintenance Examples

### Cleanup Old Media

```bash
# Delete media older than 1 year
sqlite3 scraper.db "DELETE FROM media WHERE downloaded_at < date('now', '-1 year')"

# Remove orphaned files
find downloads/ -type f -mtime +365 -delete
```

### Database Optimization

```bash
# Vacuum database to reclaim space
sqlite3 scraper.db "VACUUM"

# Analyze for query optimization
sqlite3 scraper.db "ANALYZE"
```

### Backup Database

```bash
# Daily backup
cp scraper.db backups/scraper_$(date +%Y%m%d).db

# Incremental backup
sqlite3 scraper.db ".backup backups/scraper_backup.db"
```
