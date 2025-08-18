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

	imagePath := strings.TrimPrefix(r.URL.Path, "/x/")
	srcURL := fmt.Sprintf("https://%s/%s", srcDomain, imagePath)

	fmt.Println("Processing Image:", srcURL)

	resp, err := httpClient.Get(srcURL)

	if err != nil {
		http.Error(w, "Failed to fetch source image", http.StatusBadGateway)
		fmt.Printf("Error fetching image from %s: %v\n", srcURL, err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, "Failed to fetch source image", http.StatusBadGateway)
		return
	}

	buf := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()
	const maxAcceptSize = 25 << 20
	limitReq := io.LimitReader(resp.Body, maxAcceptSize+1)
	n, err := io.Copy(buf, limitReq)

	if err != nil {
		http.Error(w, "Error reading image", http.StatusInternalServerError)
		log.Printf("read error %v\n", err)
		return
	}

	if n > maxAcceptSize {
		http.Error(w, "Source image too large", http.StatusRequestEntityTooLarge)
		return
	}

	mediaType := GetMediaType(r.Header.Get("Accept"))

	options := bimg.Options{
		Force:         false,
		Enlarge:       false,
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
	origImg := buf.Bytes()

	tag := etag.Generate(origImg, srcURL, options)
	if etag.Matched(r, tag) {
		w.WriteHeader(http.StatusNotModified)
		log.Println("Skipping already matched with ETag")
		return
	}

	newImg, err := safeProcessImage(origImg, options)

	if err != nil {
		http.Error(w, "Failed to process image", http.StatusInternalServerError)
		fmt.Printf("Error processing %s, mediaType: %v, image: %v\n ", srcURL, mediaType, err)
		return
	}

	mimeType := fmt.Sprintf("image/%s", bimg.ImageTypeName(mediaType))
	log.Println("Sending Type", mimeType)
	header := w.Header()
	header.Set("Content-Type", mimeType)

	header.Set("Cache-Control", "public, max-age=2592000")

	if tag != "" {
		header.Set("ETag", tag)
	}

	header.Set("Vary", "Accept")

	header.Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(newImg); err != nil {
		log.Printf("write error: %v\n", err)
	}
}

func Handler() func(w http.ResponseWriter, r *http.Request) {
	bimg.VipsCacheSetMax(100)
	bimg.VipsCacheSetMaxMem(config.EnvInt("VIPS_CACHE_MAX_MEM_MB", 512)) // MB
	PrintSupportedFormats()
	LoadWhitelistedDomains()
	WarmLibVips()
	return handleImage
}
