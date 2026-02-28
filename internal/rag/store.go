package rag

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/generative-ai-go/genai"
)

// Chunk is a piece of a source document together with its embedding vector.
type Chunk struct {
	Text      string
	Embedding []float32
}

// Store holds all embedded chunks in memory.
type Store struct {
	chunks []Chunk
}

// EmptyStore returns a Store with no chunks (used for graceful degradation).
func EmptyStore() *Store {
	return &Store{}
}

// New builds a Store by loading, chunking, and embedding all .txt and .md
// files found recursively under docsDir. If docsDir does not exist or
// contains no supported files, an empty Store is returned with no error.
func New(ctx context.Context, embedModel *genai.EmbeddingModel, docsDir string) (*Store, error) {
	texts, err := loadFiles(docsDir)
	if err != nil {
		return nil, err
	}
	if len(texts) == 0 {
		return EmptyStore(), nil
	}

	var allChunks []string
	for _, text := range texts {
		allChunks = append(allChunks, chunkText(text, 500, 100)...)
	}
	if len(allChunks) == 0 {
		return EmptyStore(), nil
	}

	fmt.Printf("[RAG] Embedding %d chunks from %s...\n", len(allChunks), docsDir)

	batch := embedModel.NewBatch()
	for _, t := range allChunks {
		batch.AddContent(genai.Text(t))
	}

	resp, err := embedModel.BatchEmbedContents(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("embedding chunks: %w", err)
	}

	store := &Store{}
	for i, emb := range resp.Embeddings {
		store.chunks = append(store.chunks, Chunk{
			Text:      allChunks[i],
			Embedding: emb.Values,
		})
	}

	fmt.Printf("[RAG] Indexed %d chunks.\n", len(store.chunks))
	return store, nil
}

// Search returns the top-k chunks most similar to query by cosine similarity.
// Returns an empty slice if the store has no chunks.
func (s *Store) Search(ctx context.Context, embedModel *genai.EmbeddingModel, query string, topK int) ([]Chunk, error) {
	if len(s.chunks) == 0 {
		return nil, nil
	}

	resp, err := embedModel.EmbedContent(ctx, genai.Text(query))
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	queryVec := resp.Embedding.Values

	type scored struct {
		chunk Chunk
		score float64
	}

	scores := make([]scored, len(s.chunks))
	for i, c := range s.chunks {
		scores[i] = scored{chunk: c, score: cosineSimilarity(queryVec, c.Embedding)}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if topK > len(scores) {
		topK = len(scores)
	}

	result := make([]Chunk, topK)
	for i := range result {
		result[i] = scores[i].chunk
	}
	return result, nil
}

// loadFiles walks docsDir and returns the text content of all .txt and .md files.
func loadFiles(docsDir string) ([]string, error) {
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var texts []string
	err := filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".txt" && ext != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if len(strings.TrimSpace(string(data))) > 0 {
			texts = append(texts, string(data))
		}
		return nil
	})
	return texts, err
}

// chunkText splits text into overlapping windows of chunkSize runes with the
// given overlap. Empty chunks are dropped.
func chunkText(text string, chunkSize, overlap int) []string {
	runes := []rune(text)
	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}

	var chunks []string
	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

// cosineSimilarity computes the cosine similarity between two float32 vectors.
func cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
