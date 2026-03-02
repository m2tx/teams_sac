# teams_sac

A Go bot that monitors a Microsoft Teams channel and automatically replies to new messages using Google Gemini, with optional RAG (Retrieval-Augmented Generation) support backed by local documents.

## How it works

1. Polls a Teams channel every 30 seconds via the Microsoft Graph API.
2. For each unanswered message thread, it builds a prompt enriched with:
   - Relevant document excerpts retrieved from the local RAG store (TF-IDF cosine similarity, computed in-code).
   - Prior messages in the thread (conversation history).
3. Sends the prompt to Gemini (`gemini-2.0-flash`) for generation and posts the reply back to the thread.

## Architecture

```
main.go                    — entry point: env config, OAuth2 setup, polling loop
internal/bot/bot.go        — Microsoft Graph API: fetch messages, detect replies, post replies
internal/agent/agent.go    — Gemini-powered answer generation with RAG context injection
internal/rag/store.go      — in-memory vector store: load docs, chunk, embed, cosine search
```

## Prerequisites

- Go 1.25+
- An Azure AD app registration with **application** permissions:
  - `ChannelMessage.Read.All`
  - `ChannelMessage.Send`
- A Google Cloud project with the Gemini API enabled and an API key.

## Configuration

All configuration is provided via environment variables. The bot will exit immediately if any required variable is missing.

| Variable | Required | Description |
|---|---|---|
| `AZURE_CLIENT_ID` | Yes | Azure AD app registration client ID |
| `AZURE_CLIENT_SECRET` | Yes | Azure AD app registration client secret |
| `AZURE_TENANT_ID` | Yes | Azure AD tenant ID |
| `TEAMS_TEAM_ID` | Yes | Microsoft Teams team ID |
| `TEAMS_CHANNEL_ID` | Yes | Microsoft Teams channel ID |
| `TEAMS_BOT_NAME` | Yes | Display name of the app in Teams (used to avoid reply loops) |
| `GEMINI_API_KEY` | Yes | Google Gemini API key |
| `RAG_DOCS_DIR` | No | Path to directory of `.txt`/`.md` documents for RAG (default: `./docs`) |

## RAG documents

Place `.txt` or `.md` files under `./docs` (or the path set in `RAG_DOCS_DIR`). On startup the bot will:

1. Walk the directory recursively.
2. Split each file into overlapping 500-character chunks (100-character overlap).
3. Compute TF-IDF vectors for all chunks entirely in-code (no external API).
4. At query time, compute the TF-IDF vector for the user question and retrieve the top-3 most similar chunks to inject into the Gemini prompt.

If the directory is missing or empty the bot starts without RAG and falls back to Gemini's general knowledge.

## Build & Run

```bash
go build -o teams_sac .
./teams_sac
```

Or run directly:

```bash
go run main.go
```
