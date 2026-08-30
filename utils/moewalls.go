package utils

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const moewallsBaseURL = "https://moewalls.com"

type MoeWallsEntry struct {
	Title     string
	PageURL   string
	Thumbnail string
	VideoURL  string
}

type MoeWallsClient struct {
	HTTPClient *http.Client
}

func NewMoeWallsClient() *MoeWallsClient {
	return &MoeWallsClient{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *MoeWallsClient) GetRandomWallpaper() (*MoeWallsEntry, error) {
	// Random page (1-286 based on research)
	page := rand.Intn(286) + 1
	url := fmt.Sprintf("%s/category/anime/page/%d/", moewallsBaseURL, page)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch moewalls page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("moewalls returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Find all wallpaper cards
	var entries []MoeWallsEntry
	doc.Find("li.g1-collection-item").Each(func(i int, s *goquery.Selection) {
		titleEl := s.Find("h3.entry-title a")
		thumbEl := s.Find("a.g1-frame img")

		title := strings.TrimSpace(titleEl.Text())
		pageURL, _ := titleEl.Attr("href")
		thumbnail, _ := thumbEl.Attr("src")

		if pageURL != "" {
			entries = append(entries, MoeWallsEntry{
				Title:     title,
				PageURL:   pageURL,
				Thumbnail: thumbnail,
			})
		}
	})

	if len(entries) == 0 {
		return nil, fmt.Errorf("no wallpapers found on moewalls")
	}

	entry := &entries[rand.Intn(len(entries))]

	// Fetch video URL from individual page
	if err := c.fetchVideoURL(entry); err != nil {
		// If video fetch fails, return entry with thumbnail only
		entry.VideoURL = entry.Thumbnail
	}

	return entry, nil
}

func (c *MoeWallsClient) fetchVideoURL(entry *MoeWallsEntry) error {
	req, err := http.NewRequest("GET", entry.PageURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Look for video source URL in the HTML
	// Pattern: <source src="..." type="video/mp4">
	content := string(body)
	if idx := strings.Index(content, `<source src="`); idx != -1 {
		start := idx + len(`<source src="`)
		end := strings.Index(content[start:], `"`)
		if end != -1 {
			videoURL := content[start : start+end]
			entry.VideoURL = videoURL
			return nil
		}
	}

	// Fallback: use thumbnail
	entry.VideoURL = entry.Thumbnail
	return fmt.Errorf("video URL not found, using thumbnail")
}
