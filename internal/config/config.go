package config

import (
	"errors"
	"time"

	"github.com/k4yt3x/tellama/internal/genai"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Config holds all the configuration values for the application.
type Config struct {
	Database struct {
		Path              string
		HistoryFetchLimit int
	}
	Telegram struct {
		BotToken           string
		Timeout            time.Duration
		AllowUntrustedChat bool
	}
	GenerativeAI struct {
		Provider        genai.Provider
		Mode            genai.Mode
		Timeout         time.Duration
		AllowConcurrent bool
		Template        string
		Config          genai.ProviderConfig
	}
	ResponseMessages ResponseMessages
}

// ResponseMessages contains customizable message templates for different scenarios.
type ResponseMessages struct {
	PrivateChatDisallowed string
	InternalError         string
	ServerBusy            string
}

// setupConfigPaths configures viper with the paths to look for config files.
func setupConfigPaths(configPath string) {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("tellama")
		viper.AddConfigPath(".")
		viper.AddConfigPath("configs/")
		viper.AddConfigPath("/etc/tellama/")
	}
}

// logConfigFile logs the path to the config file being used.
func logConfigFile() {
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		log.Info().Str("path", configFile).Msg("Read configs from file")
	}
}

// setDefaultValues sets default values for configuration options.
func setDefaultValues() {
	// Database defaults
	viper.SetDefault("database.path", "tellama.db")
	viper.SetDefault("database.history_fetch_limit", 10000)

	// Telegram defaults
	viper.SetDefault("telegram.timeout", 10*time.Second)
	viper.SetDefault("telegram.allow_untrusted_chats", false)

	// GenAI defaults
	viper.SetDefault("genai.timeout", 10*time.Second)
	viper.SetDefault("genai.allow_concurrent", false)
	viper.SetDefault("genai.mode", "chat")

	// Ollama defaults
	viper.SetDefault("ollama.base_url", "http://localhost:11434")
	viper.SetDefault("ollama.model", "llama3.3:70b")

	// OpenAI defaults
	viper.SetDefault("openai.base_url", "https://api.openai.com/v1/")
	viper.SetDefault("openai.model", "gpt-4o")
	viper.SetDefault("openai.frequency_penalty", 0.0)
	viper.SetDefault("openai.presence_penalty", 0.0)
	viper.SetDefault("openai.reasoning_effort", "medium")
	viper.SetDefault("openai.temperature", 1.0)
	viper.SetDefault("openai.top_p", 1.0)
}

// createOllamaConfig creates Ollama provider configuration.
func createOllamaConfig() *genai.OllamaConfig {
	ollamaOptions := map[string]any{}
	for k, v := range viper.GetStringMap("ollama.options") {
		ollamaOptions[k] = v
		log.Debug().Str("key", k).Interface("value", v).Msg("Set Ollama option")
	}

	ollamaBaseURL := viper.GetString("ollama.base_url")
	ollamaModel := viper.GetString("ollama.model")

	log.Debug().Str("host", ollamaBaseURL).Msg("Using Ollama base URL")
	log.Debug().Str("model", ollamaModel).Msg("Using Ollama model")

	return &genai.OllamaConfig{
		BaseURL: ollamaBaseURL,
		Model:   ollamaModel,
		Options: ollamaOptions,
	}
}

// createOpenAIConfig creates OpenAI provider configuration.
func createOpenAIConfig() (*genai.OpenAIConfig, error) {
	openaiBaseURL := viper.GetString("openai.base_url")
	openaiAPIKey := viper.GetString("openai.api_key")
	openaiModel := viper.GetString("openai.model")

	if openaiAPIKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	log.Debug().Str("base_url", openaiBaseURL).Msg("Using OpenAI base URL")
	log.Debug().Str("model", openaiModel).Msg("Using OpenAI model")

	return &genai.OpenAIConfig{
		BaseURL:          openaiBaseURL,
		APIKey:           openaiAPIKey,
		Model:            openaiModel,
		FrequencyPenalty: viper.GetFloat64("openai.frequency_penalty"),
		PresencePenalty:  viper.GetFloat64("openai.presence_penalty"),
		ReasoningEffort:  viper.GetString("openai.reasoning_effort"),
		Temperature:      viper.GetFloat64("openai.temperature"),
		TopP:             viper.GetFloat64("openai.top_p"),
	}, nil
}

// createProviderConfig creates the provider-specific configuration.
func createProviderConfig(provider genai.Provider, mode genai.Mode) (genai.ProviderConfig, error) {
	switch provider {
	case genai.ProviderOllama:
		return createOllamaConfig(), nil
	case genai.ProviderOpenAI:
		config, err := createOpenAIConfig()
		if err != nil {
			return nil, err
		}
		return config, nil
	default:
		return nil, errors.New("unsupported generative AI provider")
	}
}

// Load loads the configuration file and returns a Config struct.
func Load(configPath string) (*Config, error) {
	setupConfigPaths(configPath)

	if err := viper.ReadInConfig(); err != nil {
		var cfErr viper.ConfigFileNotFoundError
		if !errors.As(err, &cfErr) {
			return nil, err
		}
	}

	logConfigFile()
	setDefaultValues()

	config := &Config{}
	config.Database.Path = viper.GetString("database.path")
	config.Database.HistoryFetchLimit = viper.GetInt("database.history_fetch_limit")
	log.Debug().Str("path", config.Database.Path).Msg("Using database path")
	log.Debug().Int("limit", config.Database.HistoryFetchLimit).Msg("Using history fetch limit")

	// Telegram settings
	config.Telegram.BotToken = viper.GetString("telegram.bot_token")
	if config.Telegram.BotToken == "" {
		return nil, errors.New("telegram bot token is required")
	}
	config.Telegram.Timeout = viper.GetDuration("telegram.timeout")
	config.Telegram.AllowUntrustedChat = viper.GetBool("telegram.allow_untrusted_chats")
	log.Debug().Dur("timeout", config.Telegram.Timeout).Msg("Using Telegram timeout")
	log.Debug().Bool("value", config.Telegram.AllowUntrustedChat).Msg("Allow untrusted chats")

	// GenAI settings
	provider, err := genai.ParseProvider(viper.GetString("genai.provider"))
	if err != nil {
		return nil, err
	}
	config.GenerativeAI.Provider = provider
	mode, err := genai.ParseMode(viper.GetString("genai.mode"))
	if err != nil {
		return nil, err
	}
	config.GenerativeAI.Mode = mode
	config.GenerativeAI.Timeout = viper.GetDuration("genai.timeout")
	config.GenerativeAI.AllowConcurrent = viper.GetBool("genai.allow_concurrent")
	config.GenerativeAI.Template = viper.GetString("genai.template")
	log.Debug().
		Str("provider", config.GenerativeAI.Provider.String()).
		Msg("Using generative AI provider")
	log.Debug().Str("mode", config.GenerativeAI.Mode.String()).Msg("Using generative AI mode")
	log.Debug().Dur("timeout", config.GenerativeAI.Timeout).Msg("Using generative AI timeout")
	log.Debug().
		Bool("value", config.GenerativeAI.AllowConcurrent).
		Msg("Allow concurrent generative AI requests")

	// Set provider-specific config
	config.GenerativeAI.Config, err = createProviderConfig(provider, mode)
	if err != nil {
		return nil, err
	}

	// Validation
	if config.GenerativeAI.Template == "" && config.GenerativeAI.Mode == genai.ModeCompletion {
		return nil, errors.New("template is required for completion mode")
	}

	// Response messages
	config.ResponseMessages = ResponseMessages{
		PrivateChatDisallowed: viper.GetString("messages.private_chat_disallowed"),
		InternalError:         viper.GetString("messages.internal_error"),
		ServerBusy:            viper.GetString("messages.server_busy"),
	}

	return config, nil
}
