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
	"github.com/k4yt3x/tellama/internal/utilities"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ollama/ollama/api"
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

type ResponseMessages struct {
	privateChatDisallowed string
	internalError         string
	serverBusy            string
}

type Tellama struct {
	historyFetchLimit   int
	genaiTimeout        time.Duration
	allowUntrustedChats bool
	ollamaHost          string
	ollamaModel         string
	ollamaOptions       map[string]interface{}
	responseMessages    ResponseMessages
	template            string
	sem                 chan struct{}
	db                  *database.DatabaseManager
	bot                 *telebot.Bot
}

func NewTellama(
	telegramToken string,
	dbPath string,
	historyFetchLimit int,
	telegramTimeout time.Duration,
	genaiTimeout time.Duration,
	allowUntrustedChats bool,
	ollamaHost string,
	ollamaModel string,
	ollamaOptions map[string]interface{},
	responseMessages ResponseMessages,
	template string,
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
		historyFetchLimit:   historyFetchLimit,
		genaiTimeout:        genaiTimeout,
		allowUntrustedChats: allowUntrustedChats,
		ollamaHost:          ollamaHost,
		ollamaModel:         ollamaModel,
		ollamaOptions:       ollamaOptions,
		responseMessages:    responseMessages,
		template:            template,
		sem:                 make(chan struct{}, 1),
		db:                  db,
		bot:                 bot,
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

	chatOverride, err := t.db.GetChatOverride(chat.ID)
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

	if err := t.db.SetChatOverride(chat.ID, chat.Title, "", "", "", prompt); err != nil {
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

	if err := t.db.DeleteChatOverride(chat.ID); err != nil {
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

	if !t.checkPermissions(chat, msg.Sender, msg) && !t.allowUntrustedChats {
		return ctx.Reply("You do not have permission to use this command.")
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

	// Verify user/group has permission to use the bot
	if !t.checkPermissions(chat, user, message) && !t.allowUntrustedChats {
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

	// Get override values for this chat
	chatOverride, err := t.db.GetChatOverride(chat.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chat override")
		return err
	}

	// Add system prompt and current message to the conversation
	messages, err = t.appendCurrentMessages(messages, chat, user, message, chatOverride)
	if err != nil {
		log.Error().Err(err).Msg("Failed to append current messages")
		return ctx.Reply(t.responseMessages.internalError)
	}

	// Generate bot's response using Ollama
	log.Info().
		Int64("chat_id", chat.ID).
		Int("message_id", message.ID).
		Msg("Generating response for message")

	answer, err := t.generateResponse(messages, chatOverride)
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

func (t *Tellama) checkPermissions(
	chat *telebot.Chat,
	user *telebot.User,
	message *telebot.Message,
) bool {
	// Log the received message
	log.Info().
		Int64("chat_id", chat.ID).
		Str("chat_title", utilities.TruncateStrToLength(chat.Title, 8)).
		Str("chat_type", string(chat.Type)).
		// Int64("sender_id", user.ID).
		Str("username", user.Username).
		// Int("message_id", message.ID).
		Str("text", message.Text).
		Msg("Received message")

	if !t.db.IsChatTrusted(chat.ID) {
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

func (t *Tellama) generateResponse(
	messages []database.Message,
	chatOverride database.ChatOverride,
) (string, error) {
	// Load the prompt template
	promptTemplate := template.Must(template.New("prompt").Parse(t.template))

	// Render the prompt to be sent to Ollama
	var prompt bytes.Buffer
	err := promptTemplate.Execute(&prompt, messages)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute prompt template")
		return "", err
	}

	ollamaHost := t.ollamaHost
	if chatOverride.OllamaHost != "" {
		ollamaHost = chatOverride.OllamaHost
	}

	ollamaHostURL, err := url.Parse(ollamaHost)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse Ollama host URL")
	}
	ollamaClient := api.NewClient(ollamaHostURL, http.DefaultClient)

	model := t.ollamaModel
	if chatOverride.Model != "" {
		model = chatOverride.Model
	}

	var options map[string]interface{}
	if chatOverride.Options != "" {
		err = json.Unmarshal([]byte(chatOverride.Options), &options)
		if err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal chat override options")
			return "", err
		}
	} else {
		options = t.ollamaOptions
	}

	// Marshal Ollama options to JSON
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal Ollama options")
	}

	// Store the generation request in the database
	currentMessage := messages[len(messages)-1]
	err = t.db.StoreGenerationRequest(
		currentMessage.ChatID,
		currentMessage.ChatTitle,
		currentMessage.UserID,
		currentMessage.Username,
		model,
		string(optionsJSON),
		prompt.String(),
		ollamaHost,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store the generation request")
	}

	// Generate response from Ollama
	var responseBuilder strings.Builder
	var generateResponse api.GenerateResponse
	err = ollamaClient.Generate(context.Background(),
		&api.GenerateRequest{
			Model:     model,
			Options:   options,
			Raw:       true,
			Prompt:    prompt.String(),
			KeepAlive: &api.Duration{Duration: -1},
		}, func(resp api.GenerateResponse) error {
			generateResponse = resp
			responseBuilder.WriteString(resp.Response)
			return nil
		})
	if err != nil {
		log.Error().Err(err).Msg("Ollama chat error")
		return "", err
	}

	response := strings.TrimSpace(responseBuilder.String())
	log.Info().
		Str("response", strings.ReplaceAll(response, "\n", "\\n")).
		Str("duration", generateResponse.TotalDuration.String()).
		Int("tokens", generateResponse.EvalCount).
		Msg("Ollama response")

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
