package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func Sign(body []byte, key string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(body)
	return mac.Sum(nil)
}

func SignHex(body []byte, key string) string {
	return hex.EncodeToString(Sign(body, key))
}
