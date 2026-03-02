package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/m2tx/teams_sac/internal/rag"
)

// Agent generates replies using Gemini with optional RAG context.
type Agent struct {
	model *genai.GenerativeModel
	store *rag.Store
}

// New creates an Agent. store may be empty (graceful degradation).
func New(model *genai.GenerativeModel, store *rag.Store) *Agent {
	return &Agent{
		model: model,
		store: store,
	}
}

// Answer generates a reply to question, enriched with RAG context and thread history.
// history contains prior thread messages, oldest first (bot messages excluded).
func (a *Agent) Answer(ctx context.Context, question string, history []string) (string, error) {
	chunks, err := a.store.Search(question, 3)
	if err != nil {
		// non-fatal: proceed without RAG context
		chunks = nil
	}

	prompt := buildPrompt(question, chunks, history)

	resp, err := a.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini generate: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned empty response")
	}

	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

func buildPrompt(question string, chunks []rag.Chunk, history []string) string {
	var b strings.Builder

	b.WriteString("You are a helpful assistant integrated into a Microsoft Teams channel.\n")
	b.WriteString("Answer the user's question clearly and concisely.\n")
	b.WriteString("If the provided document excerpts are relevant, use them to inform your answer.\n")
	b.WriteString("If they are not relevant, rely on your general knowledge.\n\n")

	if len(chunks) > 0 {
		b.WriteString("## Relevant document excerpts\n")
		for i, c := range chunks {
			fmt.Fprintf(&b, "--- Excerpt %d ---\n%s\n", i+1, c.Text)
		}
		b.WriteString("\n")
	}

	if len(history) > 0 {
		b.WriteString("## Previous messages in this thread (oldest first)\n")
		for _, h := range history {
			fmt.Fprintf(&b, "- %s\n", h)
		}
		b.WriteString("\n")
	}

	b.WriteString("## User question\n")
	b.WriteString(question)
	b.WriteString("\n\n## Your answer\n")

	return b.String()
}
