package genai

import (
	"fmt"
)

type ProviderConfig interface {
	Validate() error
}

type ProviderFactory func(ProviderConfig) (GenerativeAI, error)

func New(p Provider, config ProviderConfig) (GenerativeAI, error) {
	providerRegistry := map[Provider]ProviderFactory{
		ProviderOllama: newOllamaClient,
		ProviderOpenAI: newOpenAIClient,
	}

	factory, exists := providerRegistry[p]
	if !exists {
		return nil, fmt.Errorf("provider %s not supported", p)
	}

	err := config.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return factory(config)
}
