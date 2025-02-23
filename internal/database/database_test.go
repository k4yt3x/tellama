package database

import (
	"testing"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakerModels struct {
	AllowedChat       AllowedChat
	SystemPrompt      SystemPrompt
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
		&AllowedChat{},
		&SystemPrompt{},
		&Message{},
		&GenerationRequest{},
	)
	assert.NoError(t, err)
}

func TestIsChatAllowed(t *testing.T) {
	dbManager := setupTestDB(t)

	t.Run("Allowed chat", func(t *testing.T) {
		// Arrange
		var chat AllowedChat
		faker.FakeData(&chat)
		dbManager.db.Create(&chat)

		// Act
		allowed := dbManager.IsChatAllowed(chat.ChatID)

		// Assert
		assert.True(t, allowed)
	})

	t.Run("Not allowed chat", func(t *testing.T) {
		// Act
		allowed := dbManager.IsChatAllowed(-1) // Non-existent ID

		// Assert
		assert.False(t, allowed)
	})
}

func TestSystemPrompts(t *testing.T) {
	dbManager := setupTestDB(t)
	chatIDs, err := faker.RandomInt(-1000000, 1000000, 1)
	require.NoError(t, err)
	chatID := int64(chatIDs[0])

	t.Run("Get default prompt", func(t *testing.T) {
		// Act
		prompt := dbManager.GetSystemPromptForGroup(chatID)

		// Assert
		assert.Equal(t, defaultSystemPrompt, prompt)
	})

	t.Run("Set and get custom prompt", func(t *testing.T) {
		// Arrange
		customPrompt := faker.Paragraph()

		// Act
		err = dbManager.SetSystemPromptForGroup(chatID, customPrompt)
		require.NoError(t, err)

		prompt := dbManager.GetSystemPromptForGroup(chatID)

		// Assert
		assert.Equal(t, customPrompt, prompt)
	})

	t.Run("Delete prompt", func(t *testing.T) {
		// Act
		err = dbManager.DeleteSystemPromptForGroup(chatID)
		require.NoError(t, err)

		prompt := dbManager.GetSystemPromptForGroup(chatID)

		// Assert
		assert.Equal(t, defaultSystemPrompt, prompt)
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
		Message:   faker.Paragraph(),
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
			testMessage.Message,
		)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Retrieve messages", func(t *testing.T) {
		// Act
		var messages []ChatMessage
		messages, err = dbManager.GetMessages(chatID, 10)

		// Assert
		require.NoError(t, err)
		assert.Len(t, messages, 1)

		msg := messages[0]
		assert.Equal(t, testMessage.ChatID, msg.ChatID)
		assert.Equal(t, testMessage.Message, msg.Content)
		assert.WithinDuration(t, testMessage.Timestamp, msg.Timestamp, time.Second)
	})

	t.Run("Clear messages", func(t *testing.T) {
		// Act
		err = dbManager.ClearMessages(chatID)
		require.NoError(t, err)

		var messages []ChatMessage
		messages, err = dbManager.GetMessages(chatID, 10)
		require.NoError(t, err)

		// Assert
		assert.Empty(t, messages)
	})
}

func TestGenerationRequestStorage(t *testing.T) {
	dbManager := setupTestDB(t)

	testRequest := GenerationRequest{
		Timestamp: time.Now().UTC(),
		ChatID:    faker.RandomUnixTime(),
		ChatTitle: faker.Word(),
		UserID:    faker.RandomUnixTime(),
		Username:  faker.Username(),
		Model:     faker.Word(),
		Options:   faker.Paragraph(),
		Prompt:    faker.Paragraph(),
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
