package sigstore

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// splitJWT splits a JWT token into its three parts (header, payload, signature).
func splitJWT(token string) []string {
	return strings.Split(strings.TrimSpace(token), ".")
}

// base64Decode decodes a base64url-encoded string (with or without padding).
func base64Decode(s string) ([]byte, error) {
	// Add padding if needed.
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	return base64.URLEncoding.DecodeString(s)
}

// extractExpFromJSON extracts the "exp" numeric field from a JSON payload.
// This is a lightweight parser that avoids importing encoding/json for minimal dependencies.
func extractExpFromJSON(payload []byte) (int64, error) {
	s := string(payload)

	// Find "exp" key in the JSON.
	expIdx := strings.Index(s, `"exp"`)
	if expIdx == -1 {
		return 0, fmt.Errorf("JWT payload does not contain exp claim")
	}

	// Find the colon after "exp".
	rest := s[expIdx+5:]
	colonIdx := strings.Index(rest, ":")
	if colonIdx == -1 {
		return 0, fmt.Errorf("malformed JWT payload: no value for exp claim")
	}

	// Extract the numeric value after the colon.
	valueStr := rest[colonIdx+1:]
	valueStr = strings.TrimSpace(valueStr)

	// Read digits (and possible negative sign).
	var numStr strings.Builder
	for _, ch := range valueStr {
		if ch >= '0' && ch <= '9' || ch == '-' {
			numStr.WriteRune(ch)
		} else {
			break
		}
	}

	if numStr.Len() == 0 {
		return 0, fmt.Errorf("malformed JWT payload: exp value is not numeric")
	}

	exp, err := strconv.ParseInt(numStr.String(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse exp value: %w", err)
	}

	return exp, nil
}
