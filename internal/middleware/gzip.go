package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

func Decompress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerContent := r.Header.Get("Content-Encoding")
		if !strings.Contains(headerContent, "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "invalid gzip body", http.StatusBadRequest)
			return
		}
		defer reader.Close()
		r.Body = reader
		r.Header.Del("Content-Encoding")
		next.ServeHTTP(w, r)
	})
}
