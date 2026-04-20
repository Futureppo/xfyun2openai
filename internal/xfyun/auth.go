package xfyun

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func BuildSignedURL(endpoint, apiKey, apiSecret string, now time.Time) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("endpoint must include scheme and host")
	}

	date := formatDate(now)
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}

	signatureOrigin := strings.Join([]string{
		"host: " + parsed.Host,
		"date: " + date,
		"POST " + path + " HTTP/1.1",
	}, "\n")

	mac := hmac.New(sha256.New, []byte(apiSecret))
	_, _ = mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	authOrigin := fmt.Sprintf(
		`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		apiKey,
		signature,
	)

	query := parsed.Query()
	query.Set("authorization", base64.StdEncoding.EncodeToString([]byte(authOrigin)))
	query.Set("date", date)
	query.Set("host", parsed.Host)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func formatDate(now time.Time) string {
	return now.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
}
