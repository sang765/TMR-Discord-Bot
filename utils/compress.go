package utils

import (
	"bytes"
	"image"
	"image/jpeg"
)

const (
	maxBannerSize         = 10 * 1024 * 1024 // 10MB for static images
	maxBannerSizeAnimated = 10 * 1024 * 1024 // 10MB for animated GIFs
	startQuality          = 85
	minQuality            = 30
)

// CompressToUnderLimit compresses JPEG image to be under maxBannerSize
// Starts at quality 85 and decreases until size is acceptable
func CompressToUnderLimit(data []byte) ([]byte, error) {
	// If already under limit, return as-is
	if len(data) <= maxBannerSize {
		return data, nil
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// Try decreasing quality until under limit
	quality := startQuality
	for quality >= minQuality {
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
		if err != nil {
			return nil, err
		}

		// Check if under limit
		if buf.Len() <= maxBannerSize {
			return buf.Bytes(), nil
		}

		quality -= 10
	}

	// If still over limit at minQuality, return lowest quality result
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: minQuality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
