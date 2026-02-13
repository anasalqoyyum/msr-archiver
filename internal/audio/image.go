package audio

import (
	"fmt"
	"image"
	"image/png"
	"os"

	_ "image/jpeg"
	_ "image/png"
)

// ConvertToPNG converts an image file to PNG.
func ConvertToPNG(srcPath, dstPath string) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer in.Close()

	img, _, err := image.Decode(in)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create png: %w", err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}

	return nil
}
