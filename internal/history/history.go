package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/birbusTeam-oss/quill/internal/config"
)

const maxEntries = 50

// Entry is a single transcription record.
type Entry struct {
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	WordCount int       `json:"word_count"`
}

// History manages transcription history.
type History struct {
	Entries []Entry `json:"entries"`
	mu      sync.Mutex
	path    string
}

// New loads or creates history.
func New() *History {
	h := &History{}
	dir, err := config.DataDir()
	if err == nil {
		h.path = filepath.Join(dir, "history.json")
		h.load()
	}
	return h
}

func (h *History) load() {
	data, err := os.ReadFile(h.path)
	if err != nil {
		return
	}
	json.Unmarshal(data, h)
}

func (h *History) save() error {
	if h.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.path, data, 0644)
}

// Add records a new transcription.
func (h *History) Add(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry := Entry{
		Text:      text,
		Timestamp: time.Now(),
		WordCount: len(strings.Fields(text)),
	}
	h.Entries = append(h.Entries, entry)
	if len(h.Entries) > maxEntries {
		h.Entries = h.Entries[len(h.Entries)-maxEntries:]
	}
	h.save()
}

// GetAll returns all entries.
func (h *History) GetAll() []Entry {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Entry, len(h.Entries))
	copy(out, h.Entries)
	return out
}

// Clear removes all entries.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Entries = nil
	h.save()
}

// Search returns entries containing the query.
func (h *History) Search(query string) []Entry {
	h.mu.Lock()
	defer h.mu.Unlock()
	query = strings.ToLower(query)
	var results []Entry
	for _, e := range h.Entries {
		if strings.Contains(strings.ToLower(e.Text), query) {
			results = append(results, e)
		}
	}
	return results
}
