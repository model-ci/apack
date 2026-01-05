package types

import (
	"context"
	"time"
)

type Runner interface {
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
	Restart() error

	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	CompleteStream(ctx context.Context, req *CompletionRequest) (<-chan CompletionChunk, error)
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error)
	Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)
	Model(ctx context.Context) (*ModelListResponse, error)

	StartAPI(ctx context.Context) error
	StopAPI() error
	IsAPIRunning() bool
	GetApiURL() string

	Health() (*HealthStatus, error)
	Metrics() (*Metrics, error)
	OnEvent(handler EventHandler)
}

type CompletionRequest struct {
	Prompt      string   `json:"prompt"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float32  `json:"temperature,omitempty"`
	TopP        float32  `json:"top_p,omitempty"`
	TopK        int      `json:"top_k,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
}

type CompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type CompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChoiceChunk `json:"choices"`
}

type ChatRequest struct {
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float32       `json:"temperature,omitempty"`
	TopP        float32       `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

type ChatChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []ChatChoiceChunk `json:"choices"`
}

type EmbedRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model,omitempty"`
}

type EmbedResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

type Embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

type Model struct {
	Created int64  `json:"created"`
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type ModelListResponse struct {
	Data   []Model `json:"data"`
	Object string  `json:"object"`
}

type Choice struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

type ChoiceChunk struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatChoiceChunk struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    time.Duration     `json:"uptime"`
	Services  map[string]string `json:"services"`
	Model     ModelInfo         `json:"model"`
}

type ModelInfo struct {
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	ContextSize  int    `json:"context_size"`
	Architecture string `json:"architecture"`
}

type Metrics struct {
	RequestsTotal     int64         `json:"requests_total"`
	RequestsPerSecond float64       `json:"requests_per_second"`
	AverageLatency    time.Duration `json:"average_latency"`
	TokensGenerated   int64         `json:"tokens_generated"`
	TokensPerSecond   float64       `json:"tokens_per_second"`
}

type EventType string

const (
	EventStarted   EventType = "started"
	EventStopped   EventType = "stopped"
	EventError     EventType = "error"
	EventRestarted EventType = "restarted"
	EventUIStarted EventType = "ui_started"
	EventUIStopped EventType = "ui_stopped"
)

type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

type LlamaCompletionResponse struct {
	Content            string             `json:"content"`
	Model              string             `json:"model"`
	Prompt             string             `json:"prompt"`
	SlotID             int                `json:"slot_id"`
	Stop               bool               `json:"stop"`
	StoppedEOS         bool               `json:"stopped_eos"`
	StoppedLimit       bool               `json:"stopped_limit"`
	StoppedWord        bool               `json:"stopped_word"`
	StoppingWord       string             `json:"stopping_word"`
	TokensCached       int                `json:"tokens_cached"`
	TokensEvaluated    int                `json:"tokens_evaluated"`
	TokensPredicted    int                `json:"tokens_predicted"`
	Truncated          bool               `json:"truncated"`
	GenerationSettings GenerationSettings `json:"generation_settings"`
	Timings            Timings            `json:"timings"`
}

type GenerationSettings struct {
	Temperature   float64  `json:"temperature"`
	TopP          float64  `json:"top_p"`
	TopK          int      `json:"top_k"`
	NPredict      int      `json:"n_predict"`
	Stop          []string `json:"stop"`
	RepeatPenalty float64  `json:"repeat_penalty"`
}

type Timings struct {
	PredictedMS         float64 `json:"predicted_ms"`
	PredictedN          int     `json:"predicted_n"`
	PredictedPerSecond  float64 `json:"predicted_per_second"`
	PredictedPerTokenMS float64 `json:"predicted_per_token_ms"`
	PromptMS            float64 `json:"prompt_ms"`
	PromptN             int     `json:"prompt_n"`
	PromptPerSecond     float64 `json:"prompt_per_second"`
	PromptPerTokenMS    float64 `json:"prompt_per_token_ms"`
}

type EventHandler func(event Event)
