package handler

import (
	"bytes"
	"crypto/hmac"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/Cyclov/metrics_service/internal/sign"
)

const maxRequestBodySize int64 = 128 << 10 // 128 KiB

func HashMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &hashResponseWriter{header: w.Header().Clone()}
			signature := r.Header.Get("HashSHA256")
			if signature == "" {
				next.ServeHTTP(response, r)
				response.writeSignedResponse(w, key)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				http.Error(response, "Cannot read request body", http.StatusBadRequest)
			} else {
				r.Body = io.NopCloser(bytes.NewReader(body))
				if verifyRequestHash(body, signature, key) {
					next.ServeHTTP(response, r)
				} else {
					http.Error(response, "Invalid request hash", http.StatusBadRequest)
				}
			}
			response.writeSignedResponse(w, key)
		})
	}
}

func verifyRequestHash(body []byte, signature, key string) bool {
	received, err := hex.DecodeString(signature)
	return err == nil && hmac.Equal(received, sign.Sign(body, key))
}

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
	dst.Header().Set("HashSHA256", sign.SignHex(w.body.Bytes(), key))
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
