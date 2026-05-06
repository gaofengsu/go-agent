package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

const l1File = "memory_l1.jsonl"

// L1FileMemory uses JSONL + keyword matching.
type L1FileMemory struct {
	Client llm.LLMClient
	Model  string
}

func NewL1FileMemory(client llm.LLMClient) *L1FileMemory {
	return &L1FileMemory{Client: client, Model: os.Getenv("OPENAI_MODEL")}
}

func (m *L1FileMemory) Save(ctx context.Context, userInput, aiResponse string) error {
	if aiResponse == "" {
		return nil
	}
	facts, err := m.extractFacts(ctx, userInput, aiResponse)
	if err != nil {
		return err
	}
	for _, f := range facts {
		entry := map[string]any{
			"text":      f,
			"source":    "conversation",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		if err := appendJSONL(l1File, entry); err != nil {
			return err
		}
		fmt.Printf("[Memory] Saved: %s\n", f)
	}
	return nil
}

func keywordScore(query, text string) int {
	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
		return 0
	}
	words := make(map[string]struct{})
	for _, w := range strings.Fields(strings.ToLower(text)) {
		words[w] = struct{}{}
	}
	score := 0
	for _, k := range keywords {
		if _, ok := words[k]; ok {
			score++
		}
	}
	return score
}

func (m *L1FileMemory) Search(ctx context.Context, query string, topK int) ([]Entry, error) {
	memories, err := loadJSONL(l1File)
	if err != nil {
		return nil, err
	}
	type scored struct {
		entry map[string]any
		score int
	}
	var scoredList []scored
	for _, mem := range memories {
		text, _ := mem["text"].(string)
		score := keywordScore(query, text)
		if score > 0 {
			scoredList = append(scoredList, scored{entry: mem, score: score})
		}
	}
	// simple bubble sort by score desc
	for i := 0; i < len(scoredList); i++ {
		for j := i + 1; j < len(scoredList); j++ {
			if scoredList[j].score > scoredList[i].score {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}
	var results []Entry
	for i, s := range scoredList {
		if i >= topK {
			break
		}
		results = append(results, mapToEntry(s.entry))
	}
	return results, nil
}

func (m *L1FileMemory) extractFacts(ctx context.Context, userInput, aiResponse string) ([]string, error) {
	if m.Client == nil {
		return nil, nil
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Extract key facts worth remembering long-term from this conversation.
Ignore greetings, small talk, and opinions. Only extract facts, preferences, and decisions.

User: %s
AI: %s

Return a JSON array of strings. Example: ["Alice likes Python", "Project uses React 18"]
If nothing is worth remembering, return [].`, userInput, aiResponse)},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return nil, err
	}
	var facts []string
	if err := json.Unmarshal([]byte(resp.Content), &facts); err != nil {
		return nil, nil
	}
	return facts, nil
}

func appendJSONL(path string, entry map[string]any) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(entry)
}

func loadJSONL(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err == nil {
			out = append(out, m)
		}
	}
	// keep last 200
	if len(out) > 200 {
		out = out[len(out)-200:]
	}
	return out, nil
}

func mapToEntry(m map[string]any) Entry {
	text, _ := m["text"].(string)
	ts, _ := m["timestamp"].(string)
	return Entry{Text: text, Timestamp: ts, Metadata: m}
}
