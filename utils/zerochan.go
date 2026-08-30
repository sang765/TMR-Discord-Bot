package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const zerochanBaseURL = "https://www.zerochan.net"

type ZerochanEntry struct {
	ID        int    `json:"id"`
	Primary   string `json:"primary"`
	Tags      any    `json:"tags"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Fav       int    `json:"fav"`
	Source    string `json:"source"`
	Full      string `json:"full"`
	Medium    string `json:"medium"`
	Small     string `json:"small"`
	Anime     string `json:"anime"`
	Thumbnail string `json:"thumbnail"`
	MD5       string `json:"md5"`
	Tag       string `json:"tag"`
	URL       string `json:"url"`
	Image     string `json:"image"`
	File      string `json:"file"`
}

type ZerochanResponse struct {
	Entries []ZerochanEntry
}

func (r *ZerochanResponse) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as array first
	var arr []ZerochanEntry
	if err := json.Unmarshal(data, &arr); err == nil {
		r.Entries = arr
		return nil
	}

	// Try to unmarshal as object with common field names
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// Check for common array fields
	for _, key := range []string{"items", "entries", "data", "results", "list", "posts"} {
		if arrRaw, ok := obj[key]; ok {
			arrData, _ := json.Marshal(arrRaw)
			var entries []ZerochanEntry
			if err := json.Unmarshal(arrData, &entries); err == nil {
				r.Entries = entries
				return nil
			}
		}
	}

	// If object has "id" field, it's a single entry
	if _, hasID := obj["id"]; hasID {
		var entry ZerochanEntry
		if err := json.Unmarshal(data, &entry); err == nil {
			r.Entries = []ZerochanEntry{entry}
			return nil
		}
	}

	return fmt.Errorf("unable to parse zerochan response")
}

type ZerochanClient struct {
	Tags        string
	HTTPClient  *http.Client
	ProjectName string
	Username    string
	LastRequest time.Time
}

func NewZerochanClient(tags, projectName, username string) *ZerochanClient {
	return &ZerochanClient{
		Tags:        tags,
		ProjectName: projectName,
		Username:    username,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *ZerochanClient) doRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("%s - %s", c.ProjectName, c.Username))
	return c.HTTPClient.Do(req)
}

func (c *ZerochanClient) GetRandomImage() (*ZerochanEntry, error) {
	elapsed := time.Since(c.LastRequest)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}

	var apiURL string
	tags := c.Tags
	if tags != "" {
		encodedTag := strings.ReplaceAll(tags, " ", "%20")
		apiURL = fmt.Sprintf("%s/%s?json&s=id&l=50", zerochanBaseURL, encodedTag)
	} else {
		apiURL = fmt.Sprintf("%s/?p=1&l=50&s=id&json", zerochanBaseURL)
	}

	resp, err := c.doRequest(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from zerochan: %w", err)
	}
	defer resp.Body.Close()
	c.LastRequest = time.Now()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("zerochan rate limit exceeded")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("zerochan returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read zerochan response: %w", err)
	}

	var response ZerochanResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse zerochan JSON: %w", err)
	}

	if len(response.Entries) == 0 {
		return nil, fmt.Errorf("no images found on zerochan")
	}

	entry := &response.Entries[rand.Intn(len(response.Entries))]

	// If no direct URL, try to scrape from page
	if entry.GetImageURL() == "" && entry.ID > 0 {
		fullURL, err := c.scrapeFullImageURL(entry.ID)
		if err == nil && fullURL != "" {
			entry.Full = fullURL
		}
	}

	return entry, nil
}

func (c *ZerochanClient) scrapeFullImageURL(id int) (string, error) {
	pageURL := fmt.Sprintf("%s/%d", zerochanBaseURL, id)

	resp, err := c.doRequest(pageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Look for full image URL in the page
	// ZeroChan pages have: <img id="image" src="..." />
	re := regexp.MustCompile(`id="image"[^>]*src="([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) > 1 {
		return string(matches[1]), nil
	}

	// Also try: data-src="..."
	re2 := regexp.MustCompile(`id="image"[^>]*data-src="([^"]+)"`)
	matches2 := re2.FindSubmatch(body)
	if len(matches2) > 1 {
		return string(matches2[1]), nil
	}

	return "", fmt.Errorf("could not find image URL in page")
}

func (c *ZerochanEntry) GetImageURL() string {
	// Try direct URL fields
	urls := []string{c.Full, c.URL, c.Image, c.File, c.Medium, c.Small}
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" && strings.HasPrefix(u, "http") {
			return u
		}
	}

	// Use thumbnail as fallback
	if c.Thumbnail != "" && strings.HasPrefix(c.Thumbnail, "http") {
		return c.Thumbnail
	}

	return ""
}
