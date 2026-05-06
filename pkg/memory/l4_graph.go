package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

const l4DB = "memory_l4.db"

// L4GraphMemory stores SPO triples in SQLite.
type L4GraphMemory struct {
	Client llm.LLMClient
	Model  string
	db     *sql.DB
}

func NewL4GraphMemory(client llm.LLMClient) (*L4GraphMemory, error) {
	db, err := sql.Open("sqlite", l4DB)
	if err != nil {
		return nil, err
	}
	schema := `
CREATE TABLE IF NOT EXISTS triples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject TEXT NOT NULL,
    predicate TEXT NOT NULL,
    object TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    source TEXT DEFAULT '',
    valid_from TEXT NOT NULL,
    valid_until TEXT
);
CREATE INDEX IF NOT EXISTS idx_subject ON triples(subject);
CREATE INDEX IF NOT EXISTS idx_predicate ON triples(predicate);
CREATE INDEX IF NOT EXISTS idx_active ON triples(valid_until);
`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &L4GraphMemory{Client: client, Model: os.Getenv("OPENAI_MODEL"), db: db}, nil
}

func (m *L4GraphMemory) Save(ctx context.Context, userInput, aiResponse string) error {
	if aiResponse == "" || m.Client == nil {
		return nil
	}
	triples, err := m.extractTriples(ctx, userInput, aiResponse)
	if err != nil {
		return err
	}
	for _, t := range triples {
		if err := m.addTriple(t.Subject, t.Predicate, t.Object); err != nil {
			return err
		}
	}
	return nil
}

func (m *L4GraphMemory) Search(ctx context.Context, query string, topK int) ([]Entry, error) {
	if m.Client == nil {
		return nil, nil
	}
	// extract entities from query
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Extract entity names from this message. Return JSON array of strings or [].
Message: %s`, query)},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return nil, err
	}
	var entities []string
	json.Unmarshal([]byte(resp.Content), &entities)
	var facts []string
	for _, ent := range entities {
		rows, err := m.db.Query("SELECT subject, predicate, object FROM triples WHERE subject=? AND valid_until IS NULL", normalize(ent))
		if err != nil {
			continue
		}
		for rows.Next() {
			var s, p, o string
			rows.Scan(&s, &p, &o)
			facts = append(facts, fmt.Sprintf("%s %s %s", s, p, o))
		}
		rows.Close()
	}
	var results []Entry
	for _, f := range facts {
		results = append(results, Entry{Text: f})
	}
	return results, nil
}

func (m *L4GraphMemory) addTriple(subject, predicate, object string) error {
	sub := normalize(subject)
	pred := strings.ToLower(strings.TrimSpace(predicate))
	obj := strings.ToLower(strings.TrimSpace(object))
	now := time.Now().Format(time.RFC3339)

	// detect contradictions
	rows, err := m.db.Query("SELECT id, object FROM triples WHERE subject=? AND predicate=? AND valid_until IS NULL", sub, pred)
	if err != nil {
		return err
	}
	var contradictions []int64
	for rows.Next() {
		var id int64
		var oldObj string
		rows.Scan(&id, &oldObj)
		if oldObj != obj {
			contradictions = append(contradictions, id)
		}
	}
	rows.Close()
	for _, cid := range contradictions {
		m.db.Exec("UPDATE triples SET valid_until=? WHERE id=?", now, cid)
		fmt.Printf("[Memory] Invalidated old triple #%d\n", cid)
	}

	_, err = m.db.Exec("INSERT INTO triples (subject, predicate, object, valid_from) VALUES (?,?,?,?)", sub, pred, obj, now)
	if err != nil {
		return err
	}
	fmt.Printf("[Memory] Triple: (%s, %s, %s)\n", sub, pred, object)
	return nil
}

func (m *L4GraphMemory) extractTriples(ctx context.Context, userInput, aiResponse string) ([]struct{ Subject, Predicate, Object string }, error) {
	if m.Client == nil {
		return nil, nil
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Extract factual triples from this conversation as (subject, predicate, object).
Include temporal info when possible.

User: %s
AI: %s

Return JSON array: [{"subject": "...", "predicate": "...", "object": "..."}]
Return [] if no facts.`, userInput, aiResponse)},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return nil, err
	}
	var raw []map[string]string
	if err := json.Unmarshal([]byte(resp.Content), &raw); err != nil {
		return nil, nil
	}
	var out []struct{ Subject, Predicate, Object string }
	for _, r := range raw {
		if r["subject"] != "" && r["predicate"] != "" && r["object"] != "" {
			out = append(out, struct{ Subject, Predicate, Object string }{
				Subject: r["subject"], Predicate: r["predicate"], Object: r["object"],
			})
		}
	}
	return out, nil
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
