package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
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

	// Try different quality settings
	qualities := []struct {
		fps    string
		scale  string
		dither string
	}{
		{"15", "480:-1", "floyd_steinberg"},
		{"10", "360:-1", "floyd_steinberg"},
		{"8", "320:-1", "sierra2_4a"},
		{"5", "240:-1", "none"},
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
