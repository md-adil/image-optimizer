package image

import (
	"log"
	"net/http"
	"time"
)

func LimitMiddleware(next http.Handler, maxReq int, timeout time.Duration) http.Handler {
	sem := make(chan struct{}, maxReq)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health-z" {
			next.ServeHTTP(w, r)
			return
		}
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
			return
		default:
			log.Println("Slot is not available yet, waiting...")
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			case <-time.After(timeout):
				println("Timeout, Server busy")
				http.Error(w, "Server busy", http.StatusServiceUnavailable)
			}
		}
	})
}
