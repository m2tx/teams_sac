# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go bot that monitors a Microsoft Teams channel via the Microsoft Graph API and automatically replies to new messages. It uses the OAuth2 client credentials flow (app-level auth, no user login) to authenticate against Azure AD.

## Build & Run

```bash
go build -o teams_sac .
./teams_sac
```

Or run directly:

```bash
go run main.go
```

## Configuration

Configuration is read from environment variables at startup (missing vars cause a fatal error):

| Variable | Description |
|---|---|
| `AZURE_CLIENT_ID` | Azure AD app registration client ID |
| `AZURE_CLIENT_SECRET` | Azure AD app registration client secret |
| `AZURE_TENANT_ID` | Azure AD tenant ID |
| `TEAMS_TEAM_ID` | Microsoft Teams team ID |
| `TEAMS_CHANNEL_ID` | Microsoft Teams channel ID |
| `TEAMS_BOT_NAME` | Display name of the app in Teams (used to detect own replies and avoid loops) |

Required Azure AD app permissions (application, not delegated): `ChannelMessage.Read.All`, `ChannelMessage.Send`.

## Architecture

```
main.go               — entry point: env config, OAuth2 setup, polling loop
internal/bot/bot.go   — Bot struct and all Microsoft Graph API logic
```

- **`main.go`**: Reads env vars via `mustEnv` (fatals on missing), creates an OAuth2 `clientcredentials.Config`, constructs a self-refreshing `http.Client`, instantiates a `bot.Bot`, then polls every 30 seconds via `time.Ticker`.
- **`internal/bot/bot.go`**: `Bot` holds the HTTP client and channel identifiers. `CheckAndRespond` fetches the last 10 messages (with replies via `$expand=replies`), skips own messages, and calls `sendReply` for unanswered threads. `alreadyReplied` checks whether any reply has `From.Application.DisplayName == BotName`.

The `golang.org/x/oauth2/clientcredentials` client handles token acquisition and automatic renewal transparently on each HTTP request.
