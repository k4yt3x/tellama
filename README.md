# Tellama

Tellama is a lightweight bot that integrates LLMs with Telegram's bot API. It allows you to chat with your favorite language model in Telegram private or group chats.

<p align="center">
   <img src="https://github.com/user-attachments/assets/d573da74-79ca-463e-ad6f-eb9422a8eb36"/>
</p>

## Quick Start

### 1. Build Tellama

You can skip this step if you are using the Docker image.

Install the following dependencies:

- [Go 1.24+](https://golang.org/dl/)
- [Git](https://git-scm.com/downloads)

Use the following commands to build Tellama:

```bash
git clone https://github.com/k4yt3x/tellama.git
cd tellama
go build -ldflags="-s -w" -trimpath -o bin/tellama ./cmd/tellama
```

The built binary will be located at `bin/tellama`.

### 2. Setup Telegram Bot and LLM Backend

1. Make a copy of the `configs/tellama.yaml` configuration file and name it `tellama.yaml`.
1. Create a Telegram bot and obtain a Telegram bot token from [BotFather](@BotFather).
1. Fill in the `telegram.bot_token` section in the configuration file with the token you obtained.
1. Setup an LLM backend:
   - [Ollama](https://github.com/ollama/ollama): Install and start Ollama on your machine and pull the models you want to use.
   - [OpenAI API](https://github.com/openai/openai-go): Obtain the OpenAI API base URL and API key.
1. Set the `genai.provider` to `ollama` or `openai` based on the backend you are using.
1. Fill in the `ollama` or `openai` section in the configuration file with the appropriate values.

### 3.A: Run with Docker

The official image is hosted at [ghcr.io/k4yt3x/tellama:latest](https://github.com/k4yt3x/tellama/pkgs/container/tellama). You can run the image with the command below. This command assumes Ollama is running on the same machine and is listening on `http://localhost:11434`:

```bash
# Create an empty SQLite3 database
touch tellama.db

docker run \
  --network=host \
  -v $PWD/tellama.yaml:/data/tellama.yaml \
  -v $PWD/tellama.db:/data/tellama.db \
  ghcr.io/k4yt3x/tellama:0.3.0
```

### 3.B: Run on Bare Metal

You can also run the Tellama binary directly on your machine:

```bash
bin/tellama
```

### 4. Configuration

You will need to add a custom default system prompt. Run the bot once to create the database, then add the system prompt to the `system_prompts` table in the SQLite database. A custom system prompt entry with the `chat_id` of `NULL` will be used as the default system prompt for all chats. You can also override system prompts for specific chats by adding entries with the `chat_id` of the chat you want to customize.

```sql
INSERT INTO chat_overrides (system_prompt) VALUES ('Your name is Tellama.');
```

Here is an example for how the instructions could look:

```
{{if .CurrentTime}}current_time="{{.CurrentTime}}"
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

# End System Directives
```

## License

Tellama is licensed under [GNU AGPL version 3](https://www.gnu.org/licenses/agpl-3.0.txt).

![AGPLv3](https://www.gnu.org/graphics/agplv3-155x51.png)
