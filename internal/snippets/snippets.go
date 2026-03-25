package snippets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/birbusTeam-oss/quill/internal/config"
)

// Manager handles text snippet expansion.
type Manager struct {
	Snippets map[string]string `json:"snippets"`
	mu       sync.RWMutex
	path     string
}

// New loads or creates a snippet manager.
func New() *Manager {
	m := &Manager{
		Snippets: make(map[string]string),
	}
	dir, err := config.DataDir()
	if err == nil {
		m.path = filepath.Join(dir, "snippets.json")
		m.load()
	}
	return m
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	json.Unmarshal(data, m)
	if m.Snippets == nil {
		m.Snippets = make(map[string]string)
	}
}

func (m *Manager) save() error {
	if m.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}

// Set adds or updates a snippet.
func (m *Manager) Set(trigger, expansion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Snippets[trigger] = expansion
	return m.save()
}

// Delete removes a snippet.
func (m *Manager) Delete(trigger string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Snippets, trigger)
	return m.save()
}

// Expand checks text for snippet triggers and replaces them.
func (m *Manager) Expand(text string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for trigger, expansion := range m.Snippets {
		// Case-insensitive replacement
		lower := strings.ToLower(text)
		triggerLower := strings.ToLower(trigger)
		for {
			idx := strings.Index(lower, triggerLower)
			if idx < 0 {
				break
			}
			text = text[:idx] + expansion + text[idx+len(trigger):]
			lower = strings.ToLower(text)
		}
	}
	return text
}

// GetAll returns all snippets.
func (m *Manager) GetAll() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.Snippets))
	for k, v := range m.Snippets {
		out[k] = v
	}
	return out
}
