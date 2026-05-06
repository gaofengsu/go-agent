package memory

import "context"

// Entry is a single memory record.
type Entry struct {
	Text      string
	Score     float32
	Timestamp string
	Metadata  map[string]any
}

// Memory defines the interface for all memory levels.
type Memory interface {
	Save(ctx context.Context, userInput, aiResponse string) error
	Search(ctx context.Context, query string, topK int) ([]Entry, error)
}
