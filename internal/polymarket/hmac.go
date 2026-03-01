package polymarket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

// ApiCreds holds ephemeral L2 credentials derived from an L1 auth signature.
type ApiCreds struct {
	Key        string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// BuildHmacSignature creates the HMAC-SHA256 signature for CLOB API authentication.
func BuildHmacSignature(secret, timestamp, method, path, body string) string {
	message := timestamp + method + path + body
	key, err := base64.URLEncoding.DecodeString(secret)
	if err != nil {
		key = []byte(secret)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// BuildL2Headers creates the full set of authentication headers for CLOB API requests.
func BuildL2Headers(creds ApiCreds, method, path, body string) map[string]string {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := BuildHmacSignature(creds.Secret, timestamp, method, path, body)

	return map[string]string{
		"POLY_ADDRESS":          creds.Key,
		"POLY_SIGNATURE":        sig,
		"POLY_TIMESTAMP":        timestamp,
		"POLY_NONCE":            "0",
		"POLY_API_KEY":          creds.Key,
		"POLY_PASSPHRASE":       creds.Passphrase,
	}
}
