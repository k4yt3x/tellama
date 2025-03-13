package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/k4yt3x/tellama/internal/config"
	"github.com/k4yt3x/tellama/internal/database"
	"github.com/k4yt3x/tellama/internal/genai"
	"github.com/k4yt3x/tellama/internal/utilities"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"gopkg.in/telebot.v4"
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

type Tellama struct {
	historyFetchLimit    int
	genaiTimeout         time.Duration
	allowUntrustedChats  bool
	genaiProvider        genai.Provider
	genaiMode            genai.Mode
	genaiConfig          genai.ProviderConfig
	genaiTemplate        string
	genaiAllowConcurrent bool
	responseMessages     config.ResponseMessages
	sem                  chan struct{}
	dm                   *database.Manager
	bot                  *telebot.Bot
}

func NewTellama(
	telegramToken string,
	dbPath string,
	historyFetchLimit int,
	telegramTimeout time.Duration,
	genaiTimeout time.Duration,
	allowUntrustedChats bool,
	genaiProvider genai.Provider,
	genaiMode genai.Mode,
	genaiConfig genai.ProviderConfig,
	genaiTemplate string,
	genaiAllowConcurrent bool,
	responseMessages config.ResponseMessages,
) (*Tellama, error) {
	db, err := database.NewDatabaseManager(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create a new Telebot instance
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  telegramToken,
		Poller: &telebot.LongPoller{Timeout: telegramTimeout},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Telebot: %w", err)
	}

	// Create a new Tellama instance
	t := &Tellama{
		historyFetchLimit:    historyFetchLimit,
		genaiTimeout:         genaiTimeout,
		allowUntrustedChats:  allowUntrustedChats,
		genaiProvider:        genaiProvider,
		genaiMode:            genaiMode,
		genaiConfig:          genaiConfig,
		genaiTemplate:        genaiTemplate,
		genaiAllowConcurrent: genaiAllowConcurrent,
		responseMessages:     responseMessages,
		sem:                  make(chan struct{}, 1),
		dm:                   db,
		bot:                  bot,
	}

	// Initialize the semaphore with a token
	t.sem <- struct{}{}

	// Register handlers
	bot.Handle("/getsysprompt", t.getSysPrompt)
	bot.Handle("/setsysprompt", t.setSysPrompt)
	bot.Handle("/delsysprompt", t.delSysPrompt)
	bot.Handle("/getconfig", t.getConfig)
	bot.Handle("/amnesia", t.amnesia)
	bot.Handle(telebot.OnText, t.handleMessage)

	return t, nil
}

func (t *Tellama) Run() {
	log.Info().Msg("Starting Telegram bot polling loop")
	t.bot.Start()
}

func (t *Tellama) getSysPrompt(ctx telebot.Context) error {
	chat := ctx.Chat()
	msg := ctx.Message()
	if chat == nil || msg == nil {
		return nil
	}

	if !t.checkPermissions(chat, msg.Sender, msg) {
		return ctx.Reply("You do not have permission to use this command.")
	}

	chatOverride, err := t.dm.GetChatOverride(chat.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get prompt")
		return ctx.Reply("Failed to get prompt. Please check logs for details.")
	}

	if chatOverride.SystemPrompt == "" {
		return ctx.Reply("No custom system prompt set for this chat.")
	}
	return ctx.Reply(chatOverride.SystemPrompt)
}

func (t *Tellama) setSysPrompt(ctx telebot.Context) error {
	chat := ctx.Chat()
	msg := ctx.Message()
	if chat == nil || msg == nil {
		return nil
	}

	if !t.checkPermissions(chat, msg.Sender, msg) {
		return ctx.Reply("You do not have permission to use this command.")
	}

	// Split message text into command and arguments
	parts := strings.SplitN(msg.Text, " ", 2)
	if len(parts) < 2 {
		return ctx.Reply("Please provide a prompt to set.")
	}

	prompt := strings.TrimSpace(parts[1])
	if prompt == "" {
		return ctx.Reply("Please provide a non-empty prompt to set.")
	}

	if err := t.dm.SetChatOverride(chat.ID, chat.Title, "", "", "", "", prompt); err != nil {
		log.Error().Err(err).Msg("Failed to set prompt")
		return ctx.Reply("Failed to set prompt. Please check logs for details.")
	}

	log.Info().
		Int64("chat_id", chat.ID).
		Int64("user_id", msg.Sender.ID).
		Msg("Prompt set")

	return ctx.Reply("Prompt set successfully.")
}

func (t *Tellama) delSysPrompt(ctx telebot.Context) error {
	chat := ctx.Chat()
	msg := ctx.Message()
	if chat == nil || msg == nil {
		return nil
	}

	if !t.checkPermissions(chat, msg.Sender, msg) {
		return ctx.Reply("You do not have permission to use this command.")
	}

	if err := t.dm.DeleteChatOverride(chat.ID); err != nil {
		log.Error().Err(err).Msg("Failed to delete prompt")
		return ctx.Reply("Failed to delete prompt. Please check logs for details.")
	}

	log.Info().
		Int64("group_id", chat.ID).
		Int64("user_id", msg.Sender.ID).
		Msg("Prompt deleted")

	return ctx.Reply("Prompt deleted successfully.")
}

func (t *Tellama) getConfig(ctx telebot.Context) error {
	chat := ctx.Chat()
	msg := ctx.Message()
	if chat == nil || msg == nil {
		return nil
	}

	if !t.checkPermissions(chat, msg.Sender, msg) {
		return ctx.Reply("You do not have permission to use this command.")
	}

	log.Info().
		Int64("group_id", chat.ID).
		Int64("user_id", msg.Sender.ID).
		Msg("Getting configuration")

	// Get override values for this chat
	chatOverride, err := t.dm.GetChatOverride(chat.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chat override")
		return ctx.Reply(t.responseMessages.InternalError)
	}

	genaiConfig, err := t.applyChatOverride(chatOverride)
	if err != nil {
		log.Error().Err(err).Msg("Failed to apply chat override")
		return ctx.Reply(t.responseMessages.InternalError)
	}

	config := map[string]any{}

	// Marshal the config struct to JSON then unmarshal to map to get all fields
	var providerConfig map[string]any
	var configBytes []byte
	var providerName string
	var configObj any
	var ok bool

	switch t.genaiProvider {
	case genai.ProviderOllama:
		providerName = "ollama"
		configObj, ok = genaiConfig.(*genai.OllamaConfig)
	case genai.ProviderOpenAI:
		providerName = "openai"
		configObj, ok = genaiConfig.(*genai.OpenAIConfig)
	}

	if !ok || configObj == nil {
		return ctx.Reply(fmt.Sprintf("Invalid configuration type for %s", providerName))
	}

	// Marshal the config to JSON
	configBytes, err = json.Marshal(configObj)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to marshal %s configuration", providerName)
		return ctx.Reply("Failed to serialize configuration")
	}

	// Unmarshal into a map to get all fields
	err = json.Unmarshal(configBytes, &providerConfig)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to unmarshal %s configuration", providerName)
		return ctx.Reply("Failed to process configuration")
	}

	config["provider"] = providerName
	config[providerName] = providerConfig

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal configuration")
		return ctx.Reply("Failed to get configuration. Please check logs for details.")
	}

	var reply strings.Builder
	reply.WriteString("Current configuration:\n\n```json\n")
	reply.Write(jsonData)
	reply.WriteString("\n```")

	return ctx.Reply(reply.String(), telebot.ModeMarkdown)
}

func (t *Tellama) amnesia(ctx telebot.Context) error {
	chat := ctx.Chat()
	msg := ctx.Message()
	if chat == nil || msg == nil {
		return nil
	}

	if !t.checkPermissions(chat, msg.Sender, msg) && !t.allowUntrustedChats {
		return ctx.Reply("You do not have permission to use this command.")
	}

	if err := t.dm.ClearMessages(chat.ID); err != nil {
		log.Error().Err(err).Msg("Failed to clear messages")
		return ctx.Reply("Failed to clear messages. Please check logs for details.")
	}

	log.Info().
		Int64("group_id", chat.ID).
		Int64("user_id", msg.Sender.ID).
		Msg("Messages cleared")

	return ctx.Reply("All messages forgotten.")
}

func (t *Tellama) handleMessage(ctx telebot.Context) error {
	// Validate that the received message is not empty
	message := ctx.Message()
	if message == nil || message.Text == "" {
		log.Info().Msg("Received message with invalid text")
		return nil
	}

	// Get chat and user information
	chat := ctx.Chat()
	user := ctx.Sender()
	if user == nil {
		log.Info().Msg("Received message without a valid sender")
		return nil
	}

	// Verify user/group has permission to use the bot
	if !t.checkPermissions(chat, user, message) && !t.allowUntrustedChats {
		if chat.Type == telebot.ChatPrivate {
			return ctx.Reply(t.responseMessages.PrivateChatDisallowed)
		}
		return nil
	}

	// Get historical messages for the chat
	messages, err := t.dm.GetMessages(chat.ID, t.historyFetchLimit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get message history")
		return ctx.Reply(t.responseMessages.InternalError)
	}

	// Store the user's message in the database
	if err = t.storeUserMessage(chat, user, message.Text); err != nil {
		log.Error().Err(err).Msg("Failed to store user message")
		return err
	}

	// Check if this message should trigger a bot response
	if !t.shouldProcessMessage(chat, message) {
		return nil
	}

	if t.genaiAllowConcurrent {
		return t.processMessage(ctx, chat, user, message, messages)
	}

	select {
	case <-t.sem:
		defer func() { t.sem <- struct{}{} }()
		return t.processMessage(ctx, chat, user, message, messages)
	case <-time.After(t.genaiTimeout):
		log.Warn().
			Int("message_id", message.ID).
			Msg("Failed to acquire semaphore to process message")
		return ctx.Reply(t.responseMessages.ServerBusy)
	}
}

func (t *Tellama) processMessage(
	ctx telebot.Context,
	chat *telebot.Chat,
	user *telebot.User,
	message *telebot.Message,
	messages []database.Message,
) error {
	// Get override values for this chat
	chatOverride, err := t.dm.GetChatOverride(chat.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chat override")
		return err
	}

	// Add system prompt and current message to the conversation
	messages, err = t.appendCurrentMessages(messages, chat, user, message, chatOverride)
	if err != nil {
		log.Error().Err(err).Msg("Failed to append current messages")
		return ctx.Reply(t.responseMessages.InternalError)
	}

	// Generate bot's response using Ollama
	log.Info().
		Int64("chat_id", chat.ID).
		Int("message_id", message.ID).
		Msg("Generating response for message")

	genaiConfig, err := t.applyChatOverride(chatOverride)
	if err != nil {
		log.Error().Err(err).Msg("Failed to apply chat override")
		return ctx.Reply(t.responseMessages.InternalError)
	}

	genaiClient, err := genai.New(t.genaiProvider, genaiConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create generative AI client")
		return ctx.Reply(t.responseMessages.InternalError)
	}

	response, err := t.generateResponse(messages, genaiClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate response")
		return ctx.Reply(t.responseMessages.InternalError)
	}

	if response == "" {
		log.Warn().Msg("Received empty response from generative AI")
		return nil
	}

	// Send the response back to the chat
	_, err = ctx.Bot().Reply(message, response, telebot.ModeMarkdown)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send reply with Markdown formatting")

		// Retry sending the response without Markdown formatting
		_, err = ctx.Bot().Reply(message, response)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send reply")
			return err
		}
	}

	// Store the bot's response in the database
	return t.storeBotResponse(chat, response)
}

func (t *Tellama) checkPermissions(
	chat *telebot.Chat,
	user *telebot.User,
	message *telebot.Message,
) bool {
	// Log the received message
	log.Info().
		Int64("chat_id", chat.ID).
		Str("chat_title", utilities.TruncateStrToLength(chat.Title, 12)).
		Str("chat_type", string(chat.Type)).
		// Int64("sender_id", user.ID).
		Str("username", user.Username).
		// Int("message_id", message.ID).
		Str("text", message.Text).
		Msg("Received message")

	if !t.dm.IsChatTrusted(chat.ID) {
		log.Warn().
			Int64("chat_id", chat.ID).
			Str("chat_title", chat.Title).
			Int("message_id", message.ID).
			Msg("Untrusted chat")
		return false
	}
	return true
}

func (t *Tellama) shouldProcessMessage(chat *telebot.Chat, msg *telebot.Message) bool {
	isReplyToBot := false
	if msg.ReplyTo != nil && msg.ReplyTo.Sender != nil {
		isReplyToBot = msg.ReplyTo.Sender.ID == t.bot.Me.ID
	}

	if chat.Type != telebot.ChatPrivate && !isReplyToBot &&
		!strings.HasPrefix(strings.ToLower(msg.Text), "@"+strings.ToLower(t.bot.Me.Username)) {
		return false
	}
	return true
}

func (t *Tellama) appendCurrentMessages(
	messages []database.Message,
	chat *telebot.Chat,
	user *telebot.User,
	msg *telebot.Message,
	chatOverride database.ChatOverride,
) ([]database.Message, error) {
	// If the message is a reply to the bot, include the original message
	isReplyToBot := msg.ReplyTo != nil && msg.ReplyTo.Sender != nil &&
		msg.ReplyTo.Sender.ID == t.bot.Me.ID

	// Construct the chat title
	title := chat.Title
	if chat.Type == telebot.ChatPrivate {
		title = user.FirstName
		if user.LastName != "" {
			title += " " + user.LastName
		}
	}

	systemPromptTemplateString := defaultSystemPrompt
	if chatOverride.SystemPrompt != "" {
		systemPromptTemplateString = chatOverride.SystemPrompt
	}

	// Add system prompt
	systemPromptTemplate := template.Must(
		template.New("sysprompt").Parse(systemPromptTemplateString),
	)

	// Inject context information into the system prompt template
	contextInfo := map[string]any{
		"CurrentTime": time.Now().UTC().Format("Monday, January 2, 2006, 15:04:05 MST"),
		"ChatTitle":   title,
		"ChatType":    chat.Type,
	}

	// Include the reply message in the context if the message is a reply to the bot
	if isReplyToBot {
		contextInfo["ReplyMessage"] = utilities.TruncateStrToLength(msg.ReplyTo.Text, 20)
	}

	var systemPrompt bytes.Buffer
	err := systemPromptTemplate.Execute(&systemPrompt, contextInfo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute system prompt template")
		return nil, err
	}

	return append(messages, database.Message{
		Timestamp: time.Now().UTC(),
		ChatID:    chat.ID,
		ChatTitle: title,
		Role:      "system",
		UserID:    t.bot.Me.ID,
		Username:  t.bot.Me.Username,
		FirstName: "system",
		Content:   systemPrompt.String(),
	}, database.Message{
		Timestamp: time.Now().UTC(),
		ChatID:    chat.ID,
		ChatTitle: title,
		Role:      "user",
		UserID:    user.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Content:   msg.Text,
	}), nil
}

func (t *Tellama) applyChatOverride(
	chatOverride database.ChatOverride,
) (genai.ProviderConfig, error) {
	// Make a copy of the generative AI configuration
	genaiConfig := t.genaiConfig

	// Apply chat override values
	switch t.genaiProvider {
	case genai.ProviderOllama:
		ollamaConfig, ok := genaiConfig.(*genai.OllamaConfig)
		if !ok {
			return nil, errors.New("invalid config type for Ollama")
		}
		if chatOverride.BaseURL != "" {
			ollamaConfig.BaseURL = chatOverride.BaseURL
		}
		if chatOverride.Model != "" {
			ollamaConfig.Model = chatOverride.Model
		}
		if chatOverride.Options != "" {
			err := json.Unmarshal([]byte(chatOverride.Options), &ollamaConfig.Options)
			if err != nil {
				log.Error().Err(err).Msg("Failed to unmarshal chat override options")
				return nil, err
			}
		}
	case genai.ProviderOpenAI:
		openaiConfig, ok := genaiConfig.(*genai.OpenAIConfig)
		if !ok {
			return nil, errors.New("invalid config type for OpenAI")
		}
		if chatOverride.BaseURL != "" {
			openaiConfig.BaseURL = chatOverride.BaseURL
		}
		if chatOverride.APIKey != "" {
			openaiConfig.APIKey = chatOverride.APIKey
		}
		if chatOverride.Model != "" {
			openaiConfig.Model = chatOverride.Model
		}
	}

	return genaiConfig, nil
}

func (t *Tellama) generateResponse(
	messages []database.Message,
	genaiClient genai.GenerativeAI,
) (string, error) {
	var response string
	var genStats genai.GenerateStats
	var err error

	switch t.genaiMode {
	case genai.ModeChat:
		genaiMessages := make([]genai.Message, len(messages))
		for i, message := range messages {
			genaiMessages[i] = genai.Message{
				Role:    message.Role,
				Content: message.Content,
			}
		}

		// Use the generative AI to chat with the user
		response, genStats, err = genaiClient.Chat(genaiMessages)
		if err != nil {
			log.Error().Err(err).Msg("Generative AI completion error")
			return "", err
		}
	case genai.ModeCompletion:
		// Load the prompt template
		promptTemplate := template.Must(template.New("prompt").Parse(t.genaiTemplate))

		// Render the prompt to be sent to the generative AI
		var prompt bytes.Buffer
		err = promptTemplate.Execute(&prompt, messages)
		if err != nil {
			log.Error().Err(err).Msg("Failed to execute prompt template")
			return "", err
		}

		// Use the generative AI to complete the prompt
		response, genStats, err = genaiClient.Complete(prompt.String())
		if err != nil {
			log.Error().Err(err).Msg("Generative AI completion error")
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported Generative AI mode: %s", t.genaiMode)
	}

	response = strings.TrimSpace(response)
	log.Info().
		Str("response", strings.ReplaceAll(response, "\n", "\\n")).
		Str("duration", genStats.TotalDuration.String()).
		Int64("tokens", genStats.TokenCount).
		Msg("Generative AI response")

	// Remove reasoning content
	if idx := strings.Index(response, "</think>"); idx != -1 {
		response = strings.TrimSpace(response[idx+len("</think>"):])
	}
	return response, nil
}

func (t *Tellama) storeUserMessage(
	chat *telebot.Chat,
	user *telebot.User,
	text string,
) error {
	err := t.dm.StoreMessage(
		chat.ID,
		chat.Title,
		"user",
		user.ID,
		user.Username,
		user.FirstName,
		user.LastName,
		text,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store user message")
	}
	return err
}

func (t *Tellama) storeBotResponse(chat *telebot.Chat, answer string) error {
	err := t.dm.StoreMessage(
		chat.ID,
		chat.Title,
		"assistant",
		t.bot.Me.ID,
		t.bot.Me.Username,
		t.bot.Me.FirstName,
		t.bot.Me.LastName,
		answer,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store bot response")
	}
	return err
}
