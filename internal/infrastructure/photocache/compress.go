package photocache

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/nfnt/resize"
)

// CompressPhoto resizes and re-compresses a JPEG image.
// It limits the longest side to maxDimension pixels while maintaining aspect ratio,
// then re-encodes at the given JPEG quality (1-100).
// If the image is already smaller than maxDimension, it is re-encoded at the target
// quality without upscaling.
// Returns the compressed bytes, or an error if decoding/encoding fails.
func CompressPhoto(data []byte, maxDimension uint, quality int) ([]byte, error) {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())

	// Resize only if the image exceeds maxDimension on any side.
	// Passing 0 for one dimension tells nfnt/resize to maintain aspect ratio.
	if width > maxDimension || height > maxDimension {
		if width >= height {
			img = resize.Resize(maxDimension, 0, img, resize.Lanczos3)
		} else {
			img = resize.Resize(0, maxDimension, img, resize.Lanczos3)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode image: %w", err)
	}

	return buf.Bytes(), nil
}
