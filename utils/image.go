package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
	"time"
)

func DownloadAndCropIcon(url string, size int) ([]byte, error) {
	data, err := downloadImageBytes(url)
	if err != nil {
		return nil, err
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	cropped := CropToSquare(img)

	resized := Resize(cropped, size, size)

	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "png":
		err = png.Encode(&buf, resized)
	default:
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return buf.Bytes(), nil
}

func CropToSquare(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	size := width
	if height < width {
		size = height
	}

	x0 := bounds.Min.X + (width-size)/2
	y0 := bounds.Min.Y + (height-size)/2

	srcRect := image.Rect(x0, y0, x0+size, y0+size)
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), img, srcRect.Min, draw.Src)

	return dst
}

func Resize(img *image.RGBA, width, height int) *image.RGBA {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + x*srcW/width
			srcY := bounds.Min.Y + y*srcH/height
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}

	return dst
}

func SmartCropIcon(url string, size int) ([]byte, error) {
	data, err := downloadImageBytes(url)
	if err != nil {
		return nil, err
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	cropped := CropToSquare(img)
	resized := Resize(cropped, size, size)

	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "png":
		err = png.Encode(&buf, resized)
	default:
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return buf.Bytes(), nil
}

func GetImageBounds(url string) (int, int, error) {
	data, err := downloadImageBytes(url)
	if err != nil {
		return 0, 0, err
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}

	return config.Width, config.Height, nil
}

func downloadImageBytes(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func NewTransparentImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	clearColor := color.RGBA{0, 0, 0, 0}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, clearColor)
		}
	}
	return img
}
