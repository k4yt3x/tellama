#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import datetime
import sqlite3
from typing import Any

from loguru import logger

# The default fallback instructions for the bot
DEFAULT_INSTRUCTIONS = """<instructions>
- Your name is Tellama.
- You are an AI chatbot built by @K4YT3X for Telegram group chats.
- You should not engage in any harmful, illegal, or unethical conversations.
- You should be polite, respectful, and helpful to all users.
- You should obey laws, morals, and ethics.
- Contents between `<instructions></instructions>` are instructions for you to follow.
- Contents after `<instructions></instructions>` are messages from users in the chat.
- User messages are in the format of `<nickname>: <message>`.
- Your responses should be text-only, without any tags or identifiers.
</instructions>"""


class DatabaseManager:
    """
    Handles all database operations for the Tellama bot.
    """

    def __init__(self, db_path: str = "tellama.db") -> None:
        """
        Initialize and create all necessary tables for the database.

        :param db_path: The path to the SQLite database file.
        """
        self.db_path = db_path
        self._create_tables()

    def _create_tables(self) -> None:
        """
        Create the necessary SQLite tables if they do not exist.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()

            # Tellama general settings
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS tellama_settings (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    setting TEXT,
                    value TEXT
                )
                """
            )

            # Conversation history
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS conversation_history (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    timestamp TEXT,
                    chat_id INTEGER,
                    chat_name TEXT,
                    user_id INTEGER,
                    full_name TEXT,
                    message TEXT
                )
                """
            )

            # Groups allowed to use the bot
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS allowed_groups (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    chat_id INTEGER,
                    chat_name TEXT
                )
                """
            )

            # Users allowed to use the bot
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS allowed_users (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    user_id INTEGER,
                    user_name TEXT
                )
                """
            )

            # Group-specific instructions
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS chat_instructions (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    chat_id INTEGER,
                    instructions TEXT
                )
                """
            )

    def _get_default_instructions(self) -> str:
        """
        Read the default instructions and return them as a string.

        :return: The default instructions as a string.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT instructions FROM chat_instructions WHERE chat_id IS NULL"
            )
            row = cursor.fetchone()
        if row is not None and len(row) > 0:
            return row[0]

        logger.warning("No default instructions found in the database.")
        logger.warning(
            "You should add the default instructions (chat_id == NULL) "
            "to the chat_instructions table."
        )
        return DEFAULT_INSTRUCTIONS

    def get_setting(self, setting: str) -> str | None:
        """
        Get the value of a specific setting from the tellama_settings table.

        :param setting: The name of the setting to retrieve.
        :return: The value of the setting.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT value FROM tellama_settings WHERE setting = ?",
                (setting,),
            )
            row = cursor.fetchone()
        return row[0] if row else None

    def set_setting(self, setting: str, value: str) -> None:
        """
        Set the value of a specific setting in the tellama_settings table.

        :param setting: The name of the setting to set.
        :param value: The value to set for the setting.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT OR REPLACE INTO tellama_settings (setting, value)
                VALUES (?, ?)
                """,
                (setting, value),
            )

    def is_group_allowed(self, chat_id: int) -> bool:
        """
        Check if a given chat_id is in the allowed_groups table.

        :param chat_id: The ID of the group to check.
        :return: True if the group is allowed, False otherwise.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT chat_id FROM allowed_groups WHERE chat_id = ?",
                (chat_id,),
            )
            row = cursor.fetchone()
        return row is not None

    def is_user_allowed(self, user_id: int) -> bool:
        """
        Check if a given user_id is in the allowed_users table.

        :param user_id: The ID of the user to check.
        :return: True if the user is allowed, False otherwise.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT user_id FROM allowed_users WHERE user_id = ?",
                (user_id,),
            )
            row = cursor.fetchone()
        return row is not None

    def store_message(
        self,
        chat_id: int,
        chat_name: str,
        user_id: int,
        full_name: str,
        message_text: str,
    ) -> None:
        """
        Store a single message in the conversation_history table.

        :param chat_id: The ID of the group where the message was sent.
        :param chat_name: The name of the group where the message was sent.
        :param user_id: The ID of the user who sent the message.
        :param full_name: The full name of the user who sent the message.
        :param message_text: The text of the message that was sent.
        """
        utc_time = datetime.datetime.now(datetime.timezone.utc).isoformat()
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO conversation_history
                (timestamp, chat_id, chat_name, user_id, full_name, message)
                VALUES (?, ?, ?, ?, ?, ?)
                """,
                (utc_time, chat_id, chat_name, user_id, full_name, message_text),
            )

    def get_conversation_history(
        self,
        chat_id: int,
        limit: int = 10000,
    ) -> list[dict[str, Any]]:
        """
        Fetch the most recent `limit` rows for the given group,
        then return them in ascending (chronological) order.

        :param chat_id: The ID of the group to fetch history for.
        :param limit: The maximum number of rows to fetch.
        :return: A list of dictionaries representing the conversation history.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                SELECT timestamp, chat_name, user_id, full_name, message
                FROM conversation_history
                WHERE chat_id = ?
                ORDER BY id DESC
                LIMIT ?
                """,
                (chat_id, limit),
            )
            rows = cursor.fetchall()

        # rows are returned in descending order; reverse for chronological order
        rows.reverse()

        history_block = []
        for timestamp, grp_name, uid, fname, msg in rows:
            history_block.append(
                {
                    "timestamp": timestamp,
                    "chat_id": chat_id,
                    "chat_name": grp_name,
                    "user_id": uid,
                    "full_name": fname,
                    "message": msg,
                }
            )

        return history_block

    def fetch_instructions_for_group(self, chat_id: int) -> str:
        """
        Return group-specific instructions if available;
        otherwise return the global INSTRUCTIONS.

        :param chat_id: The ID of the group to fetch instructions for.
        :return: The instructions for the given group.
        """
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT instructions FROM chat_instructions WHERE chat_id = ?",
                (chat_id,),
            )
            row = cursor.fetchone()

        return row[0] if row else self._get_default_instructions()
