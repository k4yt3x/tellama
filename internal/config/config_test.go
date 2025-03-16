package config //nolint:testpackage // Unit tests are in the same package

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k4yt3x/tellama/internal/genai"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetViper resets viper state between tests.
func resetViper() {
	viper.Reset()
	viper.SetConfigType("yaml")
}

func TestLoad_ValidConfig(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
database:
  path: test.db
  history_fetch_limit: 100
telegram:
  bot_token: test_token
  timeout: 5s
  allow_untrusted_chats: true
genai:
  provider: openai
  mode: chat
  timeout: 15s
  allow_concurrent: true
openai:
  api_key: test_api_key
  model: gpt-4
messages:
  private_chat_disallowed: "Private chats not allowed"
  internal_error: "Error occurred"
  server_busy: "Server is busy"
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	cfg, err := Load(configPath)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "test.db", cfg.Database.Path)
	assert.Equal(t, 100, cfg.Database.HistoryFetchLimit)
	assert.Equal(t, "test_token", cfg.Telegram.BotToken)
	assert.Equal(t, 5*time.Second, cfg.Telegram.Timeout)
	assert.True(t, cfg.Telegram.AllowUntrustedChat)
	assert.Equal(t, genai.ProviderOpenAI, cfg.GenerativeAI.Provider)
	assert.Equal(t, genai.ModeChat, cfg.GenerativeAI.Mode)
	assert.Equal(t, 15*time.Second, cfg.GenerativeAI.Timeout)
	assert.True(t, cfg.GenerativeAI.AllowConcurrent)
	assert.Equal(t, "Private chats not allowed", cfg.ResponseMessages.PrivateChatDisallowed)
	assert.Equal(t, "Error occurred", cfg.ResponseMessages.InternalError)
	assert.Equal(t, "Server is busy", cfg.ResponseMessages.ServerBusy)

	// Check OpenAI config
	openaiCfg, ok := cfg.GenerativeAI.Config.(*genai.OpenAIConfig)
	require.True(t, ok)
	assert.Equal(t, "test_api_key", openaiCfg.APIKey)
	assert.Equal(t, "gpt-4", openaiCfg.Model)
}

func TestLoad_OllamaConfig(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
database:
  path: test.db
telegram:
  bot_token: test_token
genai:
  provider: ollama
  mode: chat
ollama:
  base_url: http://ollama-server:11434
  model: llama3:latest
  options:
    temperature: 0.8
    top_k: 50.0
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	cfg, err := Load(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, genai.ProviderOllama, cfg.GenerativeAI.Provider)
	ollamaCfg, ok := cfg.GenerativeAI.Config.(*genai.OllamaConfig)
	require.True(t, ok)
	assert.Equal(t, "http://ollama-server:11434", ollamaCfg.BaseURL)
	assert.Equal(t, "llama3:latest", ollamaCfg.Model)
	assert.InEpsilon(t, 0.8, ollamaCfg.Options["temperature"], 0.0001)
	assert.InEpsilon(t, 50, ollamaCfg.Options["top_k"], 0.0001)
}

func TestLoad_CompletionMode(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
database:
  path: test.db
telegram:
  bot_token: test_token
genai:
  provider: ollama
  mode: completion
  template: "Answer this question: {{.question}}"
ollama:
  base_url: http://ollama-server:11434
  model: llama3:latest
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	cfg, err := Load(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, genai.ModeCompletion, cfg.GenerativeAI.Mode)
	assert.Equal(t, "Answer this question: {{.question}}", cfg.GenerativeAI.Template)
}

func TestLoad_MissingBotToken(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
database:
  path: test.db
genai:
  provider: ollama
  mode: chat
ollama:
  base_url: http://ollama-server:11434
  model: llama3:latest
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	cfg, err := Load(configPath)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "telegram bot token is required")
	assert.Nil(t, cfg)
}

func TestLoad_MissingAPIKey(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
database:
  path: test.db
telegram:
  bot_token: test_token
genai:
  provider: openai
  mode: chat
openai:
  model: gpt-4
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	_, err = Load(configPath)

	// Assert
	assert.Error(t, err)
}

func TestLoad_MissingTemplateInCompletionMode(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
database:
  path: test.db
telegram:
  bot_token: test_token
genai:
  provider: ollama
  mode: completion
ollama:
  base_url: http://ollama-server:11434
  model: llama3:latest
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	cfg, err := Load(configPath)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template is required for completion mode")
	assert.Nil(t, cfg)
}

func TestLoad_UnsupportedProvider(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
database:
  path: test.db
telegram:
  bot_token: test_token
genai:
  provider: invalid_provider
  mode: chat
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	cfg, err := Load(configPath)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
	assert.Nil(t, cfg)
}

func TestLoad_DefaultValues(t *testing.T) {
	// Arrange
	resetViper()
	configContent := `
telegram:
  bot_token: test_token
genai:
  provider: ollama
  mode: chat
ollama:
  model: llama3:test
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Act
	cfg, err := Load(configPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "tellama.db", cfg.Database.Path)
	assert.Equal(t, 10000, cfg.Database.HistoryFetchLimit)
	assert.Equal(t, 10*time.Second, cfg.Telegram.Timeout)
	assert.False(t, cfg.Telegram.AllowUntrustedChat)
	assert.Equal(t, 10*time.Second, cfg.GenerativeAI.Timeout)
	assert.False(t, cfg.GenerativeAI.AllowConcurrent)

	ollamaCfg, ok := cfg.GenerativeAI.Config.(*genai.OllamaConfig)
	require.True(t, ok)
	assert.Equal(t, "http://localhost:11434", ollamaCfg.BaseURL)
	assert.Equal(t, "llama3:test", ollamaCfg.Model)
}
