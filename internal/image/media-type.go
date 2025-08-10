package image

import (
	"strings"

	"github.com/h2non/bimg"
)

func GetMediaType(accept string) bimg.ImageType {
	// if strings.Contains(accept, "image/avif") {
	// 	return bimg.AVIF
	// }
	if strings.Contains(accept, "image/webp") {
		return bimg.WEBP
	}
	return bimg.JPEG
}
