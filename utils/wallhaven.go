package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type WallhavenResponse struct {
	Data []WallhavenImage `json:"data"`
	Meta WallhavenMeta    `json:"meta"`
}

type WallhavenImage struct {
	ID        string            `json:"id"`
	URL       string            `json:"url"`
	Path      string            `json:"path"`
	Width     int               `json:"dimension_x"`
	Height    int               `json:"dimension_y"`
	Resolution string           `json:"resolution"`
	Ratio     string            `json:"ratio"`
	FileType  string            `json:"file_type"`
	Thumbs    WallhavenThumbs   `json:"thumbs"`
	Colors    []string          `json:"colors"`
}

type WallhavenThumbs struct {
	Large    string `json:"large"`
	Original string `json:"original"`
	Small    string `json:"small"`
}

type WallhavenMeta struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
}

type WallhavenClient struct {
	APIKey     string
	Categories string
	Purity     string
	Sorting    string
	Ratio      string
	HTTPClient *http.Client
}

func NewWallhavenClient(apiKey, categories, purity, sorting, ratio string) *WallhavenClient {
	return &WallhavenClient{
		APIKey:     apiKey,
		Categories: categories,
		Purity:     purity,
		Sorting:    sorting,
		Ratio:      ratio,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *WallhavenClient) GetRandomImage() (*WallhavenImage, error) {
	url := fmt.Sprintf(
		"https://wallhaven.cc/api/v1/search?categories=%s&purity=%s&sorting=%s&ratio=%s",
		c.Categories,
		c.Purity,
		c.Sorting,
		c.Ratio,
	)

	if c.APIKey != "" {
		url += "&apikey=" + c.APIKey
	}

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from wallhaven: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result WallhavenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no images found")
	}

	return &result.Data[0], nil
}

func (c *WallhavenClient) GetRandomImages(count int) ([]WallhavenImage, error) {
	url := fmt.Sprintf(
		"https://wallhaven.cc/api/v1/search?categories=%s&purity=%s&sorting=%s&ratio=%s",
		c.Categories,
		c.Purity,
		c.Sorting,
		c.Ratio,
	)

	if c.APIKey != "" {
		url += "&apikey=" + c.APIKey
	}

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from wallhaven: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result WallhavenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no images found")
	}

	return result.Data, nil
}

func BuildCategoriesString(anime, general, people int) string {
	return fmt.Sprintf("%d%d%d", anime, general, people)
}

func BuildPurityString(sfw, sketchy, nsfw int) string {
	return fmt.Sprintf("%d%d%d", sfw, sketchy, nsfw)
}
