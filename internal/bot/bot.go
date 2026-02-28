package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/m2tx/teams_sac/internal/agent"
)

type Message struct {
	ID   string `json:"id"`
	From struct {
		Application struct {
			DisplayName string `json:"displayName"`
		} `json:"application"`
	} `json:"from"`
	Body struct {
		Content string `json:"content"`
	} `json:"body"`
	Replies []Message `json:"replies"`
}

type graphResponse struct {
	Value []Message `json:"value"`
}

type Bot struct {
	client    *http.Client
	teamID    string
	channelID string
	botName   string
	agent     *agent.Agent
}

func New(client *http.Client, teamID, channelID, botName string, ag *agent.Agent) *Bot {
	return &Bot{
		client:    client,
		teamID:    teamID,
		channelID: channelID,
		botName:   botName,
		agent:     ag,
	}
}

func (b *Bot) CheckAndRespond() {
	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/teams/%s/channels/%s/messages?$top=10&$expand=replies",
		b.teamID, b.channelID,
	)

	resp, err := b.client.Get(url)
	if err != nil {
		fmt.Println("❌ Error fetching messages:", err)
		return
	}
	defer resp.Body.Close()

	var data graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		fmt.Println("❌ Error decoding JSON:", err)
		return
	}

	for _, msg := range data.Value {
		if msg.From.Application.DisplayName == b.botName {
			continue
		}
		if b.alreadyReplied(msg) {
			continue
		}
		fmt.Printf("📝 New message from: %s. Content: %s\n", msg.From.Application.DisplayName, msg.Body.Content)

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
		return "Hello! I am a Go bot and I received your message in this thread.", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return b.agent.Answer(ctx, question, history)
}

func (b *Bot) sendReply(messageID, text string) {
	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/teams/%s/channels/%s/messages/%s/replies",
		b.teamID, b.channelID, messageID,
	)

	payload := map[string]interface{}{
		"body": map[string]string{
			"content": text,
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	resp, err := b.client.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Println("❌ Error posting reply:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		fmt.Printf("✅ Reply sent to thread: %s\n", messageID)
	} else {
		fmt.Printf("⚠️ Failed to reply. Status: %s\n", resp.Status)
	}
}
