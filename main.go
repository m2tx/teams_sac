package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/generative-ai-go/genai"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/microsoft"
	"google.golang.org/api/option"

	"github.com/m2tx/teams_sac/internal/agent"
	"github.com/m2tx/teams_sac/internal/bot"
	"github.com/m2tx/teams_sac/internal/rag"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable not set: %s", key)
	}
	return v
}

func optEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func main() {
	ctx := context.Background()

	// Microsoft Graph OAuth2 client
	config := &clientcredentials.Config{
		ClientID:     mustEnv("AZURE_CLIENT_ID"),
		ClientSecret: mustEnv("AZURE_CLIENT_SECRET"),
		TokenURL:     microsoft.AzureADEndpoint(mustEnv("AZURE_TENANT_ID")).TokenURL,
		Scopes:       []string{"https://graph.microsoft.com/.default"},
	}
	msClient := config.Client(ctx)

	// Gemini client
	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(mustEnv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatalf("failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	genModel := geminiClient.GenerativeModel("gemini-2.0-flash")

	// RAG store (graceful degradation if docs dir is missing or empty)
	docsDir := optEnv("RAG_DOCS_DIR", "./docs")
	store, err := rag.New(docsDir)
	if err != nil {
		log.Printf("warning: RAG store init failed (%v), continuing without RAG", err)
		store = rag.EmptyStore()
	}

	ag := agent.New(genModel, store)

	b := bot.New(msClient, mustEnv("TEAMS_TEAM_ID"), mustEnv("TEAMS_CHANNEL_ID"), mustEnv("TEAMS_BOT_NAME"), ag)

	fmt.Printf("🤖 [%s] Bot started. Monitoring channel...\n", time.Now().Format("15:04:05"))

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		b.CheckAndRespond()
	}
}
