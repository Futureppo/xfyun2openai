package xfyun

type GenerateRequest struct {
	Header    RequestHeader    `json:"header"`
	Parameter RequestParameter `json:"parameter"`
	Payload   RequestPayload   `json:"payload"`
}

type RequestHeader struct {
	AppID   string   `json:"app_id"`
	UID     string   `json:"uid,omitempty"`
	PatchID []string `json:"patch_id,omitempty"`
}

type RequestParameter struct {
	Chat ChatParameter `json:"chat"`
}

type ChatParameter struct {
	Domain            string  `json:"domain"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	Seed              int64   `json:"seed"`
	NumInferenceSteps int     `json:"num_inference_steps"`
	GuidanceScale     float64 `json:"guidance_scale"`
	Scheduler         string  `json:"scheduler"`
}

type RequestPayload struct {
	Message         MessagePayload   `json:"message"`
	NegativePrompts *NegativePrompts `json:"negative_prompts,omitempty"`
}

type MessagePayload struct {
	Text []MessageText `json:"text"`
}

type MessageText struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type NegativePrompts struct {
	Text string `json:"text"`
}

type GenerateResponse struct {
	Header  ResponseHeader  `json:"header"`
	Payload ResponsePayload `json:"payload"`
}

type ResponseHeader struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	SID     string `json:"sid"`
	Status  int    `json:"status"`
}

type ResponsePayload struct {
	Choices ChoicesPayload `json:"choices"`
}

type ChoicesPayload struct {
	Status int          `json:"status"`
	Seq    int          `json:"seq"`
	Text   []ChoiceText `json:"text"`
}

type ChoiceText struct {
	Content string `json:"content"`
	Index   int    `json:"index"`
	Role    string `json:"role"`
}
