package database

import (
	"testing"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakerModels struct {
	AllowedChat       TrustedChat
	SystemPrompt      ChatOverride
	Message           Message
	GenerationRequest GenerationRequest
}

func setupTestDB(t *testing.T) *DatabaseManager {
	dbManager, err := NewDatabaseManager("file::memory:?cache=shared")
	require.NoError(t, err)
	return dbManager
}

func TestNewDatabaseManager(t *testing.T) {
	// Arrange
	dbPath := "file::memory:?cache=shared"

	// Act
	dbManager, err := NewDatabaseManager(dbPath)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, dbManager.db)

	// Verify tables exist
	err = dbManager.db.AutoMigrate(
		&TrustedChat{},
		&ChatOverride{},
		&Message{},
		&GenerationRequest{},
	)
	assert.NoError(t, err)
}

func TestIsChatAllowed(t *testing.T) {
	dbManager := setupTestDB(t)

	t.Run("Allowed chat", func(t *testing.T) {
		// Arrange
		var chat TrustedChat
		faker.FakeData(&chat)
		dbManager.db.Create(&chat)

		// Act
		allowed := dbManager.IsChatTrusted(chat.ChatID)

		// Assert
		assert.True(t, allowed)
	})

	t.Run("Not allowed chat", func(t *testing.T) {
		// Act
		allowed := dbManager.IsChatTrusted(-1) // Non-existent ID

		// Assert
		assert.False(t, allowed)
	})
}

func TestSystemPrompts(t *testing.T) {
	dbManager := setupTestDB(t)
	chatIDs, err := faker.RandomInt(-1000000, 1000000, 1)
	require.NoError(t, err)
	chatID := int64(chatIDs[0])

	t.Run("Get global default chat override", func(t *testing.T) {
		// Act
		var chatOverride ChatOverride
		chatOverride, err = dbManager.GetChatOverride(chatID)
		require.NoError(t, err)

		// Assert
		assert.Empty(t, chatOverride.SystemPrompt)
	})

	t.Run("Set and get chat override", func(t *testing.T) {
		// Arrange
		chatTitle := faker.Sentence()
		ollamaHost := faker.URL()
		model := faker.Word()
		options := faker.Paragraph()
		systemPrompt := faker.Paragraph()

		// Act
		err = dbManager.SetChatOverride(chatID, chatTitle, ollamaHost, model, options, systemPrompt)
		require.NoError(t, err)

		var chatOverride ChatOverride
		chatOverride, err = dbManager.GetChatOverride(chatID)
		require.NoError(t, err)

		// Assert
		assert.Equal(t, chatID, chatOverride.ChatID)
		assert.Equal(t, chatTitle, chatOverride.ChatTitle)
		assert.Equal(t, ollamaHost, chatOverride.OllamaHost)
		assert.Equal(t, model, chatOverride.Model)
		assert.Equal(t, options, chatOverride.Options)
		assert.Equal(t, systemPrompt, chatOverride.SystemPrompt)
	})

	t.Run("Delete chat override", func(t *testing.T) {
		// Act
		err = dbManager.DeleteChatOverride(chatID)
		require.NoError(t, err)

		var chatOverride ChatOverride
		chatOverride, err = dbManager.GetChatOverride(chatID)
		require.NoError(t, err)

		// Assert
		assert.Empty(t, chatOverride.ChatTitle)
		assert.Empty(t, chatOverride.OllamaHost)
		assert.Empty(t, chatOverride.Model)
		assert.Empty(t, chatOverride.Options)
		assert.Empty(t, chatOverride.SystemPrompt)
	})
}

func TestMessageStorage(t *testing.T) {
	dbManager := setupTestDB(t)
	chatIDs, err := faker.RandomInt(-1000000, 1000000, 1)
	require.NoError(t, err)
	chatID := int64(chatIDs[0])

	// Generate test data
	testMessage := Message{
		Timestamp: time.Now().UTC(),
		ChatID:    chatID,
		ChatTitle: faker.Word(),
		Role:      "user",
		UserID:    faker.RandomUnixTime(),
		Username:  faker.Username(),
		FirstName: faker.FirstName(),
		LastName:  faker.LastName(),
		Content:   faker.Paragraph(),
	}

	t.Run("Store message", func(t *testing.T) {
		// Act
		err = dbManager.StoreMessage(
			testMessage.ChatID,
			testMessage.ChatTitle,
			testMessage.Role,
			testMessage.UserID,
			testMessage.Username,
			testMessage.FirstName,
			testMessage.LastName,
			testMessage.Content,
		)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Retrieve messages", func(t *testing.T) {
		// Act
		var messages []Message
		messages, err = dbManager.GetMessages(chatID, 10)

		// Assert
		require.NoError(t, err)
		assert.Len(t, messages, 1)

		msg := messages[0]
		assert.Equal(t, testMessage.ChatID, msg.ChatID)
		assert.Equal(t, testMessage.Content, msg.Content)
		assert.WithinDuration(t, testMessage.Timestamp, msg.Timestamp, time.Second)
	})

	t.Run("Clear messages", func(t *testing.T) {
		// Act
		err = dbManager.ClearMessages(chatID)
		require.NoError(t, err)

		var messages []Message
		messages, err = dbManager.GetMessages(chatID, 10)
		require.NoError(t, err)

		// Assert
		assert.Empty(t, messages)
	})
}

func TestGenerationRequestStorage(t *testing.T) {
	dbManager := setupTestDB(t)

	testRequest := GenerationRequest{
		Timestamp:  time.Now().UTC(),
		ChatID:     faker.RandomUnixTime(),
		ChatTitle:  faker.Word(),
		UserID:     faker.RandomUnixTime(),
		Username:   faker.Username(),
		Model:      faker.Word(),
		Options:    faker.Paragraph(),
		Prompt:     faker.Paragraph(),
		OllamaHost: faker.URL(),
	}

	// Act
	err := dbManager.StoreGenerationRequest(
		testRequest.ChatID,
		testRequest.ChatTitle,
		testRequest.UserID,
		testRequest.Username,
		testRequest.Model,
		testRequest.Options,
		testRequest.Prompt,
		testRequest.OllamaHost,
	)

	// Assert
	require.NoError(t, err)

	// Verify storage
	var storedRequest GenerationRequest
	result := dbManager.db.Where("chat_id = ?", testRequest.ChatID).First(&storedRequest)
	require.NoError(t, result.Error)
	assert.Equal(t, testRequest.Prompt, storedRequest.Prompt)
	assert.WithinDuration(t, testRequest.Timestamp, storedRequest.Timestamp, time.Second)
}
