package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/k4yt3x/tellama/internal/database"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ollama/ollama/api"
	"github.com/rs/zerolog/log"
	"gopkg.in/telebot.v4"
)

type ResponseMessages struct {
	privateChatDisallowed string
	internalError         string
	serverBusy            string
}

type Tellama struct {
	template          string
	historyFetchLimit int
	responseMessages  ResponseMessages
	genaiTimeout      time.Duration
	ollamaModel       string
	ollamaOptions     map[string]interface{}
	sem               chan struct{}
	db                *database.DatabaseManager
	bot               *telebot.Bot
	ollamaClient      *api.Client
}

func NewTellama(
	template string,
	historyFetchLimit int,
	dbPath string,
	responseMessages ResponseMessages,
	telegramTimeout time.Duration,
	genaiTimeout time.Duration,
	telegramToken string,
	ollamaHost string,
	ollamaModel string,
	ollamaOptions map[string]interface{},
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

	ollamaBaseURL, err := url.Parse(ollamaHost)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Ollama host URL: %w", err)
	}

	// Create a new Tellama instance
	t := &Tellama{
		template:          template,
		historyFetchLimit: historyFetchLimit,
		responseMessages:  responseMessages,
		genaiTimeout:      genaiTimeout,
		ollamaModel:       ollamaModel,
		ollamaOptions:     ollamaOptions,
		sem:               make(chan struct{}, 1),
		db:                db,
		bot:               bot,
		ollamaClient:      api.NewClient(ollamaBaseURL, http.DefaultClient),
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

	systemPrompt := t.db.GetSystemPromptForGroup(chat.ID)
	if systemPrompt == "" {
		systemPrompt = "No custom system prompt available for this group."
	}

	return ctx.Reply(systemPrompt)
}

func (t *Tellama) setSysPrompt(ctx telebot.Context) error {
	chat := ctx.Chat()
	msg := ctx.Message()
	if chat == nil || msg == nil {
		return nil
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

	if err := t.db.SetSystemPromptForGroup(chat.ID, prompt); err != nil {
		log.Error().Err(err).Msg("Failed to set prompt")
		return ctx.Reply("Failed to set prompt. Please check logs for details.")
	}

	log.Info().
		Int64("group_id", chat.ID).
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

	if err := t.db.DeleteSystemPromptForGroup(chat.ID); err != nil {
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

	log.Info().
		Int64("group_id", chat.ID).
		Int64("user_id", msg.Sender.ID).
		Msg("Getting configuration")

	config := map[string]interface{}{
		"model":         t.ollamaModel,
		"options":       t.ollamaOptions,
		"history_limit": t.historyFetchLimit,
		// "template":      t.template,
	}

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal configuration")
		return ctx.Reply("Failed to get configuration. Please check logs for details.")
	}

	var reply strings.Builder
	reply.WriteString("Current configuration:\n\n```json")
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

	if err := t.db.ClearMessages(chat.ID); err != nil {
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
	select {
	case <-t.sem:
		defer func() { t.sem <- struct{}{} }()
		return t.processMessage(ctx)
	case <-time.After(t.genaiTimeout):
		message := ctx.Message()
		log.Warn().
			Int("message_id", message.ID).
			Msg("Failed to acquire semaphore to process message")
		return ctx.Reply(t.responseMessages.serverBusy)
	}
}

func (t *Tellama) processMessage(ctx telebot.Context) error {
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

	// Log the received message
	log.Info().
		Int64("chat_id", chat.ID).
		Str("chat_title", chat.Title).
		Str("chat_type", string(chat.Type)).
		Int64("sender_id", user.ID).
		Str("username", user.Username).
		Int("message_id", message.ID).
		Str("text", message.Text).
		Msg("Received message")

	// Verify user/group has permission to use the bot
	if !t.db.IsChatAllowed(chat.ID) {
		log.Warn().
			Int64("chat_id", chat.ID).
			Str("chat_title", chat.Title).
			Int("message_id", message.ID).
			Msg("Unauthorized chat")

		if chat.Type == telebot.ChatPrivate {
			return ctx.Reply(t.responseMessages.privateChatDisallowed)
		}
		return nil
	}

	// Get historical messages for the chat
	messages, err := t.db.GetMessages(chat.ID, t.historyFetchLimit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get message history")
		return ctx.Reply(t.responseMessages.internalError)
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

	// Add system prompt and current message to the conversation
	messages, err = t.appendCurrentMessages(messages, chat, user, message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to append current messages")
		return ctx.Reply(t.responseMessages.internalError)
	}

	// Generate bot's response using Ollama
	log.Info().
		Int64("chat_id", chat.ID).
		Int("message_id", message.ID).
		Msg("Generating response for message")

	answer, err := t.generateResponse(messages)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate response")
		return ctx.Reply(t.responseMessages.internalError)
	}

	// Skip responding if the model indicates to do so
	if answer == "<skip>" {
		return nil
	}

	// Send the response back to the chat
	_, err = ctx.Bot().Reply(message, answer, telebot.ModeMarkdown)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send reply with Markdown formatting")

		// Retry sending the response without Markdown formatting
		_, err = ctx.Bot().Reply(message, answer)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send reply")
			return err
		}
	}

	// Store the bot's response in the database
	return t.storeBotResponse(chat, answer)
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
	messages []database.ChatMessage,
	chat *telebot.Chat,
	user *telebot.User,
	msg *telebot.Message,
) ([]database.ChatMessage, error) {
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

	// Add system prompt
	sysPromptTemplate := template.Must(
		template.New("sysprompt").Parse(t.db.GetSystemPromptForGroup(chat.ID)),
	)

	// Inject context information into the system prompt template
	contextInfo := map[string]interface{}{
		"CurrentTime": time.Now().UTC().Format(time.RFC3339),
		"ChatTitle":   title,
		"ChatType":    chat.Type,
	}

	// Include the reply message in the context if the message is a reply to the bot
	if isReplyToBot {
		contextInfo["ReplyMessage"] = msg.ReplyTo.Text
	}

	var systemPrompt bytes.Buffer
	err := sysPromptTemplate.Execute(&systemPrompt, contextInfo)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute system prompt template")
		return nil, err
	}

	return append(messages, database.ChatMessage{
		Timestamp: time.Now().UTC(),
		ChatID:    chat.ID,
		ChatTitle: title,
		Role:      "system",
		UserID:    t.bot.Me.ID,
		Username:  t.bot.Me.Username,
		FirstName: "system",
		Content:   systemPrompt.String(),
	}, database.ChatMessage{
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

func (t *Tellama) generateResponse(messages []database.ChatMessage) (string, error) {
	// Load the prompt template
	promptTemplate := template.Must(template.New("prompt").Parse(t.template))

	// Render the prompt to be sent to Ollama
	var prompt bytes.Buffer
	err := promptTemplate.Execute(&prompt, messages)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute prompt template")
		return "", err
	}

	optionsJSON, err := json.Marshal(t.ollamaOptions)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal Ollama options")
	}

	currentMessage := messages[len(messages)-1]
	err = t.db.StoreGenerationRequest(
		currentMessage.ChatID,
		currentMessage.ChatTitle,
		currentMessage.UserID,
		currentMessage.Username,
		t.ollamaModel,
		string(optionsJSON),
		prompt.String(),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store the generation request")
	}

	// Generate response from Ollama
	var responseBuilder strings.Builder
	err = t.ollamaClient.Generate(context.Background(),
		&api.GenerateRequest{
			Model:     t.ollamaModel,
			Options:   t.ollamaOptions,
			Raw:       true,
			Prompt:    prompt.String(),
			KeepAlive: &api.Duration{Duration: -1},
		}, func(resp api.GenerateResponse) error {
			responseBuilder.WriteString(resp.Response)
			return nil
		})
	if err != nil {
		log.Error().Err(err).Msg("Ollama chat error")
		return "", err
	}

	answer := strings.TrimSpace(responseBuilder.String())
	log.Info().
		Str("text", strings.ReplaceAll(answer, "\n", "\\n")).
		Msg("Ollama response")

	// Clean up response
	if idx := strings.Index(answer, "</think>"); idx != -1 {
		answer = strings.TrimSpace(answer[idx+len("</think>"):])
	}
	answer = strings.ReplaceAll(answer, "<|start_header_id|>assistant<|end_header_id|>", "")

	return answer, nil
}

func (t *Tellama) storeUserMessage(
	chat *telebot.Chat,
	user *telebot.User,
	text string,
) error {
	err := t.db.StoreMessage(
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
	err := t.db.StoreMessage(
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
