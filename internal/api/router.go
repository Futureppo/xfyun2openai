package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"xfyun2openai/internal/config"
	"xfyun2openai/internal/openai"
	"xfyun2openai/internal/pool"
	"xfyun2openai/internal/xfyun"
)

type Service struct {
	cfg    *config.Config
	pool   *pool.Pool
	client *xfyun.Client
	logger *slog.Logger
}

func NewService(cfg *config.Config, p *pool.Pool, client *xfyun.Client, logger *slog.Logger) *Service {
	return &Service{
		cfg:    cfg,
		pool:   p,
		client: client,
		logger: logger,
	}
}

func NewRouter(service *Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", withMethod(http.MethodGet, http.HandlerFunc(service.handleHealthz)))
	mux.Handle("/v1/models", withAuth(service.cfg.Server.APIKeys, withMethod(http.MethodGet, http.HandlerFunc(service.handleModels))))
	mux.Handle("/v1/images/generations", withAuth(service.cfg.Server.APIKeys, withMethod(http.MethodPost, http.HandlerFunc(service.handleImages))))

	return withRequestID(mux)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeOpenAIError(w http.ResponseWriter, err *openai.HTTPError) {
	openai.WriteError(w, err)
}
