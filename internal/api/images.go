package api

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"xfyun2openai/internal/config"
	"xfyun2openai/internal/openai"
	"xfyun2openai/internal/pool"
	"xfyun2openai/internal/xfyun"
)

var supportedSchedulers = map[string]struct{}{
	"DPM++ 2M Karras":  {},
	"DPM++ SDE Karras": {},
	"DDIM":             {},
	"Euler a":          {},
	"Euler":            {},
}

func (s *Service) handleImages(w http.ResponseWriter, r *http.Request) {
	if ct := strings.TrimSpace(r.Header.Get("Content-Type")); ct != "" && !strings.Contains(strings.ToLower(ct), "application/json") {
		writeOpenAIError(w, openai.NewHTTPError(http.StatusBadRequest, "Content-Type must be application/json", "invalid_request_error", "content_type", "invalid_content_type"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req openai.OpenAIImageRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		writeOpenAIError(w, openai.NewHTTPError(http.StatusBadRequest, "invalid JSON body", "invalid_request_error", "body", "invalid_json"))
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeOpenAIError(w, openai.NewHTTPError(http.StatusBadRequest, "request body must contain a single JSON object", "invalid_request_error", "body", "invalid_json"))
		return
	}

	modelName, modelCfg, n, uid, errResp := s.validateRequest(req, r.Context())
	if errResp != nil {
		writeOpenAIError(w, errResp)
		return
	}

	data := make([]openai.OpenAIImageData, 0, n)
	created := time.Now().Unix()
	for imageIndex := 0; imageIndex < n; imageIndex++ {
		seed, err := seedForRequest(req, imageIndex)
		if err != nil {
			writeOpenAIError(w, openai.NewHTTPError(http.StatusBadRequest, err.Error(), "invalid_request_error", "x_fyun.seed", "invalid_seed"))
			return
		}

		image, errResp := s.generateOneImage(r.Context(), modelName, modelCfg, req, uid, seed, imageIndex)
		if errResp != nil {
			writeOpenAIError(w, errResp)
			return
		}

		data = append(data, openai.OpenAIImageData{B64JSON: image})
	}

	writeJSON(w, http.StatusOK, openai.OpenAIImageResponse{
		Created: created,
		Data:    data,
	})
}

func (s *Service) validateRequest(req openai.OpenAIImageRequest, ctx context.Context) (string, config.ModelConfig, int, string, *openai.HTTPError) {
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "model is required", "invalid_request_error", "model", "missing_model")
	}

	modelCfg, ok := s.cfg.Models[modelName]
	if !ok {
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusNotFound, "model not found", "invalid_request_error", "model", "model_not_found")
	}

	if strings.TrimSpace(req.Prompt) == "" {
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "prompt is required", "invalid_request_error", "prompt", "missing_prompt")
	}
	if utf8.RuneCountInString(req.Prompt) > 1024 {
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "prompt must be <= 1024 characters", "invalid_request_error", "prompt", "prompt_too_long")
	}

	n := 1
	if req.N != nil {
		n = *req.N
	}
	if n < 1 || n > 10 {
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "n must be between 1 and 10", "invalid_request_error", "n", "invalid_n")
	}

	responseFormat := openai.ResponseFormatB64JSON
	if req.ResponseFormat != nil {
		responseFormat = *req.ResponseFormat
	}
	switch responseFormat {
	case openai.ResponseFormatB64JSON:
	case openai.ResponseFormatURL:
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "url response_format is not supported yet", "invalid_request_error", "response_format", "unsupported_response_format")
	default:
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "only b64_json response_format is supported", "invalid_request_error", "response_format", "unsupported_response_format")
	}

	if req.Size != nil {
		if _, _, err := xfyun.ParseSize(*req.Size); err != nil {
			return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, err.Error(), "invalid_request_error", "size", "invalid_size")
		}
	}

	if req.XFYun != nil {
		if utf8.RuneCountInString(req.XFYun.NegativePrompt) > 1024 {
			return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "x_fyun.negative_prompt must be <= 1024 characters", "invalid_request_error", "x_fyun.negative_prompt", "negative_prompt_too_long")
		}
		if req.XFYun.Seed != nil && (*req.XFYun.Seed < 0 || *req.XFYun.Seed > math.MaxUint32) {
			return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "x_fyun.seed must be between 0 and 4294967295", "invalid_request_error", "x_fyun.seed", "invalid_seed")
		}
		if req.XFYun.Steps != nil && (*req.XFYun.Steps <= 0 || *req.XFYun.Steps > 50) {
			return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "x_fyun.steps must be between 1 and 50", "invalid_request_error", "x_fyun.steps", "invalid_steps")
		}
		if req.XFYun.GuidanceScale != nil && (*req.XFYun.GuidanceScale <= 0 || *req.XFYun.GuidanceScale > 20) {
			return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "x_fyun.guidance_scale must be between 0 and 20", "invalid_request_error", "x_fyun.guidance_scale", "invalid_guidance_scale")
		}
		if req.XFYun.Scheduler != "" {
			if _, ok := supportedSchedulers[strings.TrimSpace(req.XFYun.Scheduler)]; !ok {
				return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusBadRequest, "x_fyun.scheduler is not supported", "invalid_request_error", "x_fyun.scheduler", "invalid_scheduler")
			}
		}
	}

	if _, ok := supportedSchedulers[strings.TrimSpace(modelCfg.Defaults.Scheduler)]; !ok {
		return "", config.ModelConfig{}, 0, "", openai.NewHTTPError(http.StatusInternalServerError, "configured default scheduler is invalid", "server_error", "scheduler", "invalid_default_scheduler")
	}

	uid := sanitizeUser(strings.TrimSpace(req.User))
	if uid == "" {
		uid = RequestIDFromContext(ctx)
	}

	return modelName, modelCfg, n, uid, nil
}

func (s *Service) generateOneImage(
	ctx context.Context,
	modelName string,
	modelCfg config.ModelConfig,
	req openai.OpenAIImageRequest,
	uid string,
	seed int64,
	imageIndex int,
) (string, *openai.HTTPError) {
	maxAttempts := 1 + s.cfg.Routing.MaxRetries
	if maxAttempts > len(modelCfg.Apps) {
		maxAttempts = len(modelCfg.Apps)
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	tried := make(map[string]struct{}, len(modelCfg.Apps))
	var lastUpstreamErr *xfyun.UpstreamError

	for attempt := 0; attempt < maxAttempts; attempt++ {
		lease, err := s.pool.Acquire(modelName, tried)
		if err != nil {
			if lastUpstreamErr != nil {
				return "", mapUpstreamError(lastUpstreamErr)
			}
			return "", openai.NewHTTPError(http.StatusServiceUnavailable, "all configured apps are unavailable or rate-limited", "server_error", "model", "no_available_app")
		}

		tried[lease.Name] = struct{}{}
		appCfg := lease.Config()
		upstreamReq, err := xfyun.BuildRequest(appCfg, modelCfg, req, seed, uid)
		if err != nil {
			lease.Finish(pool.FinishResult{})
			return "", openai.NewHTTPError(http.StatusBadRequest, err.Error(), "invalid_request_error", "size", "invalid_size")
		}

		startedAt := time.Now()
		result, err := s.client.Generate(ctx, modelCfg.Endpoint, appCfg, upstreamReq)
		if err == nil {
			lease.Finish(pool.FinishResult{Success: true})
			s.logAttempt(RequestIDFromContext(ctx), modelName, lease.Name, result.SID, time.Since(startedAt), "success", imageIndex, attempt+1)
			return result.ImageB64, nil
		}

		var upstreamErr *xfyun.UpstreamError
		if errors.As(err, &upstreamErr) {
			lastUpstreamErr = upstreamErr
			lease.Finish(pool.FinishResult{
				Retryable: upstreamErr.Retryable,
				Cooldown:  upstreamErr.Cooldown,
			})
			s.logAttempt(RequestIDFromContext(ctx), modelName, lease.Name, upstreamErr.SID, time.Since(startedAt), "error", imageIndex, attempt+1)
			if upstreamErr.Retryable && attempt < maxAttempts-1 {
				continue
			}
			return "", mapUpstreamError(upstreamErr)
		}

		lease.Finish(pool.FinishResult{})
		return "", openai.NewHTTPError(http.StatusBadGateway, "xfyun upstream request failed", "api_error", "upstream", "upstream_error")
	}

	if lastUpstreamErr != nil {
		return "", mapUpstreamError(lastUpstreamErr)
	}

	return "", openai.NewHTTPError(http.StatusServiceUnavailable, "all configured apps are unavailable or rate-limited", "server_error", "model", "no_available_app")
}

func (s *Service) logAttempt(requestID, modelName, appName, sid string, latency time.Duration, status string, imageIndex, attempt int) {
	s.logger.Info("image_generation_attempt",
		slog.String("request_id", requestID),
		slog.String("model", modelName),
		slog.String("selected_app_name", appName),
		slog.String("xfyun_sid", sid),
		slog.Int64("latency_ms", latency.Milliseconds()),
		slog.String("status", status),
		slog.Int("image_index", imageIndex),
		slog.Int("attempt", attempt),
	)
}

func mapUpstreamError(err *xfyun.UpstreamError) *openai.HTTPError {
	switch {
	case err.ContentPolicy:
		return openai.NewHTTPError(http.StatusBadRequest, err.Message, "content_policy_violation", "prompt", fmt.Sprintf("xfyun_%d", err.XFYunCode))
	case err.HTTPStatus == http.StatusBadRequest:
		return openai.NewHTTPError(http.StatusBadRequest, err.Message, "invalid_request_error", "upstream", fmt.Sprintf("xfyun_%d", err.XFYunCode))
	case err.HTTPStatus == http.StatusUnauthorized:
		return openai.NewHTTPError(http.StatusServiceUnavailable, "xfyun app authentication failed", "server_error", "upstream", "xfyun_auth_failed")
	case err.HTTPStatus == http.StatusForbidden:
		return openai.NewHTTPError(http.StatusServiceUnavailable, "xfyun app authorization was rejected", "server_error", "upstream", "xfyun_auth_failed")
	case err.HTTPStatus >= http.StatusInternalServerError || err.Retryable:
		return openai.NewHTTPError(http.StatusServiceUnavailable, "xfyun upstream service unavailable", "server_error", "upstream", "upstream_unavailable")
	default:
		return openai.NewHTTPError(http.StatusBadGateway, err.Message, "api_error", "upstream", "upstream_error")
	}
}

func seedForRequest(req openai.OpenAIImageRequest, imageIndex int) (int64, error) {
	if req.XFYun != nil && req.XFYun.Seed != nil {
		return *req.XFYun.Seed + int64(imageIndex), nil
	}

	max := big.NewInt(0).SetUint64(math.MaxUint32)
	value, err := crand.Int(crand.Reader, max)
	if err != nil {
		return 0, fmt.Errorf("generate random seed: %w", err)
	}

	return value.Int64(), nil
}

func sanitizeUser(user string) string {
	user = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, user)
	if len(user) > 32 {
		return user[:32]
	}
	return user
}
