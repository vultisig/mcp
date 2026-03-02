package polymarket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
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

	// Polymarket secrets may arrive in URL-safe or standard base64, with or without padding.
	// The TS client normalizes base64url → standard before decoding.
	// Try both: URL-safe first, then standard base64.
	stripped := strings.TrimRight(secret, "=")
	key, err := base64.RawURLEncoding.DecodeString(stripped)
	if err != nil {
		// Secret may use standard base64 alphabet (+ and / instead of - and _)
		key, err = base64.RawStdEncoding.DecodeString(stripped)
		if err != nil {
			key = []byte(secret)
		}
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// BuildL2Headers creates the full set of authentication headers for CLOB API requests.
// address is the maker's Polygon wallet address (for POLY_ADDRESS).
// creds contains the derived API key, secret, and passphrase (for POLY_API_KEY).
func BuildL2Headers(address string, creds ApiCreds, method, path, body string) map[string]string {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := BuildHmacSignature(creds.Secret, timestamp, method, path, body)

	return map[string]string{
		"POLY_ADDRESS":    address,
		"POLY_SIGNATURE":  sig,
		"POLY_TIMESTAMP":  timestamp,
		"POLY_API_KEY":    creds.Key,
		"POLY_PASSPHRASE": creds.Passphrase,
	}
}
