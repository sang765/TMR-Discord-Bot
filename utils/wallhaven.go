package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

const wallhavenBaseURL = "https://wallhaven.cc/api/v1"

type WallhavenEntry struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Path        string `json:"path"`
	Resolution  string `json:"resolution"`
	Category    string `json:"category"`
	Purity      string `json:"purity"`
	FileType    string `json:"file_type"`
	Favorites   int    `json:"favorites"`
	Views       int    `json:"views"`
	Source      string `json:"source"`
	CreatedAt   string `json:"created_at"`
	DimensionX  int    `json:"dimension_x"`
	DimensionY  int    `json:"dimension_y"`
	Thumbs      struct {
		Large    string `json:"large"`
		Original string `json:"original"`
		Small    string `json:"small"`
	} `json:"thumbs"`
}

type WallhavenResponse struct {
	Data []WallhavenEntry `json:"data"`
	Meta struct {
		Total int `json:"total"`
	} `json:"meta"`
}

type WallhavenClient struct {
	Tags        string
	HTTPClient  *http.Client
	LastRequest time.Time
}

func NewWallhavenClient(tags string) *WallhavenClient {
	return &WallhavenClient{
		Tags: tags,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *WallhavenClient) GetRandomImage() (*WallhavenEntry, error) {
	// Rate limit
	elapsed := time.Since(c.LastRequest)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}

	// categories=010 = anime only (general=100, anime=010, people=001)
	apiURL := fmt.Sprintf("%s/search?q=%s&sorting=random&limit=100&purity=100&categories=010", wallhavenBaseURL, c.Tags)

	resp, err := c.HTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from wallhaven: %w", err)
	}
	defer resp.Body.Close()
	c.LastRequest = time.Now()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("wallhaven rate limit exceeded")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wallhaven returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallhaven response: %w", err)
	}

	var response WallhavenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse wallhaven JSON: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no images found on wallhaven")
	}

	return &response.Data[rand.Intn(len(response.Data))], nil
}

func (c *WallhavenEntry) GetImageURL() string {
	if c.Path != "" {
		return c.Path
	}
	if c.Thumbs.Original != "" {
		return c.Thumbs.Original
	}
	return c.URL
}
