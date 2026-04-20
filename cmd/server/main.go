package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"xfyun2openai/internal/api"
	"xfyun2openai/internal/config"
	applog "xfyun2openai/internal/log"
	"xfyun2openai/internal/pool"
	"xfyun2openai/internal/xfyun"
)

func main() {
	logger := applog.New()

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("load .env failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("load config failed", slog.String("path", cfgPath), slog.String("error", err.Error()))
		os.Exit(1)
	}

	service := api.NewService(
		cfg,
		pool.New(cfg),
		xfyun.NewClient(time.Duration(cfg.XFYun.DefaultTimeoutSeconds)*time.Second),
		logger,
	)

	server := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           api.NewRouter(service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      time.Duration(cfg.XFYun.DefaultTimeoutSeconds+10) * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("starting server",
		slog.String("listen", cfg.Server.Listen),
		slog.String("config_path", cfgPath),
		slog.Int("models", len(cfg.Models)),
		slog.Int("apps", len(cfg.Apps)),
	)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
