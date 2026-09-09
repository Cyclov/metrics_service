package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cyclov/metrics_service/internal/agent"
	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/Cyclov/metrics_service/internal/repository"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSignature(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestHashMiddleware(t *testing.T) {
	for _, tt := range []struct {
		name       string
		key        string
		signKey    string
		header     string
		compressed bool
		wantStatus int
	}{
		{name: "disabled", header: "invalid", wantStatus: http.StatusOK},
		{name: "valid", key: "test-key", signKey: "test-key", wantStatus: http.StatusOK},
		{name: "gzip", key: "test-key", signKey: "test-key", compressed: true, wantStatus: http.StatusOK},
		{name: "missing", key: "test-key", wantStatus: http.StatusOK},
		{name: "missing gzip", key: "test-key", compressed: true, wantStatus: http.StatusOK},
		{name: "malformed", key: "test-key", header: "not-hex", wantStatus: http.StatusBadRequest},
		{name: "wrong key", key: "test-key", signKey: "wrong-key", wantStatus: http.StatusBadRequest},
		{name: "short hash", key: "test-key", header: "abcd", wantStatus: http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			storage := repository.NewMemStorage()
			h := HashMiddleware(tt.key)(GzipMiddleware(http.HandlerFunc(New(storage, nil).Update)))
			body := []byte(`{"id":"Alloc","type":"gauge","value":12.5}`)
			if tt.compressed {
				var compressed bytes.Buffer
				zw := gzip.NewWriter(&compressed)
				_, err := zw.Write(body)
				require.NoError(t, err)
				require.NoError(t, zw.Close())
				body = compressed.Bytes()
			}
			req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
			if tt.header != "" {
				req.Header.Set("HashSHA256", tt.header)
			}
			if tt.signKey != "" {
				req.Header.Set("HashSHA256", testSignature(body, tt.signKey))
			}
			if tt.compressed {
				req.Header.Set("Content-Encoding", "gzip")
				req.Header.Set("Accept-Encoding", "gzip")
			}
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			assert.Equal(t, tt.wantStatus, resp.Code)
			value, found, err := storage.Gauge(context.Background(), "Alloc")
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus == http.StatusOK, found)
			if found {
				assert.Equal(t, 12.5, value)
				assert.Contains(t, resp.Header().Get("Content-Type"), "application/json")
			}
			if tt.key == "" {
				assert.Empty(t, resp.Header().Get("HashSHA256"))
			} else {
				assert.Equal(t, testSignature(resp.Body.Bytes(), tt.key), resp.Header().Get("HashSHA256"))
			}
			if tt.compressed {
				assert.Equal(t, "gzip", resp.Header().Get("Content-Encoding"))
				zr, err := gzip.NewReader(resp.Body)
				require.NoError(t, err)
				defer zr.Close()
				decoded, err := io.ReadAll(zr)
				require.NoError(t, err)
				assert.JSONEq(t, `{"id":"Alloc","type":"gauge","value":12.5}`, string(decoded))
			}
		})
	}
}

func TestHashMiddlewareWithAgent(t *testing.T) {
	storage := repository.NewMemStorage()
	h := HashMiddleware("test-key")(GzipMiddleware(http.HandlerFunc(New(storage, nil).Updates)))
	server := httptest.NewServer(h)
	defer server.Close()
	sender := agent.NewSender(server.URL, resty.New(), "test-key")
	delta := int64(3)
	require.NoError(t, sender.Send([]models.Metrics{{ID: "PollCount", MType: models.Counter, Delta: &delta}}))
	value, found, err := storage.Counter(context.Background(), "PollCount")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, delta, value)
}

func TestHashMiddlewareUnsignedValueRequest(t *testing.T) {
	for _, encoding := range []string{"identity", "gzip"} {
		t.Run(encoding, func(t *testing.T) {
			storage := repository.NewMemStorage()
			require.NoError(t, storage.SetGauge(context.Background(), "Alloc", 12.5))
			h := HashMiddleware("test-key")(GzipMiddleware(http.HandlerFunc(New(storage, nil).Value)))
			req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewBufferString(`{"id":"Alloc","type":"gauge"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept-Encoding", encoding)
			req.Header.Set("Hash", "none")
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			require.Equal(t, http.StatusOK, resp.Code)
			assert.Contains(t, resp.Header().Get("Content-Type"), "application/json")
			assert.Equal(t, testSignature(resp.Body.Bytes(), "test-key"), resp.Header().Get("HashSHA256"))
			var body io.Reader = resp.Body
			if encoding == "gzip" {
				assert.Equal(t, "gzip", resp.Header().Get("Content-Encoding"))
				zr, err := gzip.NewReader(body)
				require.NoError(t, err)
				defer zr.Close()
				body = zr
			}
			decoded, err := io.ReadAll(body)
			require.NoError(t, err)
			assert.JSONEq(t, `{"id":"Alloc","type":"gauge","value":12.5}`, string(decoded))
		})
	}
}

func TestHashMiddlewareEmptyResponse(t *testing.T) {
	h := HashMiddleware("test-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("HashSHA256", testSignature(nil, "test-key"))
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Empty(t, resp.Body.Bytes())
	assert.Equal(t, testSignature(nil, "test-key"), resp.Header().Get("HashSHA256"))
}
