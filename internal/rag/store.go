package rag

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Chunk is a piece of a source document together with its TF-IDF embedding vector.
type Chunk struct {
	Text      string
	Embedding []float32
}

// Store holds all embedded chunks in memory.
type Store struct {
	chunks []Chunk
	idf    map[string]float64
	vocab  map[string]int
}

// EmptyStore returns a Store with no chunks (used for graceful degradation).
func EmptyStore() *Store {
	return &Store{}
}

// New builds a Store by loading, chunking, and embedding (TF-IDF) all .txt and .md
// files found recursively under docsDir. If docsDir does not exist or contains no
// supported files, an empty Store is returned with no error.
func New(docsDir string) (*Store, error) {
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

	idf := computeIDF(allChunks)

	// Build a sorted, deterministic vocabulary.
	terms := make([]string, 0, len(idf))
	for t := range idf {
		terms = append(terms, t)
	}
	sort.Strings(terms)
	vocab := make(map[string]int, len(terms))
	for i, t := range terms {
		vocab[t] = i
	}

	fmt.Printf("[RAG] Indexing %d chunks (vocab size: %d)...\n", len(allChunks), len(vocab))

	store := &Store{idf: idf, vocab: vocab}
	for _, chunk := range allChunks {
		store.chunks = append(store.chunks, Chunk{
			Text:      chunk,
			Embedding: tfidfVector(chunk, idf, vocab),
		})
	}

	fmt.Printf("[RAG] Indexed %d chunks.\n", len(store.chunks))
	return store, nil
}

// Search returns the top-k chunks most similar to query by cosine similarity over TF-IDF vectors.
// Returns an empty slice if the store has no chunks.
func (s *Store) Search(query string, topK int) ([]Chunk, error) {
	if len(s.chunks) == 0 {
		return nil, nil
	}

	queryVec := tfidfVector(query, s.idf, s.vocab)

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

// tokenize lowercases text and splits on non-letter/non-digit runes.
func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// computeIDF computes smoothed IDF scores for all terms across the given chunks.
// IDF(t) = log((N+1) / (df(t)+1)) + 1
func computeIDF(chunks []string) map[string]float64 {
	df := map[string]int{}
	for _, chunk := range chunks {
		seen := map[string]bool{}
		for _, term := range tokenize(chunk) {
			if !seen[term] {
				df[term]++
				seen[term] = true
			}
		}
	}
	n := float64(len(chunks))
	idf := make(map[string]float64, len(df))
	for term, count := range df {
		idf[term] = math.Log((n+1)/(float64(count)+1)) + 1
	}
	return idf
}

// tfidfVector computes a dense TF-IDF vector for text using the provided IDF table and vocabulary.
func tfidfVector(text string, idf map[string]float64, vocab map[string]int) []float32 {
	terms := tokenize(text)
	vec := make([]float32, len(vocab))
	if len(terms) == 0 {
		return vec
	}
	tf := map[string]float64{}
	for _, t := range terms {
		tf[t]++
	}
	total := float64(len(terms))
	for term, freq := range tf {
		if idx, ok := vocab[term]; ok {
			vec[idx] = float32((freq / total) * idf[term])
		}
	}
	return vec
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
