<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://github.com/taigrr/crunch/raw/master/docs/crunch-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="https://github.com/taigrr/crunch/raw/master/docs/crunch-light.svg">
  <img alt="crunch" src="https://github.com/taigrr/crunch/raw/master/docs/crunch-dark.svg" width="400">
</picture>

**Crunch your daily AI coding activity into a summary.**

[![Test](https://github.com/taigrr/crunch/actions/workflows/test.yml/badge.svg)](https://github.com/taigrr/crunch/actions/workflows/test.yml)
[![Lint](https://github.com/taigrr/crunch/actions/workflows/lint.yml/badge.svg)](https://github.com/taigrr/crunch/actions/workflows/lint.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/taigrr/crunch.svg)](https://pkg.go.dev/github.com/taigrr/crunch)
[![License: 0BSD](https://img.shields.io/badge/License-0BSD-blue.svg)](LICENSE)

## The Problem

You use [Crush](https://github.com/taigrr/crush) all day, every day.
At the end of the week, you need to write status updates, fill out timesheets, or just remember what you worked on.
But the context is scattered across dozens of project directories in `crush.db` files.

## The Solution

**Crunch** scans your filesystem for `crush.db` files, extracts your prompts for a given date, and generates a structured summary using your preferred LLM.

```bash
# What did I work on today?
crunch

# What about last Tuesday?
crunch -d 2026-04-22
```

## ✨ Features

- 🔍 **Automatic discovery** — Recursively finds all `crush.db` files
- 📅 **Date filtering** — Summarize any day's activity
- 🏷️ **Project grouping** — Messages organized by inferred project
- 🌊 **Streaming output** — Real-time progress with cost estimation
- 🔌 **Multi-provider** — AWS Bedrock, Anthropic, OpenAI, OpenRouter
- ⚡ **Fast scanning** — Skips `node_modules`, `.git`, `vendor`, etc.

## 📦 Installation

```bash
go install github.com/taigrr/crunch@latest
```

Or build from source:

```bash
git clone https://github.com/taigrr/crunch
cd crunch
go build
```

## ⚡ Requirements

- Go >= **1.26.5** (for installation)
- One of:
  - AWS credentials (for Bedrock)
  - `ANTHROPIC_API_KEY`
  - `OPENAI_API_KEY`
  - `OPENROUTER_API_KEY`

## 🚀 Quick Start

```bash
# Summarize today's activity (auto-detects provider from env)
crunch

# Summarize a specific date
crunch -d 2026-04-28

# Search only in a specific directory
crunch -p ~/code

# Use a specific provider
crunch --provider anthropic

# Use a specific model
crunch --provider openrouter -m anthropic/claude-sonnet-4
```

## 📖 Flags

| Flag         | Short | Description                                           |
| ------------ | ----- | ----------------------------------------------------- |
| `--date`     | `-d`  | Date to summarize (YYYY-MM-DD, default: today)        |
| `--path`     | `-p`  | Path to search (default: home directory)              |
| `--dir`      |       | Base directory to strip from project paths            |
| `--provider` |       | LLM provider (bedrock, anthropic, openai, openrouter) |
| `--model`    | `-m`  | Model to use (provider-specific)                      |
| `--api-key`  |       | API key (overrides env variable)                      |
| `--verbose`  | `-v`  | Verbose output                                        |
| `--help`     | `-h`  | Show help                                             |

## 🔌 Providers

| Provider     | Env Variable         | Default Model                                |
| ------------ | -------------------- | -------------------------------------------- |
| `bedrock`    | AWS credentials      | `us.anthropic.claude-sonnet-4-20250514-v1:0` |
| `anthropic`  | `ANTHROPIC_API_KEY`  | `claude-sonnet-4-20250514`                   |
| `openai`     | `OPENAI_API_KEY`     | `gpt-4o`                                     |
| `openrouter` | `OPENROUTER_API_KEY` | `anthropic/claude-sonnet-4`                  |

Provider is auto-detected from environment variables. If multiple are set, priority is: Anthropic > OpenAI > OpenRouter > Bedrock.

## ⚙️ Environment Variables

```bash
# Provider-specific API keys
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export OPENROUTER_API_KEY="sk-or-..."

# Override defaults with CRUNCH_ prefix
export CRUNCH_PROVIDER="anthropic"
export CRUNCH_MODEL="claude-sonnet-4-20250514"
export CRUNCH_API_KEY="sk-ant-..."
```

## 🔧 How It Works

1. **Scan** — Walks your filesystem looking for `crush.db` files
2. **Skip** — Ignores common dependency directories (node_modules, vendor, .git, etc.)
3. **Extract** — Reads user messages from SQLite for the target date
4. **Group** — Organizes messages by project (inferred from file path)
5. **Summarize** — Sends grouped prompts to your LLM with streaming output
6. **Display** — Shows real-time progress with token/cost estimation

## 📋 Output

The summary is structured for easy scanning:

- Project-by-project breakdown
- Key tasks and accomplishments
- Technologies used
- Notable patterns (debugging, feature work, refactoring)

Perfect for:

- Daily standups
- Weekly status reports
- Timesheet entries
- Personal dev journals

## 🤝 Related Projects

- [Crush](https://github.com/taigrr/crush) — The AI coding assistant that creates the databases
- [Fantasy](https://github.com/taigrr/fantasy) — The multi-provider LLM library powering crunch

## 📄 License

[0BSD](LICENSE) © [Tai Groot](https://github.com/taigrr)
