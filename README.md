# crunch

Summarize daily AI coding assistant activity from Crush databases.

## Overview

`crunch` scans your filesystem for `crush.db` files (SQLite databases created by the [Crush](https://github.com/charmcli/crush) AI coding assistant), extracts user prompts for a specified date, and generates a summary using AWS Bedrock.

## Installation

```bash
go install github.com/taigrr/crunch@latest
```

Or build from source:

```bash
git clone https://github.com/taigrr/crunch
cd crunch
go build -o crunch .
```

## Usage

```bash
# Summarize activity for a specific date
crunch -date 2026-04-28

# Search a specific directory (default: home directory)
crunch -date 2026-04-28 -path ~/code

# Verbose output (shows skipped directories and found databases)
crunch -date 2026-04-28 -v
```

## Requirements

- AWS credentials configured (via environment variables, `~/.aws/credentials`, or IAM role)
- Access to AWS Bedrock with Claude Sonnet 4 enabled
- Go 1.21+ for building

## How It Works

1. Walks the filesystem from the specified path (default: home directory)
2. Skips common dependency directories (node_modules, vendor, .git, etc.)
3. Finds all `crush.db` SQLite files
4. Extracts user messages for the target date
5. Groups messages by project (inferred from file path)
6. Sends to Claude via AWS Bedrock for summarization
7. Outputs a structured summary suitable for daily journal entries

## Output Format

The output is formatted as a daily development summary with:

- Project-by-project breakdown
- Key tasks and accomplishments
- Technologies used
- Notable patterns observed

## License

MIT
