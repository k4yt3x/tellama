package genai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

type OpenAI struct {
	Client           *openai.Client
	Model            string
	FrequencyPenalty float64
	MaxTokens        int64
	PresencePenalty  float64
	ReasoningEffort  string
	Stop             string
	Temperature      float64
	TopP             float64
}

type OpenAIConfig struct {
	BaseURL          string
	APIKey           string
	Model            string
	FrequencyPenalty float64
	MaxTokens        int64
	PresencePenalty  float64
	ReasoningEffort  string
	Stop             string
	Temperature      float64
	TopP             float64
}

func (c *OpenAIConfig) Validate() error {
	if c.BaseURL == "" {
		return errors.New("base URL cannot be empty")
	}
	if c.APIKey == "" {
		return errors.New("API key cannot be empty")
	}
	if c.Model == "" {
		return errors.New("model cannot be empty")
	}
	return nil
}

func newOpenAIClient(config ProviderConfig) (GenerativeAI, error) {
	cfg, ok := config.(*OpenAIConfig)
	if !ok {
		return nil, errors.New("invalid config type for OpenAI")
	}

	return &OpenAI{
		Client: openai.NewClient(
			option.WithBaseURL(cfg.BaseURL),
			option.WithAPIKey(cfg.APIKey),
		),
		Model:            cfg.Model,
		FrequencyPenalty: cfg.FrequencyPenalty,
		MaxTokens:        cfg.MaxTokens,
		PresencePenalty:  cfg.PresencePenalty,
		ReasoningEffort:  cfg.ReasoningEffort,
		Stop:             cfg.Stop,
		Temperature:      cfg.Temperature,
		TopP:             cfg.TopP,
	}, nil
}

// Chat generates a response from Ollama using a conversation history.
func (o *OpenAI) Chat(messages []Message) (string, GenerateStats, error) {
	params := openai.ChatCompletionNewParams{
		Messages:            openai.F([]openai.ChatCompletionMessageParamUnion{}),
		Model:               openai.F(o.Model),
		FrequencyPenalty:    openai.F(o.FrequencyPenalty),
		MaxCompletionTokens: openai.F(o.MaxTokens),
		PresencePenalty:     openai.F(o.PresencePenalty),
		ReasoningEffort:     openai.F(openai.ChatCompletionReasoningEffort(o.ReasoningEffort)),
		Stop: openai.F[openai.ChatCompletionNewParamsStopUnion](
			shared.UnionString(o.Stop),
		),
		Temperature: openai.F(o.Temperature),
		TopP:        openai.F(o.TopP),
	}

	for _, message := range messages {
		switch message.Role {
		case "user":
			params.Messages.Value = append(
				params.Messages.Value,
				openai.UserMessage(message.Content),
			)
		case "assistant":
			params.Messages.Value = append(
				params.Messages.Value,
				openai.AssistantMessage(message.Content),
			)
		case "system":
			params.Messages.Value = append(
				params.Messages.Value,
				openai.SystemMessage(message.Content),
			)
		default:
			params.Messages.Value = append(
				params.Messages.Value,
				openai.UserMessage(message.Content),
			)
		}
	}

	startTime := time.Now()
	chatCompletion, err := o.Client.Chat.Completions.New(
		context.Background(),
		params,
	)
	if err != nil {
		return "", GenerateStats{}, fmt.Errorf("OpenAI failed to generate chat completion: %w", err)
	}
	duration := time.Since(startTime)

	if len(chatCompletion.Choices) == 0 {
		return "", GenerateStats{}, errors.New("OpenAI chat completion returned no choices")
	}
	choice := chatCompletion.Choices[0]

	genStats := GenerateStats{
		DoneReason:         string(choice.FinishReason),
		TotalDuration:      duration,
		LoadDuration:       -1,
		PromptTokens:       chatCompletion.Usage.PromptTokens,
		PromptEvalDuration: -1,
		TokenCount:         chatCompletion.Usage.CompletionTokens,
		EvalDuration:       duration,
	}

	return choice.Message.Content, genStats, nil
}

func (o *OpenAI) Complete(prompt string) (string, GenerateStats, error) {
	params := openai.CompletionNewParams{
		Model: openai.F(openai.CompletionNewParamsModel(o.Model)),
		Prompt: openai.F[openai.CompletionNewParamsPromptUnion](
			shared.UnionString(prompt),
		),
		FrequencyPenalty: openai.F(o.FrequencyPenalty),
		MaxTokens:        openai.F(o.MaxTokens),
		PresencePenalty:  openai.F(o.PresencePenalty),
		Stop:             openai.F[openai.CompletionNewParamsStopUnion](shared.UnionString(o.Stop)),
		Temperature:      openai.F(o.Temperature),
		TopP:             openai.F(o.TopP),
	}

	startTime := time.Now()
	chatCompletion, err := o.Client.Completions.New(
		context.Background(),
		params,
	)
	if err != nil {
		return "", GenerateStats{}, fmt.Errorf("OpenAI failed to generate completion: %w", err)
	}
	duration := time.Since(startTime)

	if len(chatCompletion.Choices) == 0 {
		return "", GenerateStats{}, errors.New("OpenAI completion returned no choices")
	}
	choice := chatCompletion.Choices[0]

	genStats := GenerateStats{
		DoneReason:         string(choice.FinishReason),
		TotalDuration:      duration,
		LoadDuration:       -1,
		PromptTokens:       chatCompletion.Usage.PromptTokens,
		PromptEvalDuration: -1,
		TokenCount:         chatCompletion.Usage.CompletionTokens,
		EvalDuration:       duration,
	}

	return choice.Text, genStats, nil
}
