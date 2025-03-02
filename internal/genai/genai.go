package genai

import (
	"errors"
	"time"
)

type Provider int

const (
	ProviderOllama Provider = iota
	ProviderOpenAI
)

func (p Provider) String() string {
	return [...]string{"ollama", "openai"}[p]
}

func ParseProvider(s string) (Provider, error) {
	switch s {
	case "ollama":
		return ProviderOllama, nil
	case "openai":
		return ProviderOpenAI, nil
	default:
		return 0, errors.New("unknown provider")
	}
}

type Mode int

const (
	ModeChat Mode = iota
	ModeCompletion
)

func (m Mode) String() string {
	return [...]string{"chat", "completion"}[m]
}

func ParseMode(s string) (Mode, error) {
	switch s {
	case "chat":
		return ModeChat, nil
	case "completion":
		return ModeCompletion, nil
	default:
		return 0, errors.New("unknown mode")
	}
}

type Message struct {
	Role    string
	Content string
}

type GenerateStats struct {
	DoneReason         string
	TotalDuration      time.Duration
	LoadDuration       time.Duration
	PromptTokens       int64
	PromptEvalDuration time.Duration
	TokenCount         int64
	EvalDuration       time.Duration
}

type GenerativeAI interface {
	Chat(messages []Message) (string, GenerateStats, error)
	Complete(prompt string) (string, GenerateStats, error)
}
