# Contributing to Web Scraper

Thank you for considering contributing to this project! This document outlines the process and guidelines.

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help maintain a welcoming environment

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in Issues
2. Create a new issue with:
   - Clear title and description
   - Steps to reproduce
   - Expected vs actual behavior
   - Go version and OS
   - Relevant logs or screenshots

### Suggesting Enhancements

1. Check if the enhancement has been suggested
2. Create a new issue describing:
   - The problem it solves
   - Your proposed solution
   - Alternative solutions considered
   - Any potential drawbacks

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. Make your changes:
   - Follow Go conventions and style
   - Add tests for new functionality
   - Update documentation as needed
   - Keep commits focused and atomic

4. Test your changes:
   ```bash
   # Run tests
   go test -v ./...

   # Run tests with race detection
   go test -v -race ./...

   # Check coverage
   go test -coverprofile=coverage.out ./...
   go tool cover -html=coverage.out

   # Run linter
   golangci-lint run

   # Format code
   gofmt -s -w .

   # Run vet
   go vet ./...
   ```

5. Commit your changes:
   ```bash
   git commit -m "feat: add new feature"
   ```

   Use conventional commit messages:
   - `feat:` - New feature
   - `fix:` - Bug fix
   - `docs:` - Documentation changes
   - `test:` - Test changes
   - `refactor:` - Code refactoring
   - `chore:` - Maintenance tasks

6. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

7. Create a Pull Request:
   - Provide a clear title and description
   - Reference any related issues
   - Ensure all CI checks pass
   - Respond to review feedback

## Development Setup

### Prerequisites

- Go 1.21 or later
- GCC (for CGO)
- SQLite3
- Git

### Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/scraper.git
cd scraper

# Install dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run tests to verify setup
go test ./...
```

## Code Style

### Go Style Guide

Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

Key points:
- Use `gofmt` for formatting
- Follow naming conventions (MixedCaps for exported names)
- Write clear comments for exported functions
- Keep functions focused and small
- Handle errors explicitly

### Project Structure

```
scraper/
├── cmd/              # Application entry points
├── internal/         # Private application code
│   ├── models/      # Data models
│   ├── storage/     # Database and file storage
│   ├── scraper/     # Platform scrapers
│   ├── vpn/         # VPN management
│   ├── tags/        # Tagging system
│   └── gui/         # GUI implementation
├── pkg/             # Public libraries
└── testdata/        # Test fixtures
```

### Testing Guidelines

1. **Unit Tests**
   - Test individual functions in isolation
   - Use table-driven tests where appropriate
   - Mock external dependencies

2. **Integration Tests**
   - Test component interactions
   - Use in-memory databases for testing
   - Clean up resources after tests

3. **Test Naming**
   ```go
   func TestFunctionName_Scenario(t *testing.T)
   func TestFunctionName_ErrorCase(t *testing.T)
   ```

4. **Coverage**
   - Aim for >80% coverage
   - Focus on critical paths
   - Don't sacrifice quality for coverage

### Example Test

```go
func TestRedditScraper_Filter_MinUpvotes(t *testing.T) {
    // Setup
    db, fs, cleanup := setupTestDB(t)
    defer cleanup()

    mockClient := &MockHTTPClient{
        responses: map[string][]byte{
            "url": []byte("response"),
        },
    }

    // Execute
    scraper := NewScraper(db, fs, mockVPN)
    err := scraper.Scrape(ctx, filter)

    // Verify
    if err != nil {
        t.Fatalf("Scraping failed: %v", err)
    }

    media, err := db.SearchMedia(platform, "", nil, 10, 0)
    if len(media) != expectedCount {
        t.Errorf("Expected %d media items, got %d", expectedCount, len(media))
    }
}
```

## Documentation

- Update README.md for user-facing changes
- Update code comments for API changes
- Add examples for new features
- Keep EXAMPLES.md current

## CI/CD

All pull requests must pass:
- ✅ All tests
- ✅ Linting checks
- ✅ Code formatting
- ✅ Security scans
- ✅ Build on all platforms

The CI runs automatically on:
- Every push
- Every pull request
- Tagged releases

## Release Process

1. Update version in relevant files
2. Update CHANGELOG
3. Create and push tag:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
4. GitHub Actions will automatically create release

## Getting Help

- Open an issue for questions
- Check existing issues and documentation
- Be patient and respectful

## License

By contributing, you agree that your contributions will be subject to the same license as the project.

## Recognition

Contributors will be acknowledged in:
- GitHub contributors list
- Release notes
- Project documentation

Thank you for contributing! 🎉
