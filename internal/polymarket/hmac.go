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
// Returns an error if the secret cannot be decoded from base64.
func BuildHmacSignature(secret, timestamp, method, path, body string) (string, error) {
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
			return "", fmt.Errorf("HMAC secret is not valid base64 (tried URL-safe and standard): %w", err)
		}
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// BuildL2Headers creates the full set of authentication headers for CLOB API requests.
// address is the maker's Polygon wallet address (for POLY_ADDRESS).
// creds contains the derived API key, secret, and passphrase (for POLY_API_KEY).
func BuildL2Headers(address string, creds ApiCreds, method, path, body string) (map[string]string, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig, err := BuildHmacSignature(creds.Secret, timestamp, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("build HMAC signature: %w", err)
	}

	return map[string]string{
		"POLY_ADDRESS":    address,
		"POLY_SIGNATURE":  sig,
		"POLY_TIMESTAMP":  timestamp,
		"POLY_API_KEY":    creds.Key,
		"POLY_PASSPHRASE": creds.Passphrase,
	}, nil
}
