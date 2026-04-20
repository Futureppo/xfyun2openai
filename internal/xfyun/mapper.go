package xfyun

import (
	"fmt"
	"strings"

	"xfyun2openai/internal/config"
	"xfyun2openai/internal/openai"
)

var allowedSizes = map[string][2]int{
	"768x768":   {768, 768},
	"1024x1024": {1024, 1024},
	"576x1024":  {576, 1024},
	"768x1024":  {768, 1024},
	"1024x576":  {1024, 576},
	"1024x768":  {1024, 768},
}

func ParseSize(raw string) (int, int, error) {
	size := strings.TrimSpace(strings.ToLower(raw))
	size = strings.ReplaceAll(size, "x", "x")
	dims, ok := allowedSizes[size]
	if !ok {
		return 0, 0, fmt.Errorf("unsupported size %q", raw)
	}

	return dims[0], dims[1], nil
}

func BuildRequest(
	app config.AppConfig,
	model config.ModelConfig,
	req openai.OpenAIImageRequest,
	seed int64,
	uid string,
) (GenerateRequest, error) {
	size := config.DefaultSize
	if model.Defaults.Size != "" {
		size = model.Defaults.Size
	}
	if req.Size != nil && strings.TrimSpace(*req.Size) != "" {
		size = strings.ToLower(strings.TrimSpace(*req.Size))
	}

	width, height, err := ParseSize(size)
	if err != nil {
		return GenerateRequest{}, err
	}

	steps := config.DefaultSteps
	if model.Defaults.Steps > 0 {
		steps = model.Defaults.Steps
	}
	if req.XFYun != nil && req.XFYun.Steps != nil {
		steps = *req.XFYun.Steps
	}

	guidanceScale := config.DefaultGuidanceScale
	if model.Defaults.GuidanceScale > 0 {
		guidanceScale = model.Defaults.GuidanceScale
	}
	if req.XFYun != nil && req.XFYun.GuidanceScale != nil {
		guidanceScale = *req.XFYun.GuidanceScale
	}

	scheduler := config.DefaultScheduler
	if model.Defaults.Scheduler != "" {
		scheduler = model.Defaults.Scheduler
	}
	if req.XFYun != nil && strings.TrimSpace(req.XFYun.Scheduler) != "" {
		scheduler = strings.TrimSpace(req.XFYun.Scheduler)
	}

	header := RequestHeader{
		AppID: app.AppID,
		UID:   uid,
	}
	if strings.TrimSpace(model.PatchID) != "" {
		header.PatchID = []string{strings.TrimSpace(model.PatchID)}
	}
	if req.XFYun != nil && strings.TrimSpace(req.XFYun.PatchID) != "" {
		header.PatchID = []string{strings.TrimSpace(req.XFYun.PatchID)}
	}

	payload := RequestPayload{
		Message: MessagePayload{
			Text: []MessageText{{
				Role:    "user",
				Content: req.Prompt,
			}},
		},
	}
	if req.XFYun != nil && strings.TrimSpace(req.XFYun.NegativePrompt) != "" {
		payload.NegativePrompts = &NegativePrompts{
			Text: strings.TrimSpace(req.XFYun.NegativePrompt),
		}
	}

	return GenerateRequest{
		Header: header,
		Parameter: RequestParameter{
			Chat: ChatParameter{
				Domain:            model.ModelID,
				Width:             width,
				Height:            height,
				Seed:              seed,
				NumInferenceSteps: steps,
				GuidanceScale:     guidanceScale,
				Scheduler:         scheduler,
			},
		},
		Payload: payload,
	}, nil
}
