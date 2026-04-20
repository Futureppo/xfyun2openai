package openai

type ResponseFormat string

const (
	ResponseFormatB64JSON ResponseFormat = "b64_json"
	ResponseFormatURL     ResponseFormat = "url"
)

type OpenAIImageRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	N              *int            `json:"n,omitempty"`
	Size           *string         `json:"size,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	User           string          `json:"user,omitempty"`
	XFYun          *XFYunOptions   `json:"x_fyun,omitempty"`
}

type XFYunOptions struct {
	NegativePrompt string   `json:"negative_prompt,omitempty"`
	Seed           *int64   `json:"seed,omitempty"`
	Steps          *int     `json:"steps,omitempty"`
	GuidanceScale  *float64 `json:"guidance_scale,omitempty"`
	Scheduler      string   `json:"scheduler,omitempty"`
	PatchID        string   `json:"patch_id,omitempty"`
}

type OpenAIImageResponse struct {
	Created int64             `json:"created"`
	Data    []OpenAIImageData `json:"data"`
}

type OpenAIImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ModelListResponse struct {
	Object string      `json:"object"`
	Data   []ModelItem `json:"data"`
}

type ModelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
