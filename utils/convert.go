package utils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// VideoToGIF converts a video file to GIF format
// Returns the path to the generated GIF file
func VideoToGIF(inputPath string, width, fps int) (string, error) {
	if width <= 0 {
		width = 320
	}
	if fps <= 0 {
		fps = 15
	}

	// Generate output path
	ext := filepath.Ext(inputPath)
	outputPath := inputPath[:len(inputPath)-len(ext)] + ".gif"

	// FFmpeg command to convert video to GIF
	// -y: overwrite output
	// -i: input file
	// -vf: video filter (scale + palettegen for better quality)
	// -loop: loop count (0 = infinite)
	cmd := exec.Command("ffmpeg", "-y",
		"-i", inputPath,
		"-vf", fmt.Sprintf("scale=%d:-1:flags=lanczos,fps=%d", width, fps),
		"-loop", "0",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w, output: %s", err, string(output))
	}

	return outputPath, nil
}

// VideoToGIFWithPalette converts video to GIF with palette for better quality
func VideoToGIFWithPalette(inputPath string, width, fps int, duration time.Duration) (string, error) {
	if width <= 0 {
		width = 320
	}
	if fps <= 0 {
		fps = 15
	}

	ext := filepath.Ext(inputPath)
	basePath := inputPath[:len(inputPath)-len(ext)]
	palettePath := basePath + "_palette.png"
	outputPath := basePath + ".gif"

	// Step 1: Generate palette
	genPalette := exec.Command("ffmpeg", "-y",
		"-i", inputPath,
		"-vf", fmt.Sprintf("scale=%d:-1:flags=lanczos,fps=%d,palettegen=stats_mode=diff", width, fps),
		palettePath,
	)
	if output, err := genPalette.CombinedOutput(); err != nil {
		return "", fmt.Errorf("palette generation failed: %w, output: %s", err, string(output))
	}

	// Step 2: Convert with palette
	convertCmd := exec.Command("ffmpeg", "-y",
		"-i", inputPath,
		"-i", palettePath,
		"-lavfi", fmt.Sprintf("scale=%d:-1:flags=lanczos,fps=%d [x]; [x][1:v] paletteuse=dither=bayer:bayer_scale=3", width, fps),
		"-loop", "0",
		outputPath,
	)
	if output, err := convertCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("gif conversion failed: %w, output: %s", err, string(output))
	}

	// Clean up palette file
	exec.Command("rm", "-f", palettePath).Run()

	return outputPath, nil
}

// DownloadAndConvertToGIF downloads a video and converts it to GIF
func DownloadAndConvertToGIF(videoURL string, width, fps int) ([]byte, error) {
	// Download video to temp file
	tmpDir := "/tmp"
	tmpVideo := fmt.Sprintf("%s/moewalls_%d.mp4", tmpDir, time.Now().UnixNano())

	if err := downloadFile(videoURL, tmpVideo); err != nil {
		return nil, fmt.Errorf("failed to download video: %w", err)
	}
	defer exec.Command("rm", "-f", tmpVideo).Run()

	// Convert to GIF
	gifPath, err := VideoToGIFWithPalette(tmpVideo, width, fps, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to GIF: %w", err)
	}
	defer exec.Command("rm", "-f", gifPath).Run()

	// Read GIF file
	return readFile(gifPath)
}

func downloadFile(url, outputPath string) error {
	cmd := exec.Command("curl", "-L", "-o", outputPath, url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("download failed: %w, output: %s", err, string(output))
	}
	return nil
}

func readFile(path string) ([]byte, error) {
	cmd := exec.Command("cat", path)
	return cmd.Output()
}
