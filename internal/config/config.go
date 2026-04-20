package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultListen         = ":8787"
	DefaultEndpoint       = "https://maas-api.cn-huabei-1.xf-yun.com/v2.1/tti"
	DefaultTimeoutSeconds = 120
	DefaultMaxRetries     = 2
	DefaultCooldown       = 60
	DefaultSize           = "1024x1024"
	DefaultSteps          = 20
	DefaultGuidanceScale  = 5.0
	DefaultScheduler      = "Euler"
)

var allowedSizes = map[string]struct{}{
	"768x768":   {},
	"1024x1024": {},
	"576x1024":  {},
	"768x1024":  {},
	"1024x576":  {},
	"1024x768":  {},
}

var allowedSchedulers = map[string]struct{}{
	"DPM++ 2M Karras":  {},
	"DPM++ SDE Karras": {},
	"DDIM":             {},
	"Euler a":          {},
	"Euler":            {},
}

type Config struct {
	Server  ServerConfig           `yaml:"server"`
	XFYun   XFYunConfig            `yaml:"xfyun"`
	Routing RoutingConfig          `yaml:"routing"`
	Apps    map[string]AppConfig   `yaml:"apps"`
	Models  map[string]ModelConfig `yaml:"models"`
}

type ServerConfig struct {
	Listen  string   `yaml:"listen"`
	APIKeys []string `yaml:"api_keys"`
}

type XFYunConfig struct {
	DefaultEndpoint       string `yaml:"default_endpoint"`
	DefaultTimeoutSeconds int    `yaml:"default_timeout_seconds"`
}

type RoutingConfig struct {
	MaxRetries      int `yaml:"max_retries"`
	CooldownSeconds int `yaml:"cooldown_seconds"`
}

type AppConfig struct {
	AppID          string `yaml:"app_id"`
	APIKey         string `yaml:"api_key"`
	APISecret      string `yaml:"api_secret"`
	MaxConcurrency int    `yaml:"max_concurrency"`
}

type ModelConfig struct {
	DisplayName string        `yaml:"display_name"`
	ModelID     string        `yaml:"model_id"`
	ResourceID  string        `yaml:"resource_id"`
	PatchID     string        `yaml:"patch_id"`
	Endpoint    string        `yaml:"endpoint"`
	Apps        []string      `yaml:"apps"`
	Defaults    ModelDefaults `yaml:"defaults"`
}

type ModelDefaults struct {
	Size          string  `yaml:"size"`
	Steps         int     `yaml:"steps"`
	GuidanceScale float64 `yaml:"guidance_scale"`
	Scheduler     string  `yaml:"scheduler"`
}

func DefaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("CONFIG_PATH")); path != "" {
		return path
	}

	return "./config.yaml"
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	expanded := os.ExpandEnv(string(raw))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("decode config yaml: %w", err)
	}

	if err := cfg.normalizeAndValidate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) normalizeAndValidate() error {
	if strings.TrimSpace(c.Server.Listen) == "" {
		c.Server.Listen = DefaultListen
	}

	c.Server.APIKeys = compactStrings(c.Server.APIKeys)

	if strings.TrimSpace(c.XFYun.DefaultEndpoint) == "" {
		c.XFYun.DefaultEndpoint = DefaultEndpoint
	}
	if c.XFYun.DefaultTimeoutSeconds <= 0 {
		c.XFYun.DefaultTimeoutSeconds = DefaultTimeoutSeconds
	}
	if c.Routing.MaxRetries < 0 {
		return errors.New("routing.max_retries must be >= 0")
	}
	if c.Routing.MaxRetries == 0 {
		c.Routing.MaxRetries = DefaultMaxRetries
	}
	if c.Routing.CooldownSeconds <= 0 {
		c.Routing.CooldownSeconds = DefaultCooldown
	}

	if len(c.Apps) == 0 {
		return errors.New("apps must not be empty")
	}
	if len(c.Models) == 0 {
		return errors.New("models must not be empty")
	}

	for name, app := range c.Apps {
		app.AppID = strings.TrimSpace(app.AppID)
		app.APIKey = strings.TrimSpace(app.APIKey)
		app.APISecret = strings.TrimSpace(app.APISecret)
		if app.MaxConcurrency <= 0 {
			app.MaxConcurrency = 1
		}
		if app.AppID == "" || app.APIKey == "" || app.APISecret == "" {
			return fmt.Errorf("apps.%s requires app_id, api_key and api_secret", name)
		}
		c.Apps[name] = app
	}

	for name, model := range c.Models {
		model.DisplayName = strings.TrimSpace(model.DisplayName)
		model.ModelID = strings.TrimSpace(model.ModelID)
		model.ResourceID = strings.TrimSpace(model.ResourceID)
		model.PatchID = strings.TrimSpace(model.PatchID)
		model.Endpoint = strings.TrimSpace(model.Endpoint)
		if model.Endpoint == "" {
			model.Endpoint = c.XFYun.DefaultEndpoint
		}
		if model.DisplayName == "" {
			model.DisplayName = name
		}
		if model.ModelID == "" {
			return fmt.Errorf("models.%s.model_id is required", name)
		}
		if len(model.Apps) == 0 {
			return fmt.Errorf("models.%s.apps must not be empty", name)
		}
		for _, appName := range model.Apps {
			if _, ok := c.Apps[appName]; !ok {
				return fmt.Errorf("models.%s references unknown app %q", name, appName)
			}
		}
		if _, err := url.ParseRequestURI(model.Endpoint); err != nil {
			return fmt.Errorf("models.%s.endpoint is invalid: %w", name, err)
		}
		parsed, _ := url.Parse(model.Endpoint)
		if parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("models.%s.endpoint must include scheme and host", name)
		}
		if err := normalizeDefaults(&model.Defaults); err != nil {
			return fmt.Errorf("models.%s.defaults: %w", name, err)
		}
		c.Models[name] = model
	}

	return nil
}

func normalizeDefaults(defaults *ModelDefaults) error {
	defaults.Size = strings.TrimSpace(strings.ToLower(defaults.Size))
	if defaults.Size == "" {
		defaults.Size = DefaultSize
	}
	if _, ok := allowedSizes[defaults.Size]; !ok {
		return fmt.Errorf("unsupported size %q", defaults.Size)
	}
	if defaults.Steps <= 0 {
		defaults.Steps = DefaultSteps
	}
	if defaults.Steps > 50 {
		return fmt.Errorf("steps must be <= 50")
	}
	if defaults.GuidanceScale <= 0 {
		defaults.GuidanceScale = DefaultGuidanceScale
	}
	if defaults.GuidanceScale > 20 {
		return fmt.Errorf("guidance_scale must be <= 20")
	}
	defaults.Scheduler = strings.TrimSpace(defaults.Scheduler)
	if defaults.Scheduler == "" {
		defaults.Scheduler = DefaultScheduler
	}
	if _, ok := allowedSchedulers[defaults.Scheduler]; !ok {
		return fmt.Errorf("unsupported scheduler %q", defaults.Scheduler)
	}

	return nil
}

func (c *Config) SortedModelNames() []string {
	names := make([]string, 0, len(c.Models))
	for name := range c.Models {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func compactStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
