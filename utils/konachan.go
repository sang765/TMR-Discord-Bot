package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
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
	ForumURL string `json:"forum_url"`
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

func (c *KonachanClient) GetRandomImage() (*KonachanPost, error) {
	tags := c.Tags
	if c.Rating != "" {
		tags += " rating:" + c.Rating
	}
	if c.MinScore > 0 {
		tags += fmt.Sprintf(" score:>%d", c.MinScore)
	}

	url := fmt.Sprintf(
		"https://konachan.com/post.json?limit=50&tags=%s",
		tags,
	)

	resp, err := c.HTTPClient.Get(url)
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
	tags := c.Tags
	if c.Rating != "" {
		tags += " rating:" + c.Rating
	}
	if c.MinScore > 0 {
		tags += fmt.Sprintf(" score:>%d", c.MinScore)
	}

	url := fmt.Sprintf(
		"https://konachan.com/post.json?limit=%d&tags=%s",
		count,
		tags,
	)

	resp, err := c.HTTPClient.Get(url)
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
