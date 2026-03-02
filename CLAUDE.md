# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go bot that monitors a Microsoft Teams channel via the Microsoft Graph API and automatically replies to new messages using Google Gemini (`gemini-2.0-flash`). It uses the OAuth2 client credentials flow (app-level auth, no user login) to authenticate against Azure AD. Replies are enriched with context retrieved from local documents via an in-memory RAG (Retrieval-Augmented Generation) store backed by Gemini embeddings (`text-embedding-004`).

## Build & Run

```bash
go build -o teams_sac .
./teams_sac
```

Or run directly:

```bash
go run main.go
```

Requires Go 1.25+.

## Configuration

Configuration is read from environment variables at startup (missing required vars cause a fatal error):

| Variable | Required | Description |
|---|---|---|
| `AZURE_CLIENT_ID` | Yes | Azure AD app registration client ID |
| `AZURE_CLIENT_SECRET` | Yes | Azure AD app registration client secret |
| `AZURE_TENANT_ID` | Yes | Azure AD tenant ID |
| `TEAMS_TEAM_ID` | Yes | Microsoft Teams team ID |
| `TEAMS_CHANNEL_ID` | Yes | Microsoft Teams channel ID |
| `TEAMS_BOT_NAME` | Yes | Display name of the app in Teams (used to detect own replies and avoid loops) |
| `GEMINI_API_KEY` | Yes | Google Gemini API key |
| `RAG_DOCS_DIR` | No | Path to directory of `.txt`/`.md` documents for RAG (default: `./docs`) |

Required Azure AD app permissions (application, not delegated): `ChannelMessage.Read.All`, `ChannelMessage.Send`.

## Architecture

```
main.go                    — entry point: env config, OAuth2 + Gemini setup, RAG init, polling loop
internal/bot/bot.go        — Bot struct: fetch messages, detect replies, collect thread history, post replies
internal/agent/agent.go    — Gemini-powered answer generation with RAG context and thread history injection
internal/rag/store.go      — in-memory vector store: load docs, chunk, embed (batch), cosine similarity search
```

### `main.go`
Reads env vars via `mustEnv` (fatals on missing) and `optEnv` (returns default). Creates:
- An OAuth2 `clientcredentials.Config` → self-refreshing `http.Client` for Graph API calls.
- A `genai.Client` → `GenerativeModel("gemini-2.0-flash")` and `EmbeddingModel("text-embedding-004")`.
- A `rag.Store` (gracefully degrades to an empty store if `RAG_DOCS_DIR` is missing or empty).
- An `agent.Agent` wiring the models and store together.
- A `bot.Bot`, then polls every 30 seconds via `time.Ticker`.

### `internal/bot/bot.go`
`Bot` holds the Graph API HTTP client, channel identifiers, and an `*agent.Agent`. `CheckAndRespond` fetches the last 10 top-level messages (`$top=10&$expand=replies`), skips messages sent by the bot itself, and for each unanswered thread calls `generateReply` then `sendReply`. `alreadyReplied` checks whether any reply has `From.Application.DisplayName == BotName`. `collectHistory` gathers non-bot reply bodies (oldest first) to pass as conversation context.

### `internal/agent/agent.go`
`Agent.Answer` embeds the user question, calls `store.Search` to retrieve the top-3 most relevant chunks, then builds a structured prompt (system instructions → document excerpts → thread history → user question) and sends it to Gemini. RAG failures are non-fatal; the prompt is sent without context.

### `internal/rag/store.go`
`New` walks `docsDir` recursively, reads all `.txt`/`.md` files, splits them into overlapping 500-rune chunks (100-rune overlap), and batch-embeds them via `text-embedding-004`. `Search` embeds the query and returns the top-k chunks by cosine similarity. Returns an empty store (no error) when the directory is absent or contains no supported files.
