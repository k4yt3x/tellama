package database

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type DatabaseManager struct {
	db *gorm.DB
}

type AllowedChat struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	ChatID    int64  `gorm:"unique"`
	ChatTitle string `gorm:"unique"`
}

type ChatOverride struct {
	ID           uint  `gorm:"primaryKey;autoIncrement"`
	ChatID       int64 `gorm:"unique"`
	ChatTitle    string
	OllamaHost   string
	Model        string
	Options      string
	SystemPrompt string
}

type Message struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Timestamp time.Time `gorm:"autoCreateTime"`
	ChatID    int64     `gorm:"index"`
	ChatTitle string
	Role      string
	UserID    int64
	Username  string
	FirstName string
	LastName  string
	Content   string
}

type GenerationRequest struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	Timestamp  time.Time `gorm:"autoCreateTime"`
	ChatID     int64     `gorm:"index"`
	ChatTitle  string
	UserID     int64
	Username   string
	Model      string
	Options    string
	Prompt     string
	OllamaHost string
}

func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.AutoMigrate(&AllowedChat{}, &ChatOverride{}, &Message{}, &GenerationRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate tables: %w", err)
	}

	return &DatabaseManager{db: db}, nil
}

func (dm *DatabaseManager) IsChatAllowed(chatID int64) bool {
	var allowedChat AllowedChat
	result := dm.db.Where("chat_id = ?", chatID).First(&allowedChat)
	return !errors.Is(result.Error, gorm.ErrRecordNotFound)
}

func (dm *DatabaseManager) GetGlobalChatOverride() (ChatOverride, error) {
	var chatOverride ChatOverride
	result := dm.db.Where("chat_id IS NULL").First(&chatOverride)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return chatOverride, result.Error
	}
	return chatOverride, nil
}

func (dm *DatabaseManager) GetChatOverride(chatID int64) (ChatOverride, error) {
	// Get the default chat override
	globalChatOverride, err := dm.GetGlobalChatOverride()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return globalChatOverride, err
	}

	// Get the chat-specific override
	var chatOverride ChatOverride
	result := dm.db.Where("chat_id = ?", chatID).First(&chatOverride)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return globalChatOverride, nil
	} else if result.Error != nil {
		return chatOverride, result.Error
	}

	// Merge non-empty fields from chatOverride into globalChatOverride
	globalChatOverride.ChatID = chatOverride.ChatID
	if chatOverride.ChatTitle != "" {
		globalChatOverride.ChatTitle = chatOverride.ChatTitle
	}
	if chatOverride.OllamaHost != "" {
		globalChatOverride.OllamaHost = chatOverride.OllamaHost
	}
	if chatOverride.Model != "" {
		globalChatOverride.Model = chatOverride.Model
	}
	if chatOverride.Options != "" {
		globalChatOverride.Options = chatOverride.Options
	}
	if chatOverride.SystemPrompt != "" {
		globalChatOverride.SystemPrompt = chatOverride.SystemPrompt
	}

	return globalChatOverride, nil
}

func (dm *DatabaseManager) SetChatOverride(
	chatID int64,
	chatTitle string,
	ollamaHost string,
	model string,
	options string,
	systemPrompt string,
) error {
	chatOverride := ChatOverride{
		ChatID: chatID,
	}

	// Prepare the map of columns to update only if the field is non-empty
	updates := map[string]interface{}{}

	if chatTitle != "" {
		chatOverride.ChatTitle = chatTitle
		updates["chat_title"] = chatTitle
	}
	if ollamaHost != "" {
		chatOverride.OllamaHost = ollamaHost
		updates["ollama_host"] = ollamaHost
	}
	if model != "" {
		chatOverride.Model = model
		updates["model"] = model
	}
	if options != "" {
		chatOverride.Options = options
		updates["options"] = options
	}
	if systemPrompt != "" {
		chatOverride.SystemPrompt = systemPrompt
		updates["system_prompt"] = systemPrompt
	}

	// Update only the non-empty fields
	return dm.db.Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "chat_id"}},
			DoUpdates: clause.Assignments(updates),
		},
	).Create(&chatOverride).Error
}

func (dm *DatabaseManager) DeleteChatOverride(chatID int64) error {
	return dm.db.Where("chat_id = ?", chatID).Delete(&ChatOverride{}).Error
}

func (dm *DatabaseManager) StoreMessage(
	chatID int64,
	chatTitle string,
	role string,
	userID int64,
	username string,
	firstName string,
	lastName string,
	messageText string,
) error {
	return dm.db.Create(&Message{
		ChatID:    chatID,
		ChatTitle: chatTitle,
		Role:      role,
		UserID:    userID,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
		Content:   messageText,
	}).Error
}

func (dm *DatabaseManager) GetMessages(chatID int64, limit int) ([]Message, error) {
	var messages []Message
	result := dm.db.Where("chat_id = ?", chatID).
		Order("id DESC").
		Limit(limit).
		Find(&messages)
	if result.Error != nil {
		return nil, result.Error
	}

	history := make([]Message, len(messages))
	for i, m := range messages {
		history[i] = Message{
			Timestamp: m.Timestamp,
			ChatID:    m.ChatID,
			ChatTitle: m.ChatTitle,
			Role:      m.Role,
			UserID:    m.UserID,
			Username:  m.Username,
			FirstName: m.FirstName,
			LastName:  m.LastName,
			Content:   m.Content,
		}
	}

	slices.Reverse(history)
	return history, nil
}

func (dm *DatabaseManager) ClearMessages(chatID int64) error {
	return dm.db.Where("chat_id = ?", chatID).Delete(&Message{}).Error
}

func (dm *DatabaseManager) StoreGenerationRequest(
	chatID int64,
	chatTitle string,
	userID int64,
	username string,
	model string,
	options string,
	prompt string,
	ollamaHost string,
) error {
	return dm.db.Create(&GenerationRequest{
		ChatID:     chatID,
		ChatTitle:  chatTitle,
		UserID:     userID,
		Username:   username,
		Model:      model,
		Options:    options,
		Prompt:     prompt,
		OllamaHost: ollamaHost,
	}).Error
}
