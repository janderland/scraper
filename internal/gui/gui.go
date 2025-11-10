package gui

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/sahilm/fuzzy"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/storage"
	"github.com/janderland/scraper/internal/tags"
)

// GUI represents the graphical user interface
type GUI struct {
	app         fyne.App
	window      fyne.Window
	db          *storage.Database
	tagger      *tags.Tagger
	currentPage int
	pageSize    int
	mediaItems  []*models.Media
	selectedTags []int64
	searchTerm   string
}

// NewGUI creates a new GUI
func NewGUI(db *storage.Database, tagger *tags.Tagger) *GUI {
	a := app.New()
	w := a.NewWindow("Media Scraper Browser")
	w.Resize(fyne.NewSize(1200, 800))

	return &GUI{
		app:         a,
		window:      w,
		db:          db,
		tagger:      tagger,
		currentPage: 0,
		pageSize:    20,
	}
}

// Run starts the GUI
func (g *GUI) Run() {
	g.window.SetContent(g.buildMainUI())
	g.window.ShowAndRun()
}

// buildMainUI builds the main user interface
func (g *GUI) buildMainUI() fyne.CanvasObject {
	// Search bar
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search media...")
	searchEntry.OnChanged = func(text string) {
		g.searchTerm = text
		g.currentPage = 0
		g.refreshMediaGrid()
	}

	// Platform filter
	platformSelect := widget.NewSelect(
		[]string{"All", "Reddit", "Instagram", "Facebook"},
		func(selected string) {
			g.currentPage = 0
			g.refreshMediaGrid()
		},
	)
	platformSelect.SetSelected("All")

	// Tag filter
	tagSelect := g.createTagFilter()

	// Filter controls
	filterBox := container.NewBorder(
		nil, nil,
		widget.NewLabel("Search:"),
		container.NewHBox(
			widget.NewLabel("Platform:"),
			platformSelect,
			widget.NewLabel("Tags:"),
			tagSelect,
		),
		searchEntry,
	)

	// Media grid
	mediaGrid := g.createMediaGrid()

	// Navigation buttons
	prevButton := widget.NewButton("Previous", func() {
		if g.currentPage > 0 {
			g.currentPage--
			g.refreshMediaGrid()
		}
	})

	nextButton := widget.NewButton("Next", func() {
		g.currentPage++
		g.refreshMediaGrid()
	})

	pageLabel := widget.NewLabel(fmt.Sprintf("Page %d", g.currentPage+1))

	navBox := container.NewHBox(
		prevButton,
		pageLabel,
		nextButton,
	)

	// Tag management button
	manageTagsButton := widget.NewButton("Manage Tags", func() {
		g.showTagManagement()
	})

	// Stats display
	statsLabel := widget.NewLabel("Loading...")
	g.updateStatsLabel(statsLabel)

	// Toolbar
	toolbar := container.NewBorder(
		nil, nil,
		manageTagsButton,
		statsLabel,
		widget.NewLabel("Media Browser"),
	)

	// Main layout
	content := container.NewBorder(
		container.NewVBox(toolbar, filterBox),
		navBox,
		nil,
		nil,
		mediaGrid,
	)

	return content
}

// createMediaGrid creates the media grid display
func (g *GUI) createMediaGrid() *fyne.Container {
	// Load initial media
	media, err := g.db.SearchMedia("", g.searchTerm, g.selectedTags, g.pageSize, g.currentPage*g.pageSize)
	if err != nil {
		fmt.Printf("Error loading media: %v\n", err)
		return container.NewVBox(widget.NewLabel("Error loading media"))
	}

	g.mediaItems = media

	// Create grid
	grid := container.New(layout.NewGridLayout(4))

	for _, m := range media {
		mediaItem := m // Capture for closure
		thumbnail := g.createThumbnail(mediaItem)

		// Click handler
		thumbnail.OnTapped = func() {
			g.showMediaDetail(mediaItem)
		}

		grid.Add(thumbnail)
	}

	// Fill empty cells
	for len(grid.Objects) < g.pageSize {
		grid.Add(widget.NewLabel(""))
	}

	return container.NewScroll(grid)
}

// createThumbnail creates a thumbnail widget for a media item
func (g *GUI) createThumbnail(media *models.Media) *widget.Button {
	// Load image
	var thumbnail string
	if media.LocalPath != "" && g.fileExists(media.LocalPath) {
		thumbnail = media.LocalPath
	}

	// Create button with image preview
	label := fmt.Sprintf("%s\n%s", media.Title, media.Platform)
	if len(label) > 50 {
		label = label[:47] + "..."
	}

	btn := widget.NewButton(label, nil)
	return btn
}

// createTagFilter creates the tag filter dropdown
func (g *GUI) createTagFilter() *widget.Select {
	allTags, err := g.tagger.GetAllTags()
	if err != nil {
		fmt.Printf("Error loading tags: %v\n", err)
		return widget.NewSelect([]string{}, nil)
	}

	tagNames := []string{"All Tags"}
	for _, tag := range allTags {
		tagNames = append(tagNames, tag.Name)
	}

	return widget.NewSelect(tagNames, func(selected string) {
		if selected == "All Tags" {
			g.selectedTags = nil
		} else {
			// Find tag ID
			for _, tag := range allTags {
				if tag.Name == selected {
					g.selectedTags = []int64{tag.ID}
					break
				}
			}
		}
		g.currentPage = 0
		g.refreshMediaGrid()
	})
}

// showMediaDetail shows detailed view of a media item
func (g *GUI) showMediaDetail(media *models.Media) {
	detailWindow := g.app.NewWindow(media.Title)
	detailWindow.Resize(fyne.NewSize(800, 600))

	// Load and display image
	var img *canvas.Image
	if media.LocalPath != "" && g.fileExists(media.LocalPath) {
		img = canvas.NewImageFromFile(media.LocalPath)
		img.FillMode = canvas.ImageFillContain
	} else {
		img = canvas.NewImageFromFile("") // Placeholder
	}

	// Metadata display
	metadataText := fmt.Sprintf(`
Platform: %s
Type: %s
Author: %s
Upvotes: %d
Posted: %s
Downloaded: %s
URL: %s
Source: %s
`,
		media.Platform,
		media.MediaType,
		media.Author,
		media.Upvotes,
		media.PostedAt.Format("2006-01-02 15:04"),
		media.DownloadedAt.Format("2006-01-02 15:04"),
		media.URL,
		media.SourceURL,
	)

	if media.Description != "" {
		metadataText += fmt.Sprintf("\nDescription: %s", media.Description)
	}

	metadata := widget.NewLabel(metadataText)
	metadataScroll := container.NewScroll(metadata)

	// Tags display
	mediaTags, _ := g.tagger.GetMediaTags(media.ID)
	tagLabels := []string{}
	for _, tag := range mediaTags {
		tagLabels = append(tagLabels, tag.Name)
	}
	tagsText := "Tags: " + strings.Join(tagLabels, ", ")
	tagsLabel := widget.NewLabel(tagsText)

	// Add tag button
	addTagButton := widget.NewButton("Add Tag", func() {
		g.showAddTagDialog(media, detailWindow)
	})

	// Fullscreen button
	fullscreenButton := widget.NewButton("Fullscreen", func() {
		g.showFullscreen(media)
	})

	// Layout
	sidebar := container.NewVBox(
		metadataScroll,
		tagsLabel,
		addTagButton,
		fullscreenButton,
	)

	split := container.NewHSplit(
		img,
		sidebar,
	)
	split.SetOffset(0.7)

	detailWindow.SetContent(split)
	detailWindow.Show()
}

// showFullscreen shows media in fullscreen
func (g *GUI) showFullscreen(media *models.Media) {
	fullscreenWindow := g.app.NewWindow("Fullscreen - " + media.Title)
	fullscreenWindow.SetFullScreen(true)

	var img *canvas.Image
	if media.LocalPath != "" && g.fileExists(media.LocalPath) {
		img = canvas.NewImageFromFile(media.LocalPath)
		img.FillMode = canvas.ImageFillContain
	} else {
		img = canvas.NewImageFromFile("")
	}

	// Close on click
	closeButton := widget.NewButton("Close (ESC)", func() {
		fullscreenWindow.Close()
	})

	content := container.NewBorder(
		nil,
		closeButton,
		nil,
		nil,
		img,
	)

	fullscreenWindow.SetContent(content)
	fullscreenWindow.Show()
}

// showAddTagDialog shows dialog to add a tag
func (g *GUI) showAddTagDialog(media *models.Media, parent fyne.Window) {
	allTags, err := g.tagger.GetAllTags()
	if err != nil {
		dialog.ShowError(err, parent)
		return
	}

	tagNames := []string{}
	for _, tag := range allTags {
		tagNames = append(tagNames, tag.Name)
	}

	tagSelect := widget.NewSelect(tagNames, nil)

	dialog.ShowForm("Add Tag", "Add", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Tag", tagSelect),
	}, func(confirmed bool) {
		if confirmed && tagSelect.Selected != "" {
			// Find tag ID
			for _, tag := range allTags {
				if tag.Name == tagSelect.Selected {
					if err := g.tagger.AddManualTag(media.ID, tag.ID); err != nil {
						dialog.ShowError(err, parent)
					}
					break
				}
			}
		}
	}, parent)
}

// showTagManagement shows the tag management interface
func (g *GUI) showTagManagement() {
	tagWindow := g.app.NewWindow("Manage Tags")
	tagWindow.Resize(fyne.NewSize(600, 400))

	allTags, err := g.tagger.GetAllTags()
	if err != nil {
		dialog.ShowError(err, tagWindow)
		return
	}

	// Tag list
	tagList := widget.NewList(
		func() int { return len(allTags) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			tag := allTags[i]
			text := tag.Name
			if tag.RegexRule != "" {
				text += fmt.Sprintf(" (regex: %s)", tag.RegexRule)
			}
			label.SetText(text)
		},
	)

	// Add tag button
	addButton := widget.NewButton("Add Tag", func() {
		g.showAddTagForm(tagWindow)
	})

	content := container.NewBorder(
		addButton,
		nil,
		nil,
		nil,
		tagList,
	)

	tagWindow.SetContent(content)
	tagWindow.Show()
}

// showAddTagForm shows form to add a new tag
func (g *GUI) showAddTagForm(parent fyne.Window) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Tag name")

	regexEntry := widget.NewEntry()
	regexEntry.SetPlaceHolder("Regex rule (optional)")

	dialog.ShowForm("Add Tag", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Regex Rule", regexEntry),
	}, func(confirmed bool) {
		if confirmed && nameEntry.Text != "" {
			if _, err := g.tagger.CreateTag(nameEntry.Text, regexEntry.Text); err != nil {
				dialog.ShowError(err, parent)
			}
		}
	}, parent)
}

// refreshMediaGrid refreshes the media grid
func (g *GUI) refreshMediaGrid() {
	g.window.SetContent(g.buildMainUI())
}

// updateStatsLabel updates the statistics label
func (g *GUI) updateStatsLabel(label *widget.Label) {
	// This would query the database for stats
	label.SetText("Total: 0 images, 0 videos")
}

// fileExists checks if a file exists
func (g *GUI) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FuzzySearch performs fuzzy search on media metadata
func (g *GUI) FuzzySearch(query string) ([]*models.Media, error) {
	// Get all media
	allMedia, err := g.db.SearchMedia("", "", nil, 10000, 0)
	if err != nil {
		return nil, err
	}

	// Build searchable strings
	type searchItem struct {
		media  *models.Media
		text   string
	}

	var items []searchItem
	for _, m := range allMedia {
		text := fmt.Sprintf("%s %s %s %s",
			m.Title, m.Description, m.Author, m.Platform)
		items = append(items, searchItem{media: m, text: text})
	}

	// Perform fuzzy search
	matches := fuzzy.Find(query, []string{})

	// Simple contains search as fallback
	var results []*models.Media
	lowerQuery := strings.ToLower(query)
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.text), lowerQuery) {
			results = append(results, item.media)
		}
	}

	return results, nil
}
