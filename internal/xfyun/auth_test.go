package xfyun

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"testing"
	"time"
)

func TestBuildSignedURL(t *testing.T) {
	now := time.Date(2024, time.May, 14, 8, 46, 48, 0, time.UTC)
	signedURL, err := BuildSignedURL(
		"https://maas-api.cn-huabei-1.xf-yun.com/v2.1/tti",
		"test-api-key",
		"test-api-secret",
		now,
	)
	if err != nil {
		t.Fatalf("BuildSignedURL returned error: %v", err)
	}

	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	query := parsed.Query()
	if got, want := query.Get("host"), "maas-api.cn-huabei-1.xf-yun.com"; got != want {
		t.Fatalf("host mismatch: got %q want %q", got, want)
	}
	if got, want := query.Get("date"), "Tue, 14 May 2024 08:46:48 GMT"; got != want {
		t.Fatalf("date mismatch: got %q want %q", got, want)
	}

	authorizationRaw, err := base64.StdEncoding.DecodeString(query.Get("authorization"))
	if err != nil {
		t.Fatalf("decode authorization: %v", err)
	}

	signatureOrigin := "host: maas-api.cn-huabei-1.xf-yun.com\n" +
		"date: Tue, 14 May 2024 08:46:48 GMT\n" +
		"POST /v2.1/tti HTTP/1.1"
	mac := hmac.New(sha256.New, []byte("test-api-secret"))
	_, _ = mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	wantAuthorization := `api_key="test-api-key", algorithm="hmac-sha256", headers="host date request-line", signature="` + signature + `"`
	if got := string(authorizationRaw); got != wantAuthorization {
		t.Fatalf("authorization mismatch:\n got: %q\nwant: %q", got, wantAuthorization)
	}
}

func TestBuildSignedURLInvalidEndpoint(t *testing.T) {
	if _, err := BuildSignedURL("://bad-url", "key", "secret", time.Now()); err == nil {
		t.Fatal("expected invalid endpoint error")
	}
}
