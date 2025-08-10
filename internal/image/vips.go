package image

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/h2non/bimg"
)

func PrintSupportedFormats() {
	fmt.Println("Supported AVIF:", bimg.IsImageTypeSupportedByVips(bimg.AVIF))
	fmt.Println("Supported WebP:", bimg.IsImageTypeSupportedByVips(bimg.WEBP))
	fmt.Println("Supported JPEG:", bimg.IsImageTypeSupportedByVips(bimg.JPEG))
}

// ---------- Warm libvips ----------
func WarmLibvips() {
	// A tiny in-memory image (1x1 white PNG)
	// PNG is safe to decode in all libvips builds
	const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII="
	img, err := base64.StdEncoding.DecodeString(onePixelPNG)
	if err != nil {
		log.Println("Failed to decode warm-up PNG:", err)
		return
	}

	formats := []bimg.ImageType{
		bimg.JPEG,
		bimg.PNG,
		bimg.WEBP,
		bimg.AVIF, // Will fail silently if not supported by libvips
		bimg.TIFF,
	}

	for _, format := range formats {
		opts := bimg.Options{
			Type:    format,
			Quality: 90,
			Width:   1,
			Height:  1,
		}
		_, err := bimg.NewImage(img).Process(opts)
		if err != nil {
			log.Printf("warming format %v failed: %v", format, err)
		} else {
			log.Printf("warmed format: %v", format)
		}
	}
}
