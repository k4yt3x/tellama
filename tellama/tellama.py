#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from loguru import logger
from ollama import AsyncClient
from telegram import Chat, Update
from telegram.ext import ApplicationBuilder, ContextTypes, MessageHandler

from .database_manager import DatabaseManager


class Tellama:
    """
    The Tellama bot application.
    """

    def __init__(
        self,
        telegram_token: str,
        ollama_model: str,
        ollama_options: dict | None = None,
        history_limit: int = 1000,
        db_path: str = "tellama.db",
    ) -> None:
        """
        Initialize the bot application with the given Telegram token,
        Ollama model, and optional database path.

        :param telegram_token: The Telegram Bot API token.
        :param ollama_model: The Ollama model to use.
        :param ollama_options: The Ollama options to use.
        :param history_limit: The maximum number of history messages to use in context.
        :param db_path: The path to the SQLite database file.
        """
        self.ollama_model = ollama_model
        self.ollama_options = ollama_options
        self.history_limit = history_limit

        # Database handler
        self.db = DatabaseManager(db_path=db_path)

        # Build Telegram application
        self.application = ApplicationBuilder().token(telegram_token).build()

        # Register a handler for all text messages
        self.application.add_handler(MessageHandler(None, self.handle_message))

    async def handle_message(
        self,
        update: Update,
        _: ContextTypes.DEFAULT_TYPE,
    ) -> None:

        # Ignore messages without text or chat
        chat = update.effective_chat
        message = update.effective_message
        if not chat or not message or not message.text:
            logger.warning("Received message with invalid chat or message text.")
            return

        # Get the user who sent the message
        from_user = message.from_user
        if not from_user:
            logger.warning("Received message without a valid user.")
            return

        # Ignore messages from the bot itself
        if from_user.id == self.application.bot.id:
            return

        logger.info(
            "Received message (group: {}; user: {}): {}".format(
                chat.id, from_user.id, message.text
            )
        )

        # Check if the message is from a private chat
        if chat.type == Chat.PRIVATE:
            chat_name = f"Private Chat with {from_user.full_name}"

            # Check if the user is allowed to chat with the bot in private
            if not self.db.is_user_allowed(from_user.id):
                logger.warning(f"Unauthorized private user: {from_user.id}")
                private_chat_disallowed_message = self.db.get_setting(
                    "private_chat_disallowed_message"
                )

                if private_chat_disallowed_message is None:
                    private_chat_disallowed_message = (
                        "Sorry, you do not have permission to chat with me."
                    )

                await message.reply_text(private_chat_disallowed_message)
                return
        else:
            chat_name = chat.title if chat.title else "Unknown Chat"
            # Check if the group is allowed
            if not self.db.is_group_allowed(chat.id):
                logger.warning(f"Unauthorized group: {chat.id}")
                return

        try:
            # Fetch prior conversation history for context
            history_messages = self.db.get_conversation_history(
                chat.id, limit=self.history_limit
            )
            messages = []
            for hm in history_messages:
                if int(hm["user_id"]) == self.application.bot.id:
                    messages.append({"role": "assistant", "content": hm["message"]})
                else:
                    messages.append(
                        {
                            "role": "user",
                            "content": f"{hm['full_name']}: {hm['message']}",
                        }
                    )

            # Store the user's message in the database
            self.db.store_message(
                chat_id=chat.id,
                chat_name=chat_name,
                user_id=from_user.id,
                full_name=from_user.full_name,
                message_text=message.text,
            )

            # Ignore messages that do not mention the bot (in group chats only)
            if chat.type != Chat.PRIVATE and not message.text.lower().startswith(
                f"@{self.application.bot.username.lower()}"
            ):
                return

            # Compose prompt with instructions + user query
            instructions = self.db.fetch_instructions_for_group(chat.id)
            prompt = f"{instructions}\n\n{from_user.full_name}: {message.text}"
            logger.info(f"Processing prompt with Ollama: {message.text}")

            response = await AsyncClient().chat(
                model=self.ollama_model,
                options=self.ollama_options,
                messages=messages + [{"role": "user", "content": prompt}],
            )

            # Extract answer text
            answer = response["message"]["content"].strip()
            logger.info(f"Ollama response: {answer.replace(chr(10), '\\n')}")

            # Remove any <think> blocks
            if "</think>" in answer:
                answer = answer.split("</think>")[-1].strip()

            # Don't reply if the answer is <skip>
            if answer == "<skip>":
                return

            # Reply to the user
            await message.reply_text(answer)

            # Get the bot's full name
            bot_user = await self.application.bot.get_me()
            bot_full_name = " ".join(
                part for part in [bot_user.first_name, bot_user.last_name] if part
            )

            # Store Ollama's response in the database
            self.db.store_message(
                chat_id=chat.id,
                chat_name=chat_name,
                user_id=self.application.bot.id,
                full_name=bot_full_name,
                message_text=answer,
            )

        except Exception as error:
            logger.exception(error)

            internal_error_message = self.db.get_setting("internal_error_message")

            if internal_error_message is None:
                internal_error_message = (
                    "An internal error occurred while processing your message."
                )

            await message.reply_text(internal_error_message)

    def run(self) -> int:
        """
        Start the bot's polling loop.
        """
        logger.info("Starting Tellama polling loop")
        self.application.run_polling()
        return 1
