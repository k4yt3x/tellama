package main

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/k4yt3x/tellama/internal/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// runBot is the Cobra command handler that sets up Tellama.
func runBot(cmd *cobra.Command, _ []string) {
	// Get the path to the config file
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse the config flag")
	}

	// Load configuration
	config, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize Tellama
	tellama, err := NewTellama(
		config.Telegram.BotToken,
		config.Database.Path,
		config.Database.HistoryFetchLimit,
		config.Telegram.Timeout,
		config.GenerativeAI.Timeout,
		config.Telegram.AllowUntrustedChat,
		config.GenerativeAI.Provider,
		config.GenerativeAI.Mode,
		config.GenerativeAI.Config,
		config.GenerativeAI.Template,
		config.GenerativeAI.AllowConcurrent,
		config.ResponseMessages,
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
