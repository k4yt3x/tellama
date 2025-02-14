#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import argparse
import os
import sys

from loguru import logger

from .tellama import Tellama

LOGURU_FORMAT = (
    "<green>{time:HH:mm:ss.SSSSSS!UTC}</green> | "
    "<level>{level: <8}</level> | "
    "<level>{message}</level>"
)


def str_to_bool(value: str) -> bool:
    """
    Convert a string to a boolean.
    """
    return value.lower() in ("1", "true", "yes", "on")


def parse_args() -> argparse.Namespace:
    """
    Parse command-line arguments.

    :return: Parsed arguments
    """
    parser = argparse.ArgumentParser(prog="tellama", description="Run Tellama bot")

    # Required arguments
    parser.add_argument(
        "--telegram-bot-token",
        default=os.environ.get("TELEGRAM_BOT_TOKEN"),
        required=os.environ.get("TELEGRAM_BOT_TOKEN") is None,
        help="Telegram Bot API token",
    )
    parser.add_argument(
        "--db-path",
        default=os.environ.get("DB_PATH", "tellama.db"),
        help="Path to the SQLite database file",
    )
    parser.add_argument(
        "--history-limit",
        type=int,
        default=int(os.environ.get("HISTORY_LIMIT", 1000)),
        help="Maximum number of history messages to use in context",
    )

    # Ollama options argument group
    ollama_group = parser.add_argument_group("Ollama Options")
    ollama_group.add_argument(
        "--model",
        default=os.environ.get("OLLAMA_MODEL"),
        required=os.environ.get("OLLAMA_MODEL") is None,
        help="Ollama model to use",
    )
    ollama_group.add_argument(
        "--num-ctx",
        type=int,
        default=(
            os.environ.get("OLLAMA_NUM_CTX")
            if os.environ.get("OLLAMA_NUM_CTX")
            else None
        ),
        help="Number of context tokens",
    )
    ollama_group.add_argument(
        "--temperature",
        type=float,
        default=(
            os.environ.get("OLLAMA_TEMPERATURE")
            if os.environ.get("OLLAMA_TEMPERATURE")
            else None
        ),
        help="Sampling temperature",
    )
    ollama_group.add_argument(
        "--top-p",
        type=float,
        default=(
            os.environ.get("OLLAMA_TOP_P") if os.environ.get("OLLAMA_TOP_P") else None
        ),
        help="Top-p nucleus sampling",
    )
    ollama_group.add_argument(
        "--top-k",
        type=int,
        default=(
            os.environ.get("OLLAMA_TOP_K") if os.environ.get("OLLAMA_TOP_K") else None
        ),
        help="Top-k sampling",
    )
    ollama_group.add_argument(
        "--min-p",
        type=float,
        default=(
            os.environ.get("OLLAMA_MIN_P") if os.environ.get("OLLAMA_MIN_P") else None
        ),
        help="Minimum probability for token selection",
    )
    ollama_group.add_argument(
        "--repetition-penalty",
        type=float,
        default=(
            os.environ.get("OLLAMA_REPETITION_PENALTY")
            if os.environ.get("OLLAMA_REPETITION_PENALTY")
            else None
        ),
        help="Repetition penalty",
    )
    ollama_group.add_argument(
        "--max-new-tokens",
        type=int,
        default=(
            os.environ.get("OLLAMA_MAX_NEW_TOKENS")
            if os.environ.get("OLLAMA_MAX_NEW_TOKENS")
            else None
        ),
        help="Maximum number of new tokens",
    )
    ollama_group.add_argument(
        "--do-sample",
        action="store_true",
        default=str_to_bool(os.environ.get("OLLAMA_DO_SAMPLE", "false")),
        help="Enable sampling",
    )

    return parser.parse_args()


def main() -> int:
    """
    Tellama program entry point.

    :return: Exit code
    """
    # Set up logging
    # logging.basicConfig(level=logging.INFO)
    logger.remove()
    logger.add(sys.stderr, colorize=True, format=LOGURU_FORMAT)

    # Parse command-line arguments
    args = parse_args()

    # Dynamically build Ollama options dictionary
    ollama_options = {}
    for opt in [
        "num_ctx",
        "temperature",
        "top_p",
        "top_k",
        "min_p",
        "repetition_penalty",
        "max_new_tokens",
    ]:
        value = getattr(args, opt)
        if value is not None:
            ollama_options[opt] = value

    if args.do_sample:
        ollama_options["do_sample"] = True

    # Initialize Tellama with dynamic ollama options
    tellama = Tellama(
        telegram_token=args.telegram_bot_token,
        ollama_model=args.model,
        ollama_options=ollama_options,
        history_limit=args.history_limit,
        db_path=args.db_path,
    )

    # Run Tellama and start monitoring for messages
    return tellama.run()


if __name__ == "__main__":
    sys.exit(main())
