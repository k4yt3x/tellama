package genai

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type OpenAI struct {
	Client           *openai.Client
	Model            string
	ReasoningEffort  string
	FrequencyPenalty float64
	PresencePenalty  float64
	Temperature      float64
	TopP             float64
}

type OpenAIConfig struct {
	BaseURL          string
	APIKey           string
	Model            string
	ReasoningEffort  string
	FrequencyPenalty float64
	PresencePenalty  float64
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
		ReasoningEffort:  cfg.ReasoningEffort,
		FrequencyPenalty: cfg.FrequencyPenalty,
		PresencePenalty:  cfg.PresencePenalty,
		Temperature:      cfg.Temperature,
		TopP:             cfg.TopP,
	}, nil
}

// Chat generates a response from Ollama using a conversation history.
func (o *OpenAI) Chat(messages []Message) (string, GenerateStats, error) {
	param := openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{}),
		Seed:     openai.Int(1),
		Model:    openai.F(openai.ChatModelGPT4o),
	}

	for _, message := range messages {
		switch message.Role {
		case "user":
			param.Messages.Value = append(param.Messages.Value, openai.UserMessage(message.Content))
		case "assistant":
			param.Messages.Value = append(
				param.Messages.Value,
				openai.AssistantMessage(message.Content),
			)
		case "system":
			param.Messages.Value = append(
				param.Messages.Value,
				openai.SystemMessage(message.Content),
			)
		default:
			param.Messages.Value = append(param.Messages.Value, openai.UserMessage(message.Content))
		}
	}

	chatCompletion, err := o.Client.Chat.Completions.New(
		context.Background(),
		param,
	)
	if err != nil {
		return "", GenerateStats{}, fmt.Errorf("OpenAI failed to generate chat completion: %w", err)
	}

	if len(chatCompletion.Choices) == 0 {
		return "", GenerateStats{}, errors.New("OpenAI chat completion returned no choices")
	}
	choice := chatCompletion.Choices[0]

	genStats := GenerateStats{
		DoneReason:         string(choice.FinishReason),
		TotalDuration:      -1,
		LoadDuration:       -1,
		PromptTokens:       chatCompletion.Usage.PromptTokens,
		PromptEvalDuration: -1,
		TokenCount:         chatCompletion.Usage.CompletionTokens,
		EvalDuration:       -1,
	}

	return choice.Message.Content, genStats, nil
}

func (o *OpenAI) Complete(_ string) (string, GenerateStats, error) {
	return "", GenerateStats{}, errors.New("completion mode is not supported by OpenAI")
}
