# Tellama

**Tellama** is a lightweight bot that integrates LLMs with Telegram's bot API. It allows you to chat with your favorite language model in Telegram private or group chats.

> [!IMPORTANT]
> Tellama is still in early stages of development. The current version is finished in just two afternoons. You may run into bugs, unexpected behavior, or incomplete documentation. Please report any problems you encounter by [opening an issue](https://github.com/k4yt3x/tellama/issues/new).

Here is a demo of Tellama in action:

<p align="center">
   <img src="https://github.com/user-attachments/assets/d573da74-79ca-463e-ad6f-eb9422a8eb36"/>
</p>

## Quick Start

### Option 1: Run with Docker

The official image is hosted at [ghcr.io/k4yt3x/tellama:latest](https://github.com/k4yt3x/tellama/pkgs/container/tellama).

To pass options with environment variables:

```bash
docker run \
  --network=host \
  -v $PWD:/data \
  -e TELEGRAM_TOKEN="YOUR_TELEGRAM_BOT_TOKEN" \
  -e OLLAMA_MODEL="YourModelName" \
  ghcr.io/k4yt3x/tellama:latest
```

You can also save the environment variables in a `.env` file to make things easier:

```bash
docker run \
  --network=host \
  -v $PWD:/data \
  --env-file .env \
  ghcr.io/k4yt3x/tellama:latest
```

To pass options with command-line arguments:

```bash
docker run \
  --network=host \
  -v $PWD:/data \
  ghcr.io/k4yt3x/tellama:latest \
  --telegram-token="YOUR_TELEGRAM_BOT_TOKEN" \
  --model="YourModelName"
```

### Option 2: Install from PyPI

You can also install Tellama directly using pip:

```bash
pip install tellama
```

To pass options with environment variables:

```bash
export TELEGRAM_TOKEN="YOUR_TELEGRAM_BOT_TOKEN"
export OLLAMA_MODEL="YourModelName"

tellama
```

You can also save the environment variables in a `.env` file to make things easier:

```bash
set -a; source .env; set +a
tellama
```

To pass options with command-line arguments:

```bash
tellama --telegram-token="YOUR_TELEGRAM_BOT_TOKEN" --model="YourModelName"
```

## Further Configuration

To see all available options, run:

```bash
tellama --help
```

or consult the [project repository](https://github.com/k4yt3x/tellama) for more details.

## Configuration

You will need to add custom default instructions for the model. Run the bot once to create the database, then add the instructions to the `chat_instructions` table in the SQLite database. A custom instruction entry with the `chat_id` of `NULL` will be used as the default instruction for all chats. You can also add instructions for specific chats by adding entries with the `chat_id` of the chat you want to customize.

```sql
INSERT INTO chat_instructions (instructions) VALUES ('Your custom instructions');
```

Here is an example for how the instructions should look like:

```
<instructions>
- Your name is Tellama.
- You are an AI chatbot built by @JohnDoe for Telegram group chats.
- You should not engage in any harmful, illegal, or unethical conversations.
- You should be polite, respectful, and helpful to all users.
- You should obey laws, morals, and ethics.
- Contents between `<instructions></instructions>` are instructions for you to follow.
- Contents after `<instructions></instructions>` are messages from users in the chat.
- User messages are in the format of `<nickname>: <message>`.
- Your responses should be text-only, without any tags or identifiers.
</instructions>
```

## License

Tellama is licensed under [GNU AGPL version 3](https://www.gnu.org/licenses/agpl-3.0.txt).

![AGPLv3](https://www.gnu.org/graphics/agplv3-155x51.png)
