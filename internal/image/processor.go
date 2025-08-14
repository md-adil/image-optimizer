package image

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/h2non/bimg"

	"image-loader/internal/config"
	"image-loader/internal/image/etag"
)

var (
	// HTTP client with keepalive + timeouts
	httpClient = &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   50,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	bufPool = sync.Pool{
		New: func() any { return new(bytes.Buffer) },
	}
)

func safeProcessImage(origImg []byte, opts bimg.Options) (newImg []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in bimg processing: %v", r)
			err = fmt.Errorf("internal processing error")
		}
	}()
	newImg, err = bimg.NewImage(origImg).Process(opts)
	return
}

func putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}

func handleImage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	srcDomain := query.Get("d")

	width, _ := strconv.Atoi(query.Get("w"))

	height, _ := strconv.Atoi(query.Get("h"))

	quality, _ := strconv.Atoi(query.Get("q"))

	if srcDomain == "" {
		http.Error(w, "Missing source domain", http.StatusBadRequest)
		return
	}

	if !IsDomainAllowed(srcDomain) {
		http.Error(w, "Forbidden domain", http.StatusForbidden)
		return
	}

	// Build source URL
	imagePath := strings.TrimPrefix(r.URL.Path, "/x/")
	srcURL := fmt.Sprintf("https://%s/%s", srcDomain, imagePath)

	fmt.Println("Processing Image:", srcURL)

	// Fetch original image
	resp, err := httpClient.Get(srcURL)
	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "Failed to fetch source image", http.StatusBadGateway)
		fmt.Printf("Error fetching image from %s: %v\n", srcURL, err)
		return
	}

	defer resp.Body.Close()

	// Read into buffer from pool (we still need full bytes for bimg)
	buf := bufPool.Get().(*bytes.Buffer)
	// Cap the max size you will accept to avoid OOM attacks (e.g. 25MB)
	const maxAcceptSize = 25 << 20
	limitReq := io.LimitReader(resp.Body, maxAcceptSize+1)
	n, err := io.Copy(buf, limitReq)

	if err != nil {
		http.Error(w, "Error reading image", http.StatusInternalServerError)
		putBuffer(buf)
		log.Printf("read error %v\n", err)
		return
	}

	if n > maxAcceptSize {
		http.Error(w, "Source image too large", http.StatusRequestEntityTooLarge)
		putBuffer(buf)
		return
	}

	mediaType := GetMediaType(r.Header.Get("Accept"))

	// Process image
	options := bimg.Options{
		Force:         false, // do not force resize
		Enlarge:       false, // allow enlarging the image
		Compression:   1,
		StripMetadata: true,
		Type:          mediaType,
	}

	if quality > 0 {
		options.Quality = quality
	}

	if width > 0 {
		options.Width = width
	}

	if height > 0 {
		options.Height = height
	}
	// Current design: pass buf.Bytes() directly to bimg to avoid extra full-copy
	origImg := buf.Bytes()
	// ETag Handling, if matched with cache don't cache it again.
	tag := etag.Generate(origImg, srcURL, options)
	if etag.Matched(r, tag) {
		w.WriteHeader(http.StatusNotModified)
		log.Println("Skipping already matched with E-Tag")
		return
	}

	newImg, err := safeProcessImage(origImg, options)
	putBuffer(buf)

	if err != nil {
		http.Error(w, "Failed to process image", http.StatusInternalServerError)
		fmt.Printf("Error processing %s, mediaType: %v, image: %v\n ", srcURL, mediaType, err)
		return
	}

	mimeType := fmt.Sprintf("image/%s", bimg.ImageTypeName(mediaType))
	log.Println("Sending Type", mimeType)
	// Detect MIME type
	header := w.Header()
	header.Set("Content-Type", mimeType)

	// Tell CloudFront/CDNs & browsers to cache for 1 Month
	header.Set("Cache-Control", "public, max-age=2592000")

	// ETag for validation — you can generate from content hash
	if tag != "" {
		header.Set("ETag", tag)
	}

	// Vary by Accept header (for WebP/AVIF negotiation)
	header.Set("Vary", "Accept")

	// Optional: Security headers
	header.Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(newImg); err != nil {
		// client may have disconnected
		log.Printf("write error: %v\n", err)
	}
}

func Handler() func(w http.ResponseWriter, r *http.Request) {
	bimg.VipsCacheSetMax(100)
	bimg.VipsCacheSetMaxMem(config.EnvInt("VIPS_CACHE_MAX_MEM_MB", 512)) // MB

	PrintSupportedFormats()
	LoadWhitelistedDomains()
	WarmLibvips()
	return handleImage
}
