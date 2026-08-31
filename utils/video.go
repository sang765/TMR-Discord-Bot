package utils

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
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
	
	// Sample frames every 0.5 seconds, find closest match to first frame
	bestMatchTime := duration
	bestSimilarity := 1.0 // Lower = more similar
	
	sampleInterval := 0.5
	for t := sampleInterval; t < duration-0.5; t += sampleInterval {
		frame := extractFrameAt(data, t)
		if frame == nil {
			continue
		}
		
		similarity := compareFrames(firstFrame, frame)
		if similarity < bestSimilarity {
			bestSimilarity = similarity
			bestMatchTime = t
			
			// If very similar (>85%), use this as loop point
			if similarity < 0.15 {
				break
			}
		}
	}
	
	return bestMatchTime
}

// getVideoDuration extracts video duration in seconds using ffprobe
func getVideoDuration(data []byte) float64 {
	ffprobePath := getFFprobePath()
	
	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		"pipe:0",
	)
	cmd.Stdin = bytes.NewReader(data)
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

// extractFrameAt extracts a single frame from video at given time
// Returns RGBA image data
func extractFrameAt(data []byte, timeSec float64) *image.RGBA {
	ffmpegPath := getFFmpegPath()
	
	cmd := exec.Command(ffmpegPath,
		"-ss", fmt.Sprintf("%.2f", timeSec),
		"-i", "pipe:0",
		"-vframes", "1",
		"-f", "image2pipe",
		"-v", "error",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	
	img, _, err := image.Decode(bytes.NewReader(out))
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
// Uses downscaled comparison for speed
func compareFrames(a, b *image.RGBA) float64 {
	if a == nil || b == nil {
		return 1.0
	}
	
	// Downscale to 64x64 for fast comparison
	size := 64
	aSmall := downscaleImage(a, size)
	bSmall := downscaleImage(b, size)
	
	if aSmall == nil || bSmall == nil {
		return 1.0
	}
	
	// Calculate normalized MSE
	var sumDiff float64
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
		}
	}
	
	mse := sumDiff / pixels
	// Normalize to 0-1 range (max possible diff is 255^2 = 65025)
	return math.Sqrt(mse) / 255.0
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

	// Try different quality settings
	qualities := []struct {
		fps    string
		scale  string
		dither string
	}{
		{"30", "480:-1", "floyd_steinberg"},
		{"25", "480:-1", "floyd_steinberg"},
		{"20", "400:-1", "floyd_steinberg"},
		{"15", "360:-1", "floyd_steinberg"},
		{"10", "320:-1", "sierra2_4a"},
		{"8", "240:-1", "none"},
	}

	for _, q := range qualities {
		cmd := exec.Command(ffmpegPath,
			"-i", "pipe:0",
			"-vf", fmt.Sprintf("fps=%s,scale=%s:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128:stats_mode=diff[p];[s1][p]paletteuse=dither=%s", q.fps, q.scale, q.dither),
			"-loop", "0",
			"-y",
			"pipe:1",
		)

		cmd.Stdin = bytes.NewReader(data)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &bytes.Buffer{}

		if err := cmd.Run(); err != nil {
			continue
		}

		gifData := out.Bytes()
		if len(gifData) <= maxBytes {
			return gifData, nil
		}
	}

	// If still over limit, return smallest version
	cmd := exec.Command(ffmpegPath,
		"-i", "pipe:0",
		"-vf", "fps=3,scale=200:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=64:stats_mode=diff[p];[s1][p]paletteuse=dither=none",
		"-loop", "0",
		"-y",
		"pipe:1",
	)

	cmd.Stdin = bytes.NewReader(data)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gif conversion failed: %w", err)
	}

	return out.Bytes(), nil
}
