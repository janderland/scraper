#!/bin/bash

# CI validation script - runs the same checks as CI locally

set -e

echo "======================================"
echo "Running CI validation checks..."
echo "======================================"
echo ""

# Check 1: Formatting
echo "✓ Checking code formatting..."
UNFORMATTED=$(gofmt -l . 2>&1 || true)
if [ -n "$UNFORMATTED" ]; then
    echo "❌ The following files are not formatted:"
    echo "$UNFORMATTED"
    echo ""
    echo "Run: gofmt -s -w ."
    exit 1
fi
echo "✓ All files are properly formatted"
echo ""

# Check 2: Go vet
echo "✓ Running go vet..."
if ! go vet ./...; then
    echo "❌ go vet found issues"
    exit 1
fi
echo "✓ go vet passed"
echo ""

# Check 3: Build
echo "✓ Building project..."
if ! go build ./...; then
    echo "❌ Build failed"
    exit 1
fi
echo "✓ Build successful"
echo ""

# Check 4: Tests
echo "✓ Running tests..."
if ! go test -short ./...; then
    echo "❌ Tests failed"
    exit 1
fi
echo "✓ Tests passed"
echo ""

# Check 5: Build CLI
echo "✓ Building CLI..."
if ! go build -o /tmp/scraper ./cmd/scraper; then
    echo "❌ CLI build failed"
    exit 1
fi
echo "✓ CLI built successfully"
echo ""

# Check 6: Build GUI (may fail in headless)
echo "✓ Building GUI..."
if go build -o /tmp/scraper-gui ./cmd/gui 2>/dev/null; then
    echo "✓ GUI built successfully"
else
    echo "⚠ GUI build failed (expected in headless environment)"
fi
echo ""

echo "======================================"
echo "✓ All CI checks passed!"
echo "======================================"
