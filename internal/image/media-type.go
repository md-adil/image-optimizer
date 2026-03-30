package image

import (
	"strconv"
	"strings"

	"github.com/h2non/bimg"
)

// acceptItem holds a parsed Accept header media type with its q-value.
type acceptItem struct {
	mimeType string
	q        float64
}

// parseAccept parses the Accept header and returns items sorted by q-value descending.
func parseAccept(accept string) []acceptItem {
	parts := strings.Split(accept, ",")
	items := make([]acceptItem, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.Split(part, ";")
		mime := strings.TrimSpace(segments[0])
		q := 1.0
		for _, seg := range segments[1:] {
			seg = strings.TrimSpace(seg)
			if strings.HasPrefix(seg, "q=") {
				if v, err := strconv.ParseFloat(seg[2:], 64); err == nil {
					q = v
				}
			}
		}
		items = append(items, acceptItem{mimeType: mime, q: q})
	}
	// Stable sort by q descending (insertion sort — small N)
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].q > items[j-1].q; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	return items
}

// GetMediaType returns the best supported image type based on the Accept header.
// AVIF is preferred over WebP when both are accepted with equal or higher q-value.
func GetMediaType(accept string) bimg.ImageType {
	for _, item := range parseAccept(accept) {
		switch item.mimeType {
		case "image/avif":
			return bimg.AVIF
		case "image/webp":
			return bimg.WEBP
		}
	}
	return bimg.JPEG
}
