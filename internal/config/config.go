package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds all Yappie settings.
type Config struct {
	// Input
	Hotkey string `json:"hotkey"`

	// Transcription
	Model         string `json:"model"`
	Language      string `json:"language"`
	WhisperPath   string `json:"whisper_path"`
	ModelPath     string `json:"model_path"`
	RemoveFillers bool   `json:"remove_fillers"`
	Threads       int    `json:"threads"`

	// Behavior
	LogTranscriptions  bool `json:"log_transcriptions"`
	PlaySounds         bool `json:"play_sounds"`
	AutoCapitalize     bool `json:"auto_capitalize"`
	AddPunctuation     bool `json:"add_punctuation"`

	mu   sync.RWMutex
	path string
}

// DefaultConfig returns config with sane defaults.
func DefaultConfig() *Config {
	return &Config{
		Hotkey:             "ctrl+alt",
		Model:              "tiny.en",
		Language:           "en",
		WhisperPath:        "",
		ModelPath:          "",
		RemoveFillers:      true,
		Threads:            4,
		LogTranscriptions:  true,
		PlaySounds:         true,
		AutoCapitalize:     true,
		AddPunctuation:     true,
	}
}

// configDir returns %APPDATA%/Yappie, creating it if needed.
func configDir() (string, error) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		appdata = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}
	dir := filepath.Join(appdata, "Yappie")
	return dir, os.MkdirAll(dir, 0755)
}

// ConfigPath returns the full path to config.json.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads config from disk, or returns defaults if file doesn't exist.
func Load() (*Config, error) {
	p, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			cfg.path = p
			_ = cfg.Save()
			return cfg, nil
		}
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}
	cfg.path = p

	// Validate
	if cfg.Threads < 1 {
		cfg.Threads = 4
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}

	return cfg, nil
}

// Save writes config to disk.
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	p := c.path
	if p == "" {
		var err error
		p, err = ConfigPath()
		if err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// GetHotkey returns a thread-safe copy of the hotkey string.
func (c *Config) GetHotkey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Hotkey
}

// SetHotkey updates the hotkey and saves.
func (c *Config) SetHotkey(hk string) error {
	c.mu.Lock()
	c.Hotkey = hk
	c.mu.Unlock()
	return c.Save()
}

// DataDir returns the Yappie data directory (%APPDATA%/Yappie).
func DataDir() (string, error) {
	return configDir()
}
