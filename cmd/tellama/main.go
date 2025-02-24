package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

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

	// Read the Telegram Bot API token
	telegramBotToken := viper.GetString("telegram_bot_token")
	if telegramBotToken == "" {
		log.Fatal().Msg("Telegram Bot API token is required")
	}

	// Read the database path
	viper.SetDefault("database_path", "tellama.db")
	databasePath := viper.GetString("database_path")
	log.Debug().Str("path", databasePath).Msg("Using database path")

	// Read history fetch limit
	viper.SetDefault("history_fetch_limit", 10000)
	historyFetchLimit := viper.GetInt("history_fetch_limit")
	log.Debug().Int("limit", historyFetchLimit).Msg("Using history fetch limit")

	// Read the Telegram timeout
	viper.SetDefault("telegram_timeout", 10*time.Second)
	telegramTimeout := viper.GetDuration("telegram_timeout")
	log.Debug().Dur("timeout", telegramTimeout).Msg("Using Telegram timeout")

	// Read the Generative AI timeout
	viper.SetDefault("genai_timeout", 10*time.Second)
	genaiTimeout := viper.GetDuration("genai_timeout")
	log.Debug().Dur("timeout", genaiTimeout).Msg("Using GenAI timeout")

	// Read the allow unauthorized chats flag
	viper.SetDefault("allow_unauthorized_chats", false)
	allowUnauthorizedChats := viper.GetBool("allow_unauthorized_chats")
	log.Debug().Bool("value", allowUnauthorizedChats).Msg("Allow unauthorized chats")

	// Read the Ollama host
	viper.SetDefault("ollama.host", "http://localhost:11434")
	ollamaHost := viper.GetString("ollama.host")
	log.Debug().Str("host", ollamaHost).Msg("Using Ollama host")

	// Read the Ollama model
	viper.SetDefault("ollama.model", "llama3.3:70b")
	ollamaModel := viper.GetString("ollama.model")
	if ollamaModel == "" {
		log.Fatal().Msg("Ollama model is required")
	}

	// Read Ollama options into a map
	ollamaOptions := map[string]interface{}{}
	for k, v := range viper.GetStringMap("ollama.options") {
		ollamaOptions[k] = v
		log.Debug().Str("key", k).Interface("value", v).Msg("Set Ollama option")
	}

	// Read the response messages
	responseMessages := ResponseMessages{
		privateChatDisallowed: viper.GetString("messages.private_chat_disallowed"),
		internalError:         viper.GetString("messages.internal_error"),
		serverBusy:            viper.GetString("messages.server_busy"),
	}

	// Initialize Tellama
	tellama, err := NewTellama(
		telegramBotToken,
		databasePath,
		historyFetchLimit,
		telegramTimeout,
		genaiTimeout,
		allowUnauthorizedChats,
		ollamaHost,
		ollamaModel,
		ollamaOptions,
		responseMessages,
		viper.GetString("template"),
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
