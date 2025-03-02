package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/k4yt3x/tellama/internal/genai"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runBot is the Cobra command handler that sets up Tellama.
func runBot(cmd *cobra.Command, _ []string) {
	// Get the path to the config file
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse the config flag")
	}

	// If a config file path is explicitly provided, use it
	// Otherwise, search for the config file
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("tellama")
		viper.AddConfigPath(".")
		viper.AddConfigPath("configs/")
		viper.AddConfigPath("/etc/tellama/")
	}

	// Read the config file
	err = viper.ReadInConfig()
	if err != nil {
		var cfErr viper.ConfigFileNotFoundError
		// Ignore the config file not found error
		if !errors.As(err, &cfErr) {
			log.Fatal().Err(err).Msg("Error reading config file")
		}
	}

	// Log the config file path
	configFile := viper.ConfigFileUsed()
	log.Info().Str("path", configFile).Msg("Read configs from file")

	// Read the database path
	viper.SetDefault("database.path", "tellama.db")
	databasePath := viper.GetString("database.path")
	log.Debug().Str("path", databasePath).Msg("Using database path")

	// Read history fetch limit
	viper.SetDefault("database.history_fetch_limit", 10000)
	historyFetchLimit := viper.GetInt("database.history_fetch_limit")
	log.Debug().Int("limit", historyFetchLimit).Msg("Using history fetch limit")

	// Read the Telegram Bot API token
	telegramBotToken := viper.GetString("telegram.bot_token")
	if telegramBotToken == "" {
		log.Fatal().Msg("Telegram Bot API token is required")
	}

	// Read the Telegram timeout
	viper.SetDefault("telegram.timeout", 10*time.Second)
	telegramTimeout := viper.GetDuration("telegram.timeout")
	log.Debug().Dur("timeout", telegramTimeout).Msg("Using Telegram timeout")

	// Read the allow untrusted chats flag
	viper.SetDefault("telegram.allow_untrusted_chats", false)
	allowUntrustedChats := viper.GetBool("telegram.allow_untrusted_chats")
	log.Debug().Bool("value", allowUntrustedChats).Msg("Allow untrusted chats")
	log.Debug().Dur("timeout", telegramTimeout).Msg("Using Telegram timeout")

	// Read the generative AI timeout
	viper.SetDefault("genai.timeout", 10*time.Second)
	genaiTimeout := viper.GetDuration("genai.timeout")
	log.Debug().Dur("timeout", genaiTimeout).Msg("Using generative AI timeout")

	// Read the generative AI allow concurrent flag
	viper.SetDefault("genai.allow_concurrent", false)
	genaiAllowConcurrent := viper.GetBool("genai.allow_concurrent")
	log.Debug().Bool("value", genaiAllowConcurrent).Msg("Allow concurrent generative AI requests")

	// Read the generative AI provider
	genaiProvider, err := genai.ParseProvider(viper.GetString("genai.provider"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse the generative AI provider")
	}
	log.Debug().Str("provider", genaiProvider.String()).Msg("Using generative AI provider")

	// Read the generative AI mode
	viper.SetDefault("genai.mode", "chat")
	genaiMode, err := genai.ParseMode(viper.GetString("genai.mode"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse the generative AI mode")
	}
	log.Debug().Str("mode", genaiMode.String()).Msg("Using generative AI mode")

	var genaiConfig genai.ProviderConfig
	switch genaiProvider {
	case genai.ProviderOllama:
		// Read Ollama options into a map
		ollamaOptions := map[string]interface{}{}
		for k, v := range viper.GetStringMap("ollama.options") {
			ollamaOptions[k] = v
			log.Debug().Str("key", k).Interface("value", v).Msg("Set Ollama option")
		}

		// Read the Ollama host
		viper.SetDefault("ollama.base_url", "http://localhost:11434")
		ollamaBaseURL := viper.GetString("ollama.base_url")
		log.Debug().Str("host", ollamaBaseURL).Msg("Using Ollama base URL")

		// Read the Ollama model
		viper.SetDefault("ollama.model", "llama3.3:70b")
		ollamaModel := viper.GetString("ollama.model")
		log.Debug().Str("model", ollamaModel).Msg("Using Ollama model")

		genaiConfig = &genai.OllamaConfig{
			BaseURL: ollamaBaseURL,
			Model:   ollamaModel,
			Options: ollamaOptions,
		}
	case genai.ProviderOpenAI:
		if genaiMode == genai.ModeCompletion {
			log.Fatal().Msg("OpenAI provider does not support completion mode")
		}

		// Read the OpenAI base URL
		viper.SetDefault("openai.base_url", "https://api.openai.com/v1/")
		openaiBaseURL := viper.GetString("openai.base_url")
		if openaiBaseURL == "" {
			log.Fatal().Msg("OpenAI base URL is required")
		}
		log.Debug().Str("base_url", openaiBaseURL).Msg("Using OpenAI base URL")

		// Read the OpenAI API key
		openaiAPIKey := viper.GetString("openai.api_key")
		if openaiAPIKey == "" {
			log.Fatal().Msg("OpenAI API key is required")
		}

		// Read the OpenAI model
		viper.SetDefault("openai.model", "gpt-4o")
		openaiModel := viper.GetString("openai.model")
		log.Debug().Str("model", openaiModel).Msg("Using OpenAI model")

		// Set default OpenAI options values
		viper.SetDefault("openai.reasoning_effort", "medium")
		viper.SetDefault("openai.frequency_penalty", 0.0)
		viper.SetDefault("openai.presence_penalty", 0.0)
		viper.SetDefault("openai.temperature", 1.0)
		viper.SetDefault("openai.top_p", 1.0)

		genaiConfig = &genai.OpenAIConfig{
			BaseURL:          openaiBaseURL,
			APIKey:           openaiAPIKey,
			Model:            openaiModel,
			ReasoningEffort:  viper.GetString("openai.reasoning_effort"),
			FrequencyPenalty: viper.GetFloat64("openai.frequency_penalty"),
			PresencePenalty:  viper.GetFloat64("openai.presence_penalty"),
			Temperature:      viper.GetFloat64("openai.temperature"),
			TopP:             viper.GetFloat64("openai.top_p"),
		}
	default:
		log.Fatal().Msg("Unsupported generative AI provider")
	}

	genaiTemplate := viper.GetString("genai.template")
	if genaiTemplate == "" && genaiMode == genai.ModeCompletion {
		log.Fatal().Msg("Template is required for completion mode")
	}

	// Read the response messages
	responseMessages := ResponseMessages{
		PrivateChatDisallowed: viper.GetString("messages.private_chat_disallowed"),
		InternalError:         viper.GetString("messages.internal_error"),
		ServerBusy:            viper.GetString("messages.server_busy"),
	}

	// Initialize Tellama
	tellama, err := NewTellama(
		telegramBotToken,
		databasePath,
		historyFetchLimit,
		telegramTimeout,
		genaiTimeout,
		allowUntrustedChats,
		genaiProvider,
		genaiMode,
		genaiConfig,
		genaiTemplate,
		genaiAllowConcurrent,
		responseMessages,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Tellama")
	}

	// Run Tellama
	tellama.Run()
}

func main() {
	// Configure zerolog
	zerolog.CallerMarshalFunc = func( //nolint:reassign // Override the default caller marshal function
		_ uintptr, file string, line int,
	) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
	log.Logger = log.Output( //nolint:reassign // Override the default logger
		zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Caller().
		Timestamp().
		Logger()
	log.Info().Msgf("Initializing Tellama %s", Version)

	// Set up the root command
	cmd := &cobra.Command{
		Use: "tellama",
		Run: runBot,
	}

	// Add flags to the root command
	cmd.PersistentFlags().StringP("config", "c", "", "Path to Tellama config file")

	// Execute the root command
	err := cmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to execute root command")
	}
}
