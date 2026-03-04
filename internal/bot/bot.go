package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/m2tx/teams_sac/internal/agent"
)

type Bot struct {
	teams   *TeamsClient
	botName string
	agent   *agent.Agent
}

func New(teams *TeamsClient, botName string, ag *agent.Agent) *Bot {
	return &Bot{
		teams:   teams,
		botName: botName,
		agent:   ag,
	}
}

func (b *Bot) CheckAndRespond() {
	messages, err := b.teams.fetchMessages()
	if err != nil {
		fmt.Println("❌ Error fetching messages:", err)
		return
	}

	for _, msg := range messages {
		if msg.From.Application.DisplayName == b.botName {
			continue
		}
		if b.alreadyReplied(msg) {
			continue
		}
		fmt.Printf("📝 New message from: %s. Content: %s\n", msg.From.Application.DisplayName, msg.Body.Content)

		if !needsResponse(msg.Body.Content) {
			fmt.Printf("⏭️ Skipping message (no question/problem detected)\n")
			continue
		}

		history := collectHistory(msg, b.botName)
		reply, err := b.generateReply(msg.Body.Content, history)
		if err != nil {
			fmt.Println("❌ Error generating reply:", err)
			continue
		}
		b.sendReply(msg.ID, reply)
	}
}

func (b *Bot) alreadyReplied(msg Message) bool {
	for _, reply := range msg.Replies {
		if reply.From.Application.DisplayName == b.botName {
			return true
		}
	}
	return false
}

// needsResponse returns true if the message appears to be a question or problem report.
func needsResponse(content string) bool {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "?") {
		return true
	}
	keywords := []string{
		// English
		"problem", "issue", "error", "bug", "help", "how", "why", "what",
		"when", "where", "who", "can you", "could you", "not working", "broken", "fail",
		// Portuguese
		"problema", "erro", "ajuda", "como", "por que", "porque", "qual", "quando",
		"onde", "quem", "não funciona", "falhou", "falha", "preciso", "dúvida", "duvida",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// collectHistory returns prior reply bodies in the thread, oldest first,
// excluding the bot's own messages.
func collectHistory(msg Message, botName string) []string {
	history := make([]string, 0, len(msg.Replies))
	for _, r := range msg.Replies {
		if r.From.Application.DisplayName == botName {
			continue
		}
		if r.Body.Content != "" {
			history = append(history, r.Body.Content)
		}
	}
	return history
}

// generateReply calls the agent if available, otherwise returns a fallback string.
func (b *Bot) generateReply(question string, history []string) (string, error) {
	if b.agent == nil {
		return "Hello! I am a Teams SAC bot and I received your message in this thread.", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return b.agent.Answer(ctx, question, history)
}

func (b *Bot) sendReply(messageID, text string) {
	if err := b.teams.postReply(messageID, text); err != nil {
		fmt.Printf("⚠️ Failed to reply to thread %s: %v\n", messageID, err)
		return
	}
	fmt.Printf("✅ Reply sent to thread: %s\n", messageID)
}
