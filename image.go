package goclip

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

var ErrNoImage = errors.New("clipboard contains no image")

// ImageReader is optional; existing text drivers need not implement it.
type ImageReader interface {
	ReadImage(context.Context) (image.Image, error)
}

// ReadImage reads the active system clipboard without falling back to stale
// file-backed clipboard data. Unsupported text-only backends remain usable.
func ReadImage(ctx context.Context) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver := ActiveDriver()
	reader, ok := driver.(ImageReader)
	if !ok {
		return nil, ErrUnsupportedFormat
	}
	return reader.ReadImage(ctx)
}

const maxImageBytes = 64 << 20
const maxImagePixels = 64 << 20

func decodeClipboardImage(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, ErrNoImage
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("clipboard image exceeds size limit")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading clipboard image: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, fmt.Errorf("clipboard image dimensions exceed limit")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding clipboard image: %w", err)
	}
	return img, nil
}

// ReadImageText reads actual text offered by the image's driver, never stale
// fallback data or the image MIME representation returned by a CLI default.
func ReadImageText(ctx context.Context) (string, error) {
	return readImageText(ctx, ActiveDriver())
}

func readImageText(ctx context.Context, driver Driver) (string, error) {
	if reader, ok := driver.(interface {
		ReadImageText(context.Context) (string, error)
	}); ok {
		return reader.ReadImageText(ctx)
	}
	text, err := driver.ReadText()
	if errors.Is(err, ErrEmpty) {
		return "", nil
	}
	return text, err
}

// Contents holds representations captured for a single paste operation.
type Contents struct {
	Image image.Image
	Text  string
}

// ContentsReader optionally supplies an atomic platform clipboard snapshot.
type ContentsReader interface {
	ReadContents(context.Context) (Contents, error)
}

func ReadContents(ctx context.Context) (Contents, error) {
	if err := ctx.Err(); err != nil {
		return Contents{}, err
	}
	driver := ActiveDriver()
	if reader, ok := driver.(ContentsReader); ok {
		return reader.ReadContents(ctx)
	}
	reader, ok := driver.(ImageReader)
	if !ok {
		return Contents{}, ErrUnsupportedFormat
	}
	img, err := reader.ReadImage(ctx)
	if err != nil {
		return Contents{}, err
	}
	text, err := readImageText(ctx, driver)
	return Contents{Image: img, Text: text}, err
}
