package memory

import (
	"context"
	"math"
	"testing"

	"github.com/sanbuphy/go-agent/internal/testutil"
)

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if sim := cosineSimilarity(a, b); math.Abs(float64(sim-1.0)) > 1e-5 {
		t.Errorf("expected 1.0, got %f", sim)
	}
	c := []float32{0, 1, 0}
	if sim := cosineSimilarity(a, c); math.Abs(float64(sim)) > 1e-5 {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func TestL1FileMemory(t *testing.T) {
	client := testutil.NewMockClient("response")
	mem := NewL1FileMemory(client)
	ctx := context.Background()

	if err := mem.Save(ctx, "I like Go", "Go is great"); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	results, err := mem.Search(ctx, "Go", 5)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	// Mock client returns empty for fact extraction, so no entries expected
	_ = results
}

func TestKeywordScore(t *testing.T) {
	score := keywordScore("hello world", "hello there world")
	if score != 2 {
		t.Errorf("expected 2, got %d", score)
	}
	score = keywordScore("foo bar", "baz qux")
	if score != 0 {
		t.Errorf("expected 0, got %d", score)
	}
}
