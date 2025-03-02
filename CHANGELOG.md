# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Support for OpenAI API as generative AI backend.

## [0.2.0] - 2025-02-25

### Added

- A configuration file that manages settings for both the Telegram bot and the language model.
- Tests for the database manager module.
- The `/amnesia` command to clear the context of the current chat.
- The `/getconfig` command to retrieve the current configuration.
- The `/getsysprompt`, `/setsysprompt`, and `/delsysprompt` commands to manage custom prompts.
- The feature to inject context information into the system prompt.
- The feature to log generated prompts to the database.
- The feature to override Ollama host, model, and options for a specific chat.
- The feature to remove ChatML headers from the output.
- The feature to use custom templates to format prompts.

### Changed

- Fields and structures of several database tables.
- Rewrite the project in Go for better performance and robustness.

## [0.1.0] - 2025-02-13

### Added

- Basic Ollama and Telegram bot API integration.
- The feature to set custom special reply texts.
- The feature to store context in a SQLite database.
- The feature to use different instructions in different chats.
- The feature to whitelist groups and users.
