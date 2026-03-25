package hotkey

import (
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

// Virtual key codes.
const (
	VK_CONTROL = 0x11
	VK_MENU    = 0x12 // Alt
	VK_SHIFT   = 0x10
)

// Event types sent on the channel.
type Event int

const (
	EventStart Event = iota
	EventStop
)

// Listener polls for a hotkey combo using GetAsyncKeyState.
type Listener struct {
	keys     []int
	interval time.Duration
	Events   chan Event
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// New creates a hotkey listener for the given combo string.
// Supported: "ctrl+alt", "ctrl+shift", "alt+shift"
func New(combo string) *Listener {
	keys := parseCombo(combo)
	return &Listener{
		keys:     keys,
		interval: 50 * time.Millisecond,
		Events:   make(chan Event, 8),
		stopCh:   make(chan struct{}),
	}
}

// SetCombo updates the hotkey combo.
func (l *Listener) SetCombo(combo string) {
	l.mu.Lock()
	l.keys = parseCombo(combo)
	l.mu.Unlock()
}

// Start begins polling in a goroutine.
func (l *Listener) Start() {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	l.running = true
	l.mu.Unlock()

	go l.poll()
}

// Stop halts the polling goroutine.
func (l *Listener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running {
		return
	}
	l.running = false
	close(l.stopCh)
}

func (l *Listener) poll() {
	wasPressed := false
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.mu.Lock()
			keys := make([]int, len(l.keys))
			copy(keys, l.keys)
			l.mu.Unlock()

			allDown := true
			for _, vk := range keys {
				if !isKeyDown(vk) {
					allDown = false
					break
				}
			}

			if allDown && !wasPressed {
				wasPressed = true
				select {
				case l.Events <- EventStart:
				default:
				}
			} else if !allDown && wasPressed {
				wasPressed = false
				select {
				case l.Events <- EventStop:
				default:
				}
			}
		}
	}
}

func isKeyDown(vk int) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return *(*int16)(unsafe.Pointer(&ret)) < 0
}

func parseCombo(combo string) []int {
	combo = strings.ToLower(strings.TrimSpace(combo))
	var keys []int
	for _, part := range strings.Split(combo, "+") {
		switch strings.TrimSpace(part) {
		case "ctrl", "control":
			keys = append(keys, VK_CONTROL)
		case "alt", "menu":
			keys = append(keys, VK_MENU)
		case "shift":
			keys = append(keys, VK_SHIFT)
		}
	}
	if len(keys) == 0 {
		keys = []int{VK_CONTROL, VK_MENU} // default ctrl+alt
	}
	return keys
}
