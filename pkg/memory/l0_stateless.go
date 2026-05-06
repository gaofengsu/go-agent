package memory

import "context"

// L0Stateless is a no-op memory (baseline).
type L0Stateless struct{}

func NewL0Stateless() *L0Stateless { return &L0Stateless{} }

func (m *L0Stateless) Save(ctx context.Context, userInput, aiResponse string) error { return nil }

func (m *L0Stateless) Search(ctx context.Context, query string, topK int) ([]Entry, error) {
	return nil, nil
}
