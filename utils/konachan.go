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

type KonachanPost struct {
	ID       int    `json:"id"`
	FileURL  string `json:"file_url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Rating   string `json:"rating"`
	Tags     string `json:"tags"`
	Score    int    `json:"score"`
}

type KonachanClient struct {
	Tags       string
	Rating     string
	MinScore   int
	HTTPClient *http.Client
}

func NewKonachanClient(tags, rating string, minScore int) *KonachanClient {
	return &KonachanClient{
		Tags:     tags,
		Rating:   rating,
		MinScore: minScore,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *KonachanClient) buildTags() string {
	tags := ""
	if c.Tags != "" {
		tags = c.Tags
	}
	if c.Rating != "" {
		if tags != "" {
			tags += "+"
		}
		tags += "rating:" + c.Rating
	}
	if c.MinScore > 0 {
		if tags != "" {
			tags += "+"
		}
		tags += fmt.Sprintf("score:>%d", c.MinScore)
	}
	return tags
}

func (c *KonachanClient) GetRandomImage() (*KonachanPost, error) {
	tags := c.buildTags()

	apiURL := "https://konachan.net/post.json?limit=50"
	if tags != "" {
		apiURL += "&tags=" + url.QueryEscape(tags)
	}

	resp, err := c.HTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from konachan: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var posts []KonachanPost
	if err := json.Unmarshal(body, &posts); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(posts) == 0 {
		return nil, fmt.Errorf("no images found")
	}

	return &posts[rand.Intn(len(posts))], nil
}

func (c *KonachanClient) GetRandomImages(count int) ([]KonachanPost, error) {
	tags := c.buildTags()

	apiURL := fmt.Sprintf("https://konachan.net/post.json?limit=%d", count)
	if tags != "" {
		apiURL += "&tags=" + url.QueryEscape(tags)
	}

	resp, err := c.HTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from konachan: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var posts []KonachanPost
	if err := json.Unmarshal(body, &posts); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(posts) == 0 {
		return nil, fmt.Errorf("no images found")
	}

	return posts, nil
}
