package xfyun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"xfyun2openai/internal/config"
)

type Client struct {
	httpClient *http.Client
}

type GenerateResult struct {
	ImageB64 string
	SID      string
}

type UpstreamError struct {
	HTTPStatus    int
	Message       string
	XFYunCode     int
	SID           string
	Retryable     bool
	Cooldown      bool
	ContentPolicy bool
}

func (e *UpstreamError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "xfyun upstream error"
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Generate(
	ctx context.Context,
	endpoint string,
	app config.AppConfig,
	req GenerateRequest,
) (*GenerateResult, error) {
	signedURL, err := BuildSignedURL(endpoint, app.APIKey, app.APISecret, time.Now())
	if err != nil {
		return nil, &UpstreamError{
			HTTPStatus: 503,
			Message:    fmt.Sprintf("build signed url: %v", err),
			Retryable:  false,
			Cooldown:   true,
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal xfyun request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, signedURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, classifyTransportError(err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, &UpstreamError{
			HTTPStatus: 502,
			Message:    "read upstream response failed",
			Retryable:  true,
		}
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &UpstreamError{
			HTTPStatus: resp.StatusCode,
			Message:    strings.TrimSpace(string(responseBody)),
			Retryable:  true,
			Cooldown:   true,
		}
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, &UpstreamError{
			HTTPStatus: resp.StatusCode,
			Message:    "xfyun upstream unavailable",
			Retryable:  true,
		}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &UpstreamError{
			HTTPStatus: resp.StatusCode,
			Message:    strings.TrimSpace(string(responseBody)),
		}
	}

	var decoded GenerateResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, &UpstreamError{
			HTTPStatus: 502,
			Message:    "decode upstream response failed",
			Retryable:  true,
		}
	}

	if decoded.Header.Code != 0 {
		return nil, classifyBusinessError(decoded.Header)
	}

	for _, item := range decoded.Payload.Choices.Text {
		if strings.TrimSpace(item.Content) != "" {
			return &GenerateResult{
				ImageB64: item.Content,
				SID:      decoded.Header.SID,
			}, nil
		}
	}

	return nil, &UpstreamError{
		HTTPStatus: 502,
		Message:    "xfyun returned empty image payload",
		Retryable:  true,
		SID:        decoded.Header.SID,
	}
}

func classifyTransportError(err error) error {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return &UpstreamError{
			HTTPStatus: 503,
			Message:    err.Error(),
			Retryable:  true,
		}
	}

	return &UpstreamError{
		HTTPStatus: 502,
		Message:    err.Error(),
		Retryable:  true,
	}
}

func classifyBusinessError(header ResponseHeader) error {
	err := &UpstreamError{
		HTTPStatus: 400,
		Message:    header.Message,
		XFYunCode:  header.Code,
		SID:        header.SID,
	}

	switch header.Code {
	case 10003, 10004, 10005:
		return err
	case 10008:
		err.HTTPStatus = 503
		err.Retryable = true
		return err
	case 10021, 10022:
		err.ContentPolicy = true
		return err
	default:
		if looksLikeAuthFailure(header.Message, header.Code) {
			err.HTTPStatus = 503
			err.Retryable = true
			err.Cooldown = true
			return err
		}
		err.HTTPStatus = 502
		return err
	}
}

func looksLikeAuthFailure(message string, code int) bool {
	if code == 401 || code == 403 || code == 11200 || code == 10007 {
		return true
	}

	lower := strings.ToLower(message)
	return strings.Contains(lower, "auth") ||
		strings.Contains(lower, "signature") ||
		strings.Contains(lower, "unauthorized")
}
