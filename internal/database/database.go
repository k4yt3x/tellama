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

const defaultSystemPrompt = `{{if .CurrentTime}}current_time="{{.CurrentTime}}"
{{end}}{{if .ChatTitle}}chat_title="{{.ChatTitle}}"
{{end}}{{if .ChatType}}chat_type="{{.ChatType}}"
{{end}}
# Begin System Directives

Your name is Tellama.
You are an AI chatbot built by K4YT3X for Telegram group chats.
Your task is to help users by providing information and answering questions.
You must not engage in any harmful, illegal, or unethical conversations.
You must be polite, respectful, and helpful to all users.
You must obey laws, morals, and ethics.
You should respond in plain text.

# End System Directives`

type DatabaseManager struct {
	db *gorm.DB
}

type ChatMessage struct {
	Timestamp time.Time
	ChatID    int64
	ChatTitle string
	Role      string
	UserID    int64
	Username  string
	FirstName string
	LastName  string
	Content   string
}

type AllowedChat struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	ChatID    int64  `gorm:"unique"`
	ChatTitle string `gorm:"unique"`
}

type SystemPrompt struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	ChatID       *int64 `gorm:"unique"`
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
	Message   string
}

type GenerationRequest struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Timestamp time.Time `gorm:"autoCreateTime"`
	ChatID    int64     `gorm:"index"`
	ChatTitle string
	UserID    int64
	Username  string
	Model     string
	Options   string
	Prompt    string
}

func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.AutoMigrate(&AllowedChat{}, &SystemPrompt{}, &Message{}, &GenerationRequest{})
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

func (dm *DatabaseManager) getDefaultSystemPrompt() string {
	var systemPrompt SystemPrompt
	result := dm.db.Where("chat_id IS NULL").First(&systemPrompt)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return defaultSystemPrompt
	}
	if result.Error != nil {
		return defaultSystemPrompt
	}
	return systemPrompt.SystemPrompt
}

func (dm *DatabaseManager) GetSystemPromptForGroup(chatID int64) string {
	var systemPrompt SystemPrompt
	result := dm.db.Where("chat_id = ?", chatID).First(&systemPrompt)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return dm.getDefaultSystemPrompt()
	}
	if result.Error != nil {
		return defaultSystemPrompt
	}
	return systemPrompt.SystemPrompt
}

func (dm *DatabaseManager) SetSystemPromptForGroup(chatID int64, systemPrompt string) error {
	sp := SystemPrompt{
		ChatID:       &chatID,
		SystemPrompt: systemPrompt,
	}
	return dm.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"system_prompt"}),
	}).Create(&sp).Error
}

func (dm *DatabaseManager) DeleteSystemPromptForGroup(chatID int64) error {
	return dm.db.Where("chat_id = ?", chatID).Delete(&SystemPrompt{}).Error
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
		Message:   messageText,
	}).Error
}

func (dm *DatabaseManager) GetMessages(chatID int64, limit int) ([]ChatMessage, error) {
	var messages []Message
	result := dm.db.Where("chat_id = ?", chatID).
		Order("id DESC").
		Limit(limit).
		Find(&messages)
	if result.Error != nil {
		return nil, result.Error
	}

	history := make([]ChatMessage, len(messages))
	for i, m := range messages {
		history[i] = ChatMessage{
			Timestamp: m.Timestamp,
			ChatID:    m.ChatID,
			ChatTitle: m.ChatTitle,
			Role:      m.Role,
			UserID:    m.UserID,
			Username:  m.Username,
			FirstName: m.FirstName,
			LastName:  m.LastName,
			Content:   m.Message,
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
) error {
	return dm.db.Create(&GenerationRequest{
		ChatID:    chatID,
		ChatTitle: chatTitle,
		UserID:    userID,
		Username:  username,
		Model:     model,
		Options:   options,
		Prompt:    prompt,
	}).Error
}
