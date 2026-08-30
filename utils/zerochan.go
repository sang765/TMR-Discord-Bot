package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

const zerochanBaseURL = "https://www.zerochan.net"

type ZerochanEntry struct {
	ID      int      `json:"id"`
	Primary string   `json:"primary"`
	Tags    []string `json:"tags"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Fav     int      `json:"fav"`
	Source  string   `json:"source"`
	Full    string   `json:"full"`
	Medium  string   `json:"medium"`
	Small   string   `json:"small"`
	Anime   string   `json:"anime"`
}

// ZerochanResponse handles both array and object responses
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
	for _, key := range []string{"items", "entries", "data", "results", "list"} {
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
	// Rate limit: 60 req/min, wait at least 1 second between requests
	elapsed := time.Since(c.LastRequest)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}

	var apiURL string
	tags := c.Tags
	if tags != "" {
		// ZeroChan uses Title Case tags with spaces
		encodedTag := url.PathEscape(tags)
		apiURL = fmt.Sprintf("%s/%s?json&s=id&l=50", zerochanBaseURL, encodedTag)
	} else {
		// Browse all entries, sorted by newest
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

	return &response.Entries[rand.Intn(len(response.Entries))], nil
}

func (c *ZerochanEntry) GetImageURL() string {
	if c.Full != "" {
		return c.Full
	}
	if c.Medium != "" {
		return c.Medium
	}
	return c.Small
}
