package memory

import (
	"bufio"
	"encoding/json"
	"os"
)

const (
	defaultFilePerm   = 0644
	defaultMaxEntries = 200
)

func appendJSONL(path string, entry map[string]any) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultFilePerm)
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
	if len(out) > defaultMaxEntries {
		out = out[len(out)-defaultMaxEntries:]
	}
	return out, nil
}

func mapToEntry(m map[string]any) Entry {
	text, _ := m["text"].(string)
	ts, _ := m["timestamp"].(string)
	return Entry{Text: text, Timestamp: ts, Metadata: m}
}
