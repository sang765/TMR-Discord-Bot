package utils

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// getFFmpegPath returns the path to ffmpeg binary
// Checks local directory first, then falls back to system ffmpeg
func getFFmpegPath() string {
	// Check local directory first
	localPath := "./ffmpeg"
	if _, err := exec.LookPath(localPath); err == nil {
		return localPath
	}
	
	// Check with absolute path
	absPath, _ := filepath.Abs(localPath)
	if _, err := exec.LookPath(absPath); err == nil {
		return absPath
	}
	
	// Fallback to system ffmpeg
	return "ffmpeg"
}

// getFFprobePath returns the path to ffprobe binary
func getFFprobePath() string {
	localPath := "./ffprobe"
	if _, err := exec.LookPath(localPath); err == nil {
		return localPath
	}
	return "ffprobe"
}

// findBestLoopPoint analyzes video and finds where frame matches the first frame
// Returns the duration (seconds) to cut the video for optimal looping
// Uses 2-pass: coarse scan (0.2s) then fine scan (0.02s) around best match
func findBestLoopPoint(data []byte, maxDuration float64) float64 {
	// Get video duration first
	duration := getVideoDuration(data)
	if duration <= 0 || duration > maxDuration {
		duration = maxDuration
	}
	
	// Extract first frame
	firstFrame := extractFrameAt(data, 0)
	if firstFrame == nil {
		return duration
	}
	
	// Skip first 20% of video to avoid finding near-start frames
	minSearchTime := duration * 0.2
	if minSearchTime < 0.5 {
		minSearchTime = 0.5
	}
	
	// Pass 1: Coarse scan every 0.2s (skip early frames)
	bestCoarseTime := duration
	bestCoarseSim := 1.0
	
	for t := minSearchTime; t < duration-0.2; t += 0.2 {
		frame := extractFrameAt(data, t)
		if frame == nil {
			continue
		}
		
		similarity := compareFrames(firstFrame, frame)
		if similarity < bestCoarseSim {
			bestCoarseSim = similarity
			bestCoarseTime = t
			
			if similarity < 0.03 { // Very strict - 3% difference
				break
			}
		}
	}
	
	// Pass 2: Fine scan around best coarse match (±0.2s, step 0.02s)
	startTime := bestCoarseTime - 0.2
	if startTime < minSearchTime {
		startTime = minSearchTime
	}
	endTime := bestCoarseTime + 0.2
	if endTime >= duration {
		endTime = duration - 0.05
	}
	
	bestMatchTime := bestCoarseTime
	bestSimilarity := bestCoarseSim
	
	for t := startTime; t <= endTime; t += 0.02 {
		frame := extractFrameAt(data, t)
		if frame == nil {
			continue
		}
		
		similarity := compareFrames(firstFrame, frame)
		if similarity < bestSimilarity {
			bestSimilarity = similarity
			bestMatchTime = t
			
			if similarity < 0.01 { // Nearly identical - 1% difference
				break
			}
		}
	}
	
	// Log the loop point quality
	fmt.Printf("[Loop] Best loop at %.2fs with similarity %.4f (%.2f%% diff)\n", 
		bestMatchTime, bestSimilarity, bestSimilarity*100)
	
	// If best match is still bad (> 15% diff), don't trim - video might not loop naturally
	if bestSimilarity > 0.15 {
		fmt.Printf("[Loop] No good loop point found, using full duration\n")
		return duration
	}
	
	return bestMatchTime
}
// getVideoDuration extracts video duration in seconds using ffprobe
func getVideoDuration(data []byte) float64 {
	ffprobePath := getFFprobePath()
	
	// Write to temp file (pipe:0 unreliable on static ffprobe)
	tmpFile, err := os.CreateTemp("", "moewalls-probe-*.webm")
	if err != nil {
		return 0
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return 0
	}
	tmpFile.Close()
	
	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		tmpPath,
	)

	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}

	return duration
}

// getVideoInfo returns video resolution and codec info
func getVideoInfo(data []byte) (width, height int, codec string) {
	ffprobePath := getFFprobePath()
	
	// Write to temp file (pipe:0 unreliable on static ffprobe)
	tmpFile, err := os.CreateTemp("", "moewalls-info-*.webm")
	if err != nil {
		return 0, 0, "unknown"
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return 0, 0, "unknown"
	}
	tmpFile.Close()
	
	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,codec_name",
		"-of", "csv=p=0",
		tmpPath,
	)

	out, err := cmd.Output()
	if err != nil {
		return 0, 0, "unknown"
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) >= 3 {
		width, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
		height, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		codec = strings.TrimSpace(parts[2])
	}

	return width, height, codec
}
// extractFrameAt extracts a single frame from video at given time
// Returns RGBA image data
func extractFrameAt(data []byte, timeSec float64) *image.RGBA {
	ffmpegPath := getFFmpegPath()
	
	// Write to temp file (pipe:0 unreliable on static ffmpeg)
	tmpFile, err := os.CreateTemp("", "moewalls-frame-*.webm")
	if err != nil {
		return nil
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil
	}
	tmpFile.Close()
	
	// Output to temp PNG
	outFile, err := os.CreateTemp("", "moewalls-frame-*.png")
	if err != nil {
		return nil
	}
	outPath := outFile.Name()
	defer os.Remove(outPath)
	outFile.Close()
	
	cmd := exec.Command(ffmpegPath,
		"-ss", fmt.Sprintf("%.2f", timeSec),
		"-i", tmpPath,
		"-vframes", "1",
		"-f", "image2",
		"-v", "error",
		"-y",
		outPath,
	)
	
	if err := cmd.Run(); err != nil {
		return nil
	}
	
	imgData, err := os.ReadFile(outPath)
	if err != nil {
		return nil
	}
	
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil
	}
	
	// Convert to RGBA for comparison
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}

// compareFrames calculates similarity between two frames (0 = identical, 1 = completely different)
// Uses downscaled comparison for speed with structural similarity
func compareFrames(a, b *image.RGBA) float64 {
	if a == nil || b == nil {
		return 1.0
	}
	
	// Downscale to 128x128 for better accuracy
	size := 128
	aSmall := downscaleImage(a, size)
	bSmall := downscaleImage(b, size)
	
	if aSmall == nil || bSmall == nil {
		return 1.0
	}
	
	// Calculate normalized MSE with focus on structural similarity
	var sumDiff float64
	var sumA float64
	var sumB float64
	pixels := float64(size * size * 4) // RGBA = 4 channels
	
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c1 := aSmall.RGBAAt(x, y)
			c2 := bSmall.RGBAAt(x, y)
			
			dr := float64(c1.R - c2.R)
			dg := float64(c1.G - c2.G)
			db := float64(c1.B - c2.B)
			da := float64(c1.A - c2.A)
			
			sumDiff += dr*dr + dg*dg + db*db + da*da
			sumA += float64(c1.R) + float64(c1.G) + float64(c1.B) + float64(c1.A)
			sumB += float64(c2.R) + float64(c2.G) + float64(c2.B) + float64(c2.A)
		}
	}
	
	mse := sumDiff / pixels
	
	// Also check if overall brightness is similar
	avgA := sumA / pixels
	avgB := sumB / pixels
	brightnessDiff := math.Abs(avgA-avgB) / 255.0
	
	// Combine MSE and brightness difference
	return math.Sqrt(mse)/255.0*0.8 + brightnessDiff*0.2
}

// downscaleImage downscales image to target size using nearest neighbor
func downscaleImage(src *image.RGBA, targetSize int) *image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	
	if srcW == 0 || srcH == 0 {
		return nil
	}
	
	dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
	
	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
			srcX := bounds.Min.X + x*srcW/targetSize
			srcY := bounds.Min.Y + y*srcH/targetSize
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	
	return dst
}

// TrimVideoToDuration cuts video to specified duration
func TrimVideoToDuration(data []byte, duration float64) ([]byte, error) {
	ffmpegPath := getFFmpegPath()
	
	cmd := exec.Command(ffmpegPath,
		"-i", "pipe:0",
		"-t", fmt.Sprintf("%.2f", duration),
		"-c", "copy",
		"-y",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(data)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}
	
	if err := cmd.Run(); err != nil {
		return data, nil // Return original on error
	}
	
	return out.Bytes(), nil
}

// ResizeVideo resizes a video to the specified dimensions using ffmpeg
// Returns the resized video bytes
func ResizeVideo(data []byte, width, height int) ([]byte, error) {
	ffmpegPath := getFFmpegPath()
	
	// Check if ffmpeg is available
	if _, err := exec.LookPath(ffmpegPath); err != nil {
		return nil, fmt.Errorf("ffmpeg not installed")
	}

	cmd := exec.Command(ffmpegPath,
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2", width, height, width, height),
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-y",
		"pipe:1",
	)

	cmd.Stdin = bytes.NewReader(data)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg resize failed: %w", err)
	}

	return out.Bytes(), nil
}

// VideoToCompressedGIF converts a video to GIF and compresses to under maxBytes
// Uses decreasing quality and fps until size is acceptable
func VideoToCompressedGIF(data []byte, maxBytes int) ([]byte, error) {
	ffmpegPath := getFFmpegPath()
	
	if _, err := exec.LookPath(ffmpegPath); err != nil {
		return nil, fmt.Errorf("ffmpeg not installed")
	}

	// Validate video data first
	if len(data) < 100 {
		return nil, fmt.Errorf("video data too small (%d bytes), likely corrupted", len(data))
	}

	// Check if it's a valid video file (skip first 4KB for container header)
	// Look for ftyp, moov, or RIFF markers
	header := string(data[:min(4096, len(data))])
	if !strings.Contains(header, "ftyp") && !strings.Contains(header, "moov") && !strings.Contains(header, "RIFF") && !strings.Contains(header, "webm") {
		return nil, fmt.Errorf("invalid video format (no ftyp/moov/RIFF marker)")
	}

	// Optimize loop point for better GIF looping
	loopDuration := findBestLoopPoint(data, 8.0) // Max 8 seconds for GIF
	if loopDuration < 2.0 {
		loopDuration = 2.0 // Minimum 2 seconds
	}
	
	// Trim video to optimal loop point
	trimmed, err := TrimVideoToDuration(data, loopDuration)
	if err == nil {
		data = trimmed
	}

	// Write to temp file for ffmpeg (pipe input has issues on some systems)
	tmpInput, err := os.CreateTemp("", "moe-input-*.webm")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpInput.Name())
	defer tmpInput.Close()

	if _, err := tmpInput.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpInput.Close()

	tmpOutput, err := os.CreateTemp("", "moe-output-*.gif")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp output: %w", err)
	}
	defer os.Remove(tmpOutput.Name())
	defer tmpOutput.Close()
	tmpOutput.Close()

	// Try different quality settings
	qualities := []struct {
		fps   string
		scale string
	}{
		{"15", "480:-1"},
		{"12", "400:-1"},
		{"10", "360:-1"},
		{"8", "320:-1"},
		{"6", "240:-1"},
		{"4", "200:-1"},
	}

	for _, q := range qualities {
		cmd := exec.Command(ffmpegPath,
			"-i", tmpInput.Name(),
			"-vf", fmt.Sprintf("fps=%s,scale=%s:flags=lanczos", q.fps, q.scale),
			"-loop", "0",
			"-y", tmpOutput.Name(),
		)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("[GIF] Quality %s/%s failed: %v\n", q.fps, q.scale, err)
			if stderr.Len() > 200 {
				// Get the last 500 chars which usually has the actual error
				fmt.Printf("[GIF] stderr (tail): %s\n", stderr.String()[max(0, stderr.Len()-500):])
			}
			continue
		}

		// Read the output
		gifData, err := os.ReadFile(tmpOutput.Name())
		if err != nil {
			continue
		}

		if len(gifData) <= maxBytes {
			return gifData, nil
		}
		fmt.Printf("[GIF] Quality %s/%s too large: %d bytes\n", q.fps, q.scale, len(gifData))
	}

	// If still over limit, return smallest version
	cmd := exec.Command(ffmpegPath,
		"-i", tmpInput.Name(),
		"-vf", "fps=3,scale=200:-1:flags=lanczos",
		"-loop", "0",
		"-y", tmpOutput.Name(),
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gif conversion failed: %w\nstderr: %s", err, stderr.String())
	}

	return os.ReadFile(tmpOutput.Name())
}
