package utils

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	moewallsBaseURL     = "https://moewalls.com"
	moewallsAnimeURL    = moewallsBaseURL + "/category/anime/"
	moewallsGameURL     = moewallsBaseURL + "/category/game/"
)

type MoeWallsClient struct {
	httpClient *http.Client
	maxPages   map[string]int // Cache max pages per category
}

type MoeWallsEntry struct {
	Title     string
	PageURL   string
	VideoURL  string
	Thumbnail string
}

func NewMoeWallsClient() *MoeWallsClient {
	return &MoeWallsClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxPages: make(map[string]int),
	}
}

// getMaxPages returns the max pages for a category (with caching)
func (mc *MoeWallsClient) getMaxPages(categoryURL string) int {
	// Check cache first
	if maxPages, ok := mc.maxPages[categoryURL]; ok {
		return maxPages
	}

	// Detect max pages from the category page
	maxPages := mc.detectMaxPages(categoryURL)
	mc.maxPages[categoryURL] = maxPages
	return maxPages
}

// detectMaxPages scrapes the last page number from a category page
func (mc *MoeWallsClient) detectMaxPages(categoryURL string) int {
	req, err := http.NewRequest("GET", categoryURL, nil)
	if err != nil {
		return 50 // Default fallback
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return 50 // Default fallback
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 50
	}

	// Look for pagination links like /category/anime/page/285/
	re := regexp.MustCompile(`/page/(\d+)/`)
	matches := re.FindAllStringSubmatch(string(body), -1)
	
	maxPage := 1
	for _, match := range matches {
		if len(match) >= 2 {
			pageNum, err := strconv.Atoi(match[1])
			if err == nil && pageNum > maxPage {
				maxPage = pageNum
			}
		}
	}

	fmt.Printf("[MoeWalls] Detected max pages for %s: %d\n", categoryURL, maxPage)
	return maxPage
}

// MoeWallsResult contains the result of getting a random video
type MoeWallsResult struct {
	GIFData      []byte
	Compressed   bool
	WallpaperURL string
	VideoURL     string
}

// GetRandomVideo downloads a random anime video from MoeWalls
// Returns the video bytes and whether it was compressed
func (mc *MoeWallsClient) GetRandomVideo() ([]byte, bool, error) {
	result, err := mc.GetRandomVideoWithStatus(nil)
	if err != nil {
		return nil, false, err
	}
	return result.GIFData, result.Compressed, nil
}

// GetRandomVideoDetailed downloads a random anime video and returns full result
func (mc *MoeWallsClient) GetRandomVideoDetailed() (*MoeWallsResult, error) {
	return mc.GetRandomVideoWithStatus(nil)
}

// GetRandomVideoWithStatus downloads a random anime video with status callback
func (mc *MoeWallsClient) GetRandomVideoWithStatus(statusFunc func(string)) (*MoeWallsResult, error) {
	updateStatus := func(msg string) {
		fmt.Printf("[MoeWalls] %s\n", msg)
		if statusFunc != nil {
			statusFunc(msg)
		}
	}

	// Step 1: Randomly pick category (80% anime, 20% game)
	categoryURL := moewallsAnimeURL
	categoryName := "anime"
	if rand.Intn(100) < 20 { // 20% chance for game
		categoryURL = moewallsGameURL
		categoryName = "game"
	}

	// Step 2: Get max pages for this category (cached)
	maxPages := mc.getMaxPages(categoryURL)

	// Step 3: Get random page
	pageNum := rand.Intn(maxPages) + 1
	var pageURL string
	if pageNum == 1 {
		pageURL = categoryURL
	} else {
		pageURL = fmt.Sprintf("%s/page/%d/", categoryURL, pageNum)
	}
	updateStatus(fmt.Sprintf("📡 **Category:** %s (%d pages)\n🔗 **Page:** %s", categoryName, maxPages, pageURL))

	// Step 4: Scrape list page to get wallpaper URLs
	wallpaperURLs, err := mc.scrapeListPage(pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape list page: %w", err)
	}
	if len(wallpaperURLs) == 0 {
		return nil, fmt.Errorf("no wallpapers found on page %d", pageNum)
	}
	updateStatus(fmt.Sprintf("📋 Found %d wallpapers", len(wallpaperURLs)))

	// Step 5: Pick random wallpaper
	randomIdx := rand.Intn(len(wallpaperURLs))
	wallpaperURL := wallpaperURLs[randomIdx]
	updateStatus(fmt.Sprintf("🎲 Picked wallpaper %d/%d\n🔗 **URL:** %s", randomIdx+1, len(wallpaperURLs), wallpaperURL))

	// Step 6: Scrape individual page to get video URL
	updateStatus("🔍 Finding video URL...")
	videoURL, err := mc.scrapeVideoURL(wallpaperURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get video URL: %w", err)
	}
	updateStatus(fmt.Sprintf("✅ Video URL found\n🔗 **Video:** %s", videoURL))

	// Step 7: Download video
	startDownload := time.Now()
	updateStatus("⬇️ Downloading video...")
	videoData, err := mc.downloadVideo(videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download video: %w", err)
	}
	downloadTime := time.Since(startDownload).Seconds()
	downloadSize := len(videoData) / 1024
	
	// Get video info
	vWidth, vHeight, vCodec := getVideoInfo(videoData)
	vDuration := getVideoDuration(videoData)
	
	updateStatus(fmt.Sprintf("✅ Downloaded %d KB in %.1fs\n📐 **Resolution:** %dx%d\n🎞️ **Codec:** %s\n⏱️ **Duration:** %.1fs",
		downloadSize, downloadTime, vWidth, vHeight, vCodec, vDuration))

	// Step 8: Resize to 1920x1080 if needed (using ffmpeg)
	startResize := time.Now()
	updateStatus("📐 Resizing video...")
	resized, err := ResizeVideo(videoData, 1920, 1080)
	if err != nil {
		// If resize fails, use original
		resized = videoData
	}
	resizeTime := time.Since(startResize).Seconds()
	resizedSize := len(resized) / 1024
	rWidth, rHeight, _ := getVideoInfo(resized)
	
	if resizeTime > 0.1 { // Only show resize info if it actually ran
		updateStatus(fmt.Sprintf("✅ Resized in %.1fs\n📐 **New resolution:** %dx%d\n📦 **Size:** %d KB",
			resizeTime, rWidth, rHeight, resizedSize))
	}

	// Step 9: Convert to GIF and compress to under 10MB
	startConvert := time.Now()
	updateStatus("🎬 Converting to GIF...")
	compressed, err := VideoToCompressedGIF(resized, maxBannerSizeAnimated)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to GIF: %w", err)
	}
	convertTime := time.Since(startConvert).Seconds()
	compressedSize := len(compressed) / 1024
	
	// Calculate compression ratio
	compressionRatio := 0.0
	if len(resized) > 0 {
		compressionRatio = (1.0 - float64(len(compressed))/float64(len(resized))) * 100
	}
	
	updateStatus(fmt.Sprintf("✅ GIF ready in %.1fs\n📦 **Size:** %d KB → %d KB (%.1f%% compressed)\n⏱️ **Total time:** %.1fs",
		convertTime, downloadSize, compressedSize, compressionRatio, 
		downloadTime+resizeTime+convertTime))

	// Check if compression was applied
	wasCompressed := len(compressed) < len(resized)

	return &MoeWallsResult{
		GIFData:      compressed,
		Compressed:   wasCompressed,
		WallpaperURL: wallpaperURL,
		VideoURL:     videoURL,
	}, nil
}

// scrapeListPage extracts wallpaper URLs from a list page
func (mc *MoeWallsClient) scrapeListPage(url string) ([]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var urls []string
	doc.Find("a[href*='/anime/']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.Contains(href, "/anime/") && strings.HasSuffix(href, "/") {
			// Exclude category pages and pagination
			if strings.Contains(href, "/category/") || strings.Contains(href, "/page/") {
				return
			}
			// Only include wallpaper pages (they have -live-wallpaper in URL)
			if !strings.Contains(href, "-live-wallpaper") {
				return
			}
			// Only add unique URLs
			if !contains(urls, href) {
				urls = append(urls, href)
			}
		}
	})

	return urls, nil
}

// scrapeVideoURL extracts the video URL from a wallpaper page
func (mc *MoeWallsClient) scrapeVideoURL(pageURL string) (string, error) {
	fmt.Printf("[MoeWalls] Scraping video URL from: %s\n", pageURL)
	
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		fmt.Printf("[MoeWalls] HTTP request failed: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	fmt.Printf("[MoeWalls] Response status: %d\n", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)
	fmt.Printf("[MoeWalls] Page size: %d bytes\n", len(bodyStr))

	// Pattern 1: <source src="/wp-content/uploads/preview/...webm"
	re := regexp.MustCompile(`src="(/wp-content/uploads/preview/[^"]+\.(webm|mp4))"`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) >= 2 {
		fmt.Printf("[MoeWalls] Pattern 1 matched: %s\n", matches[1])
		return moewallsBaseURL + matches[1], nil
	}

	// Pattern 2: Full URL with preview
	re2 := regexp.MustCompile(`src="(https?://[^"]+/preview/[^"]+\.(webm|mp4))"`)
	matches2 := re2.FindStringSubmatch(bodyStr)
	if len(matches2) >= 2 {
		fmt.Printf("[MoeWalls] Pattern 2 matched: %s\n", matches2[1])
		return matches2[1], nil
	}

	// Pattern 3: data-src or lazy load
	re3 := regexp.MustCompile(`data-src="(/wp-content/uploads/[^"]+\.(webm|mp4))"`)
	matches3 := re3.FindStringSubmatch(bodyStr)
	if len(matches3) >= 2 {
		fmt.Printf("[MoeWalls] Pattern 3 matched: %s\n", matches3[1])
		return moewallsBaseURL + matches3[1], nil
	}

	// Pattern 4: Any video tag with webm/mp4
	re4 := regexp.MustCompile(`"(https?://[^"]+\.(webm|mp4))"`)
	matches4 := re4.FindStringSubmatch(bodyStr)
	if len(matches4) >= 2 {
		fmt.Printf("[MoeWalls] Pattern 4 matched: %s\n", matches4[1])
		return matches4[1], nil
	}

	// Debug: show what we found
	fmt.Printf("[MoeWalls] No video URL found. Searching for 'preview' in page...\n")
	previewIdx := strings.Index(bodyStr, "preview")
	if previewIdx >= 0 {
		start := previewIdx - 100
		if start < 0 {
			start = 0
		}
		end := previewIdx + 200
		if end > len(bodyStr) {
			end = len(bodyStr)
		}
		fmt.Printf("[MoeWalls] Context around 'preview': ...%s...\n", bodyStr[start:end])
	} else {
		fmt.Printf("[MoeWalls] 'preview' not found in page\n")
		// Show first 1000 chars of body for debugging
		if len(bodyStr) > 1000 {
			fmt.Printf("[MoeWalls] First 1000 chars: %s\n", bodyStr[:1000])
		} else {
			fmt.Printf("[MoeWalls] Full body: %s\n", bodyStr)
		}
	}

	return "", fmt.Errorf("no video URL found on page")
}

// downloadVideo downloads the video file
func (mc *MoeWallsClient) downloadVideo(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetRandomPage returns a random page number for MoeWalls (anime category)
func (mc *MoeWallsClient) GetRandomPage() int {
	maxPages := mc.getMaxPages(moewallsAnimeURL)
	return rand.Intn(maxPages) + 1
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// parseResolution parses a resolution string like "1920x1080" into width and height
func parseResolution(res string) (int, int) {
	parts := strings.Split(res, "x")
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h
}
