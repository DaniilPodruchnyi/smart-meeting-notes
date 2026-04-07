package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

func GzipCompress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gzWriter := &gzipResponseWriter{
			ResponseWriter: w,
			Writer:         nil,
		}

		next.ServeHTTP(gzWriter, r)

		if gzWriter.Writer != nil {
			gzWriter.Writer.Close()
		}
	})
}

func GzipDecompress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gzReader, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip data", http.StatusBadRequest)
				return
			}
			defer gzReader.Close()

			r.Body = io.NopCloser(gzReader)
		}

		next.ServeHTTP(w, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer      *gzip.Writer
	contentType string
	wroteHeader bool
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	w.contentType = w.Header().Get("Content-Type")

	if shouldCompress(w.contentType) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.Writer = gzip.NewWriter(w.ResponseWriter)
	}

	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		if w.contentType == "" {
			w.contentType = w.Header().Get("Content-Type")
		}
		if shouldCompress(w.contentType) {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")
		}
		w.WriteHeader(http.StatusOK)
		if shouldCompress(w.contentType) && w.Writer == nil {
			w.Writer = gzip.NewWriter(w.ResponseWriter)
		}
	}

	if w.Writer != nil {
		return w.Writer.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func shouldCompress(contentType string) bool {
	return strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html")
}
