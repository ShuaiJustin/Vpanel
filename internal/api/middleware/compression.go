package middleware

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		writer, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return writer
	},
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
	status int
	size   int
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	n, err := w.writer.Write(data)
	if n > 0 {
		w.size += n
	}
	return n, err
}

func (w *gzipResponseWriter) WriteString(s string) (int, error) {
	w.WriteHeaderNow()
	n, err := w.writer.Write([]byte(s))
	if n > 0 {
		w.size += n
	}
	return n, err
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if code > 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) WriteHeaderNow() {
	w.Header().Del("Content-Length")
	if w.status > 0 {
		w.ResponseWriter.WriteHeader(w.status)
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *gzipResponseWriter) Flush() {
	w.WriteHeaderNow()
	_ = w.writer.Flush()
	w.ResponseWriter.Flush()
}

func (w *gzipResponseWriter) Status() int {
	if w.status > 0 {
		return w.status
	}
	return w.ResponseWriter.Status()
}

func (w *gzipResponseWriter) Size() int {
	if w.size > 0 {
		return w.size
	}
	return w.ResponseWriter.Size()
}

func (w *gzipResponseWriter) Written() bool {
	return w.size > 0 || w.ResponseWriter.Written()
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func shouldSkipCompression(c *gin.Context) bool {
	if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
		return true
	}

	if strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade") {
		return true
	}

	requestPath := strings.TrimSpace(c.Request.URL.Path)
	if requestPath == "" {
		return false
	}

	if strings.HasPrefix(requestPath, "/api/") {
		return true
	}

	if strings.HasPrefix(requestPath, "/api/sse/") {
		return true
	}

	switch strings.ToLower(path.Ext(requestPath)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".ico", ".woff", ".woff2", ".mp4", ".webm", ".zip", ".gz", ".br", ".pdf":
		return true
	default:
		return false
	}
}

// Compression enables gzip compression for compressible responses.
func Compression() gin.HandlerFunc {
	return func(c *gin.Context) {
		if shouldSkipCompression(c) {
			c.Next()
			return
		}

		writer := gzipWriterPool.Get().(*gzip.Writer)
		writer.Reset(c.Writer)
		defer func() {
			_ = writer.Close()
			gzipWriterPool.Put(writer)
		}()

		headers := c.Writer.Header()
		headers.Set("Content-Encoding", "gzip")
		headers.Add("Vary", "Accept-Encoding")
		headers.Del("Content-Length")

		c.Writer = &gzipResponseWriter{
			ResponseWriter: c.Writer,
			writer:         writer,
			status:         c.Writer.Status(),
			size:           0,
		}

		c.Next()
	}
}
