# Integration Testing Guide

This guide explains how to set up and run integration tests that validate the scraper against actual Reddit, Instagram, and Facebook platforms.

## Table of Contents

- [Overview](#overview)
- [Setting Up Credentials](#setting-up-credentials)
  - [Reddit](#reddit-credentials)
  - [Instagram](#instagram-credentials)
  - [Facebook](#facebook-credentials)
- [Running Tests](#running-tests)
- [CI/CD Integration](#cicd-integration)
- [Troubleshooting](#troubleshooting)

## Overview

The scraper includes two types of tests:

1. **Unit Tests** (default) - Use mock HTTP clients, run fast, don't require credentials
2. **Integration Tests** (opt-in) - Hit real APIs, require credentials, validate actual functionality

Integration tests are marked with the `integration` build tag and will only run when explicitly requested.

### Why Integration Tests?

- Validate scrapers work against current API versions
- Catch API changes (endpoint URLs, response formats, authentication)
- Test rate limiting and error handling with real services
- Verify the full end-to-end workflow

### Important Notes

⚠️ **Rate Limiting**: Integration tests make real API calls and are subject to rate limits. Run sparingly.

⚠️ **API Changes**: Platforms frequently change their APIs and anti-scraping measures. Integration tests help detect this.

⚠️ **Credentials Security**: Never commit credentials to git. Use environment variables or `.env.test` (gitignored).

## Setting Up Credentials

### Reddit Credentials

Reddit uses OAuth2 for authentication. You'll need to create a Reddit app to get credentials.

#### Steps:

1. **Create a Reddit App**:
   - Go to https://www.reddit.com/prefs/apps
   - Scroll to "Developed Applications"
   - Click "Create App" or "Create Another App"

2. **Fill in the form**:
   - **Name**: Choose any name (e.g., "Personal Scraper")
   - **App type**: Select "script"
   - **Description**: Optional
   - **About URL**: Optional
   - **Redirect URI**: Enter `http://localhost:8080` (required but not used for script apps)

3. **Get your credentials**:
   - **Client ID**: The string under your app name (looks like `abcd1234efgh5678`)
   - **Client Secret**: The "secret" field (looks like `aBcD1234-eFgH5678_iJkL9012`)

4. **Set environment variables**:
   ```bash
   export REDDIT_CLIENT_ID="your_client_id"
   export REDDIT_CLIENT_SECRET="your_client_secret"
   export REDDIT_USERNAME="your_reddit_username"
   export REDDIT_PASSWORD="your_reddit_password"
   ```

#### Using .env.test file:

```bash
# Copy the example file
cp .env.test.example .env.test

# Edit .env.test with your credentials
nano .env.test
```

Add:
```env
REDDIT_CLIENT_ID=abcd1234efgh5678
REDDIT_CLIENT_SECRET=aBcD1234-eFgH5678_iJkL9012
REDDIT_USERNAME=your_username
REDDIT_PASSWORD=your_password
```

### Instagram Credentials

Instagram doesn't have an official public API for this purpose. The scraper uses session cookies from an authenticated browser session.

#### Steps:

1. **Log into Instagram** in your browser (Chrome, Firefox, etc.)

2. **Open Developer Tools**:
   - Chrome: `F12` or `Ctrl+Shift+I` (Windows/Linux) / `Cmd+Option+I` (Mac)
   - Firefox: `F12` or `Ctrl+Shift+I` (Windows/Linux) / `Cmd+Option+I` (Mac)

3. **Find the sessionid cookie**:
   - Go to the "Application" tab (Chrome) or "Storage" tab (Firefox)
   - Navigate to Cookies → https://www.instagram.com
   - Find the cookie named `sessionid`
   - Copy its value (long string of characters)

4. **Set environment variable**:
   ```bash
   export INSTAGRAM_SESSION_ID="your_session_id_here"
   ```

Or in `.env.test`:
```env
INSTAGRAM_SESSION_ID=1234567890abcdef%3A1234567890%3A1234567890
```

#### Important Instagram Notes:

- Session IDs expire after a period of inactivity
- Instagram has aggressive rate limiting and bot detection
- Running too many tests may trigger security checks
- Consider using a test account, not your main account
- Instagram may require 2FA, which can complicate automation

### Facebook Credentials

Facebook uses the Graph API with access tokens.

#### Steps:

1. **Create a Facebook App** (if you don't have one):
   - Go to https://developers.facebook.com/apps
   - Click "Create App"
   - Choose "Consumer" or "Business" type
   - Fill in app details

2. **Get an Access Token**:

   **Option A: Graph API Explorer (Quick, Short-lived)**
   - Go to https://developers.facebook.com/tools/explorer/
   - Select your app from the dropdown
   - Click "Generate Access Token"
   - Grant permissions: `user_photos`, `user_posts`
   - Copy the access token

   **Option B: App Dashboard (Long-lived)**
   - Go to your app dashboard
   - Navigate to Settings → Basic
   - Copy your App ID and App Secret
   - Use the token exchange endpoint to get a long-lived token

3. **Set environment variable**:
   ```bash
   export FACEBOOK_ACCESS_TOKEN="your_access_token"
   ```

Or in `.env.test`:
```env
FACEBOOK_ACCESS_TOKEN=EAABwzLixnjYBO1234567890abcdef...
```

#### Important Facebook Notes:

- Access tokens expire (short-lived: hours, long-lived: 60 days)
- Tokens are tied to specific permissions
- Facebook has strict rate limiting
- Some features require app review

## Running Tests

### Run Only Unit Tests (Default)

```bash
# All unit tests
go test ./...

# Specific package
go test ./internal/scraper/reddit
```

### Run Only Integration Tests

```bash
# All integration tests
go test -tags=integration ./...

# Specific platform
go test -tags=integration ./internal/scraper/reddit
go test -tags=integration ./internal/scraper/instagram
go test -tags=integration ./internal/scraper/facebook

# Specific test
go test -tags=integration -run TestRedditIntegration_ScrapeSaved ./internal/scraper/reddit
```

### Run All Tests (Unit + Integration)

```bash
go test -tags=integration -v ./...
```

### Verbose Output

```bash
# See detailed test output
go test -tags=integration -v ./internal/scraper/reddit

# See HTTP requests (if implemented)
go test -tags=integration -v -trace ./internal/scraper/reddit
```

### With Coverage

```bash
go test -tags=integration -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## CI/CD Integration

### GitHub Actions

Integration tests can run in CI using GitHub Secrets for credentials.

#### Setting Up Secrets:

1. Go to your repository on GitHub
2. Navigate to Settings → Secrets and variables → Actions
3. Add the following secrets:
   - `REDDIT_CLIENT_ID`
   - `REDDIT_CLIENT_SECRET`
   - `REDDIT_USERNAME`
   - `REDDIT_PASSWORD`
   - `INSTAGRAM_SESSION_ID`
   - `FACEBOOK_ACCESS_TOKEN`

#### Workflow Example:

```yaml
name: Integration Tests

on:
  # Run on manual trigger only (to avoid excessive API calls)
  workflow_dispatch:
  # Or on schedule (e.g., weekly)
  schedule:
    - cron: '0 0 * * 0'  # Every Sunday at midnight

jobs:
  integration-test:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Run Integration Tests
      env:
        REDDIT_CLIENT_ID: ${{ secrets.REDDIT_CLIENT_ID }}
        REDDIT_CLIENT_SECRET: ${{ secrets.REDDIT_CLIENT_SECRET }}
        REDDIT_USERNAME: ${{ secrets.REDDIT_USERNAME }}
        REDDIT_PASSWORD: ${{ secrets.REDDIT_PASSWORD }}
        INSTAGRAM_SESSION_ID: ${{ secrets.INSTAGRAM_SESSION_ID }}
        FACEBOOK_ACCESS_TOKEN: ${{ secrets.FACEBOOK_ACCESS_TOKEN }}
      run: go test -tags=integration -v ./...
```

### Best Practices for CI:

- **Don't run on every commit** - Use manual triggers or schedules
- **Set timeouts** - API calls can hang
- **Handle failures gracefully** - APIs may be down temporarily
- **Rotate credentials** - Use dedicated test accounts
- **Monitor rate limits** - Track API usage

## Troubleshooting

### Tests Skip with "credentials not configured"

**Problem**: All integration tests are skipped.

**Solution**:
- Verify environment variables are set: `env | grep REDDIT`
- Check `.env.test` exists and has correct values
- Make sure you're running with `-tags=integration`

### Reddit OAuth Errors

**Problem**: `401 Unauthorized` or `invalid_grant` errors.

**Solutions**:
- Verify client ID and secret are correct
- Check username and password are correct
- Ensure app type is "script" (not "web app")
- Try creating a new Reddit app

### Instagram "Login Required" Errors

**Problem**: `401` or `403` errors, or "login required" responses.

**Solutions**:
- Get a fresh session ID from your browser
- Verify you're logged into Instagram in the browser
- Check if Instagram flagged your account (try logging in normally)
- Use a different browser or private/incognito mode
- Instagram may have detected automated activity - wait 24 hours

### Facebook Token Expired

**Problem**: `Invalid OAuth access token` error.

**Solutions**:
- Generate a new access token from Graph API Explorer
- Use long-lived tokens (60-day expiration)
- Implement token refresh logic (advanced)
- Check token permissions include `user_photos`, `user_posts`

### Rate Limiting

**Problem**: `429 Too Many Requests` or similar errors.

**Solutions**:
- Wait before retrying (exponential backoff)
- Reduce `MaxPosts` in test filters
- Run fewer tests
- Spread out test runs over time
- Use VPN rotation (if configured)

### Integration Tests Take Too Long

**Problem**: Tests timeout or take minutes to run.

**Solutions**:
- Reduce `MaxPosts` in filters (use 2-5 for testing)
- Run specific tests instead of all: `go test -tags=integration -run TestName`
- Increase timeout: `go test -tags=integration -timeout 10m`
- Use parallel tests (if safe): `go test -tags=integration -parallel 4`

### Mock vs Real Client Confusion

**Problem**: Tests still use mocks even with `-tags=integration`.

**Solution**:
- Check build tags at top of file: `//go:build integration`
- Ensure `NewRealHTTPClient` is implemented (currently panics - needs implementation)
- Verify test file name ends with `_integration_test.go`

## Current Limitations

⚠️ **Real HTTP Clients Not Fully Implemented**

The integration test files are currently stubs. The `RealHTTPClient` implementations will panic with:
- Reddit: `"RealHTTPClient not yet implemented - OAuth2 flow needed"`
- Instagram: `"RealHTTPClient not yet implemented - Instagram session handling needed"`
- Facebook: `"RealHTTPClient not yet implemented - Facebook Graph API handling needed"`

To make these tests functional, you need to implement:

1. **Reddit OAuth2 Client** (`reddit/reddit_integration_test.go`):
   - Token acquisition using client credentials + password grant
   - Token refresh logic
   - Authenticated requests with Bearer token

2. **Instagram HTTP Client** (`instagram/instagram_integration_test.go`):
   - Session cookie handling
   - Proper User-Agent headers
   - GraphQL query formatting
   - CSRF token handling

3. **Facebook Graph API Client** (`facebook/facebook_integration_test.go`):
   - Access token injection in URLs
   - Pagination handling
   - Error response parsing

See the existing scraper implementations in `internal/scraper/*/` for reference on how to structure these clients.

## Next Steps

1. Copy `.env.test.example` to `.env.test`
2. Fill in your credentials in `.env.test`
3. Implement the `RealHTTPClient` for each platform (or use existing scraper HTTP clients)
4. Run a single test to verify: `go test -tags=integration -v -run TestRedditIntegration_ScrapeSaved ./internal/scraper/reddit`
5. Gradually enable more tests as you verify they work
6. Set up CI with GitHub Secrets for automated testing

## Security Reminders

✅ **DO:**
- Use environment variables or `.env.test` for credentials
- Use dedicated test accounts (not your personal accounts)
- Rotate credentials regularly
- Store secrets in GitHub Secrets for CI
- Review and audit credential access logs

❌ **DON'T:**
- Commit `.env.test` to git (it's in `.gitignore`)
- Share credentials in public channels
- Use production credentials for testing
- Hard-code credentials in test files
- Store credentials in unencrypted files

---

For more information:
- [Reddit API Documentation](https://www.reddit.com/dev/api)
- [Instagram Basic Display API](https://developers.facebook.com/docs/instagram-basic-display-api) (official, limited)
- [Facebook Graph API](https://developers.facebook.com/docs/graph-api)
