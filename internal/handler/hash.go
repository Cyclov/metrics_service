package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

// HashMiddleware verifies requests before decoding and signs the final response
// bytes. Register it before GzipMiddleware to sign compressed bodies on the wire.
func HashMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &hashResponseWriter{header: w.Header().Clone()}
			if verifyRequestHash(response, r, key) {
				next.ServeHTTP(response, r)
			}
			response.writeSignedResponse(w, key)
		})
	}
}

// verifyRequestHash restores a valid body for the next handler or writes a 400 response.
func verifyRequestHash(w http.ResponseWriter, r *http.Request, key string) bool {
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return false
	}
	received, err := hex.DecodeString(r.Header.Get("HashSHA256"))
	if err != nil || !hmac.Equal(received, hashBody(body, key)) {
		http.Error(w, "Invalid request hash", http.StatusBadRequest)
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return true
}

func hashBody(body []byte, key string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return mac.Sum(nil)
}

// Buffer the response so its signature is available before headers are sent.
type hashResponseWriter struct {
	header     http.Header
	sentHeader http.Header
	status     int
	body       bytes.Buffer
}

func (w *hashResponseWriter) Header() http.Header { return w.header }

func (w *hashResponseWriter) writeSignedResponse(dst http.ResponseWriter, key string) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	for name, values := range w.sentHeader {
		dst.Header()[name] = values
	}
	dst.Header().Set("HashSHA256", hex.EncodeToString(hashBody(w.body.Bytes(), key)))
	dst.WriteHeader(w.status)
	_, _ = dst.Write(w.body.Bytes())
}

func (w *hashResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.sentHeader = w.header.Clone()
}

func (w *hashResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		if w.header.Get("Content-Type") == "" && len(body) > 0 {
			w.header.Set("Content-Type", http.DetectContentType(body))
		}
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(body)
}
