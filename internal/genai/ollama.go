package genai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ollama/ollama/api"
)

type Ollama struct {
	Client  *api.Client
	Model   string
	Options map[string]any
}

type OllamaConfig struct {
	BaseURL string
	Model   string
	Options map[string]any
}

func (c *OllamaConfig) Validate() error {
	if c.BaseURL == "" {
		return errors.New("host cannot be empty")
	}
	if c.Model == "" {
		return errors.New("model cannot be empty")
	}
	return nil
}

func newOllamaClient(config ProviderConfig) (GenerativeAI, error) {
	cfg, ok := config.(*OllamaConfig)
	if !ok {
		return nil, errors.New("invalid config type for Ollama")
	}

	baseURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}

	return &Ollama{
		Client:  api.NewClient(baseURL, http.DefaultClient),
		Model:   cfg.Model,
		Options: cfg.Options,
	}, nil
}

// Chat generates a response from Ollama using a conversation history.
func (o *Ollama) Chat(messages []Message) (string, GenerateStats, error) {
	apiMessages := make([]api.Message, len(messages))
	for i, message := range messages {
		apiMessages[i] = api.Message{
			Role:    message.Role,
			Content: message.Content,
		}
	}

	var responseBuilder strings.Builder
	var chatResp api.ChatResponse

	err := o.Client.Chat(
		context.Background(),
		&api.ChatRequest{
			Model:    o.Model,
			Messages: apiMessages,
			Options:  o.Options,
		},
		func(resp api.ChatResponse) error {
			chatResp = resp
			responseBuilder.WriteString(resp.Message.Content)
			return nil
		},
	)
	if err != nil {
		return "", GenerateStats{}, err
	}

	genStats := GenerateStats{
		DoneReason:         chatResp.DoneReason,
		TotalDuration:      chatResp.Metrics.TotalDuration,
		LoadDuration:       chatResp.Metrics.LoadDuration,
		PromptTokens:       int64(chatResp.Metrics.PromptEvalCount),
		PromptEvalDuration: chatResp.Metrics.PromptEvalDuration,
		TokenCount:         int64(chatResp.Metrics.EvalCount),
		EvalDuration:       chatResp.Metrics.EvalDuration,
	}

	return responseBuilder.String(), genStats, nil
}

func (o *Ollama) Complete(prompt string) (string, GenerateStats, error) {
	var responseBuilder strings.Builder
	var generateResp api.GenerateResponse

	err := o.Client.Generate(
		context.Background(),
		&api.GenerateRequest{
			Model:   o.Model,
			Prompt:  prompt,
			Raw:     true,
			Options: o.Options,
		},
		func(resp api.GenerateResponse) error {
			generateResp = resp
			responseBuilder.WriteString(resp.Response)
			return nil
		},
	)
	if err != nil {
		return "", GenerateStats{}, err
	}

	response := strings.TrimSpace(responseBuilder.String())

	genStats := GenerateStats{
		DoneReason:         generateResp.DoneReason,
		TotalDuration:      generateResp.Metrics.TotalDuration,
		LoadDuration:       generateResp.Metrics.LoadDuration,
		PromptTokens:       int64(generateResp.Metrics.PromptEvalCount),
		PromptEvalDuration: generateResp.Metrics.PromptEvalDuration,
		TokenCount:         int64(generateResp.Metrics.EvalCount),
		EvalDuration:       generateResp.Metrics.EvalDuration,
	}

	return response, genStats, nil
}
