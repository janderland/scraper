#!/bin/bash

# Integration Test Runner Script
# This script helps run integration tests with proper configuration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if .env.test exists
if [ ! -f .env.test ]; then
    print_warning ".env.test file not found"
    print_info "Copy .env.test.example to .env.test and fill in your credentials"
    echo ""
    echo "  cp .env.test.example .env.test"
    echo "  nano .env.test  # Edit with your credentials"
    echo ""
    print_info "Alternatively, set environment variables:"
    echo "  export REDDIT_CLIENT_ID=..."
    echo "  export REDDIT_CLIENT_SECRET=..."
    echo "  # etc."
    echo ""
else
    print_success "Found .env.test file"
    # Load environment variables from .env.test
    export $(cat .env.test | grep -v '^#' | xargs)
fi

# Parse command line arguments
PLATFORM="${1:-all}"
VERBOSE="${2:-false}"

# Build test command
TEST_CMD="go test -tags=integration"

if [ "$VERBOSE" = "-v" ] || [ "$VERBOSE" = "verbose" ]; then
    TEST_CMD="$TEST_CMD -v"
fi

TEST_CMD="$TEST_CMD -timeout 20m"

# Show what will be tested
echo ""
print_info "Integration Test Runner"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check which credentials are available
REDDIT_AVAIL="❌"
INSTAGRAM_AVAIL="❌"
FACEBOOK_AVAIL="❌"

if [ ! -z "$REDDIT_CLIENT_ID" ] && [ ! -z "$REDDIT_CLIENT_SECRET" ]; then
    REDDIT_AVAIL="✅"
fi

if [ ! -z "$INSTAGRAM_SESSION_ID" ]; then
    INSTAGRAM_AVAIL="✅"
fi

if [ ! -z "$FACEBOOK_ACCESS_TOKEN" ]; then
    FACEBOOK_AVAIL="✅"
fi

echo ""
print_info "Available credentials:"
echo "  Reddit:    $REDDIT_AVAIL"
echo "  Instagram: $INSTAGRAM_AVAIL"
echo "  Facebook:  $FACEBOOK_AVAIL"
echo ""

# Determine which tests to run
case "$PLATFORM" in
    all)
        print_info "Running tests for: All platforms"
        TEST_CMD="$TEST_CMD ./internal/scraper/..."
        ;;
    reddit)
        print_info "Running tests for: Reddit only"
        if [ "$REDDIT_AVAIL" = "❌" ]; then
            print_error "Reddit credentials not configured!"
            exit 1
        fi
        TEST_CMD="$TEST_CMD ./internal/scraper/reddit"
        ;;
    instagram)
        print_info "Running tests for: Instagram only"
        if [ "$INSTAGRAM_AVAIL" = "❌" ]; then
            print_error "Instagram credentials not configured!"
            exit 1
        fi
        TEST_CMD="$TEST_CMD ./internal/scraper/instagram"
        ;;
    facebook)
        print_info "Running tests for: Facebook only"
        if [ "$FACEBOOK_AVAIL" = "❌" ]; then
            print_error "Facebook credentials not configured!"
            exit 1
        fi
        TEST_CMD="$TEST_CMD ./internal/scraper/facebook"
        ;;
    *)
        print_error "Invalid platform: $PLATFORM"
        echo ""
        echo "Usage: $0 [platform] [verbose]"
        echo ""
        echo "Platforms:"
        echo "  all        - Run all integration tests (default)"
        echo "  reddit     - Run Reddit tests only"
        echo "  instagram  - Run Instagram tests only"
        echo "  facebook   - Run Facebook tests only"
        echo ""
        echo "Verbose:"
        echo "  -v         - Enable verbose output"
        echo ""
        echo "Examples:"
        echo "  $0                    # Run all tests"
        echo "  $0 reddit             # Run Reddit tests only"
        echo "  $0 instagram -v       # Run Instagram tests with verbose output"
        exit 1
        ;;
esac

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Print the command that will be executed
print_info "Executing: $TEST_CMD"
echo ""

# Run the tests
if $TEST_CMD; then
    echo ""
    print_success "All integration tests passed!"
    exit 0
else
    EXIT_CODE=$?
    echo ""
    if [ $EXIT_CODE -eq 0 ]; then
        print_warning "Some tests were skipped (likely due to missing credentials)"
    else
        print_error "Integration tests failed with exit code $EXIT_CODE"
    fi
    exit $EXIT_CODE
fi
