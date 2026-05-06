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

const (
	l5File         = "memory_l5.jsonl"
	compressEvery  = 5
)

// L5SummaryMemory compresses conversations into summaries.
type L5SummaryMemory struct {
	Client llm.LLMClient
	Model  string
}

func NewL5SummaryMemory(client llm.LLMClient) *L5SummaryMemory {
	return &L5SummaryMemory{Client: client, Model: os.Getenv("OPENAI_MODEL")}
}

func (m *L5SummaryMemory) Save(ctx context.Context, userInput, aiResponse string) error {
	if aiResponse == "" || m.Client == nil {
		return nil
	}
	summary, err := m.extractSummary(ctx, userInput, aiResponse)
	if err != nil || summary == "" {
		return err
	}
	entry := map[string]any{
		"summary":      summary,
		"source_turns": 1,
		"timestamp":    time.Now().Format(time.RFC3339),
		"type":         "summary",
	}
	if err := appendJSONL(l5File, entry); err != nil {
		return err
	}
	fmt.Printf("[Memory] Summary saved: %s...\n", summary)
	m.compressMemories(ctx)
	return nil
}

func (m *L5SummaryMemory) Search(ctx context.Context, query string, topK int) ([]Entry, error) {
	keywords := strings.Fields(strings.ToLower(query))
	if len(keywords) == 0 {
		return nil, nil
	}
	f, err := os.Open(l5File)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	type scored struct {
		entry map[string]any
		score int
	}
	var scoredList []scored
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var mem map[string]any
		if err := json.Unmarshal(sc.Bytes(), &mem); err != nil {
			continue
		}
		summary, _ := mem["summary"].(string)
		words := make(map[string]struct{})
		for _, w := range strings.Fields(strings.ToLower(summary)) {
			words[w] = struct{}{}
		}
		score := 0
		for _, k := range keywords {
			if _, ok := words[k]; ok {
				score++
			}
		}
		if score > 0 {
			scoredList = append(scoredList, scored{entry: mem, score: score})
		}
	}
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
		summary, _ := s.entry["summary"].(string)
		results = append(results, Entry{Text: summary, Score: float32(s.score)})
	}
	return results, nil
}

func (m *L5SummaryMemory) extractSummary(ctx context.Context, userInput, aiResponse string) (string, error) {
	if m.Client == nil {
		return "", nil
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Summarize this conversation exchange into ONE sentence capturing any facts, preferences, or decisions worth remembering.
If nothing notable happened, output exactly: NONE

User: %s
AI: %s`, userInput, aiResponse)},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(resp.Content)
	if text == "NONE" {
		return "", nil
	}
	return text, nil
}

func (m *L5SummaryMemory) compressMemories(ctx context.Context) {
	f, err := os.Open(l5File)
	if err != nil {
		return
	}
	var memories []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var mem map[string]any
		if err := json.Unmarshal(sc.Bytes(), &mem); err == nil {
			memories = append(memories, mem)
		}
	}
	f.Close()
	if len(memories) < compressEvery {
		return
	}
	var texts []string
	for _, mem := range memories {
		texts = append(texts, mem["summary"].(string))
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Compress these memory summaries into ONE concise summary.
Keep all important facts, preferences, and decisions. Remove redundancy.

Summaries:
%s

Output a single paragraph summary.`, joinBullet(texts))},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return
	}
	compressed := strings.TrimSpace(resp.Content)
	totalTurns := 0
	for _, mem := range memories {
		totalTurns += int(mem["source_turns"].(float64))
	}
	entry := map[string]any{
		"summary":      compressed,
		"source_turns": totalTurns,
		"timestamp":    time.Now().Format(time.RFC3339),
		"type":         "compressed",
	}
	os.Remove(l5File)
	appendJSONL(l5File, entry)
	fmt.Printf("[Memory] Compressed %d summaries into 1\n", len(memories))
}
