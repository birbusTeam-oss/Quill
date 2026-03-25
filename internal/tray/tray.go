package tray

import (
	"github.com/getlantern/systray"
)

// Status represents the current app state.
type Status int

const (
	StatusIdle Status = iota
	StatusRecording
	StatusTranscribing
	StatusDone
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusIdle:
		return "Idle"
	case StatusRecording:
		return "Recording..."
	case StatusTranscribing:
		return "Transcribing..."
	case StatusDone:
		return "Done"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Tray manages the system tray icon and menu.
type Tray struct {
	QuitCh    chan struct{}
	hotkey    string
	statusItem *systray.MenuItem
}

// New creates a tray manager.
func New(hotkey string) *Tray {
	return &Tray{
		QuitCh: make(chan struct{}),
		hotkey: hotkey,
	}
}

// Run starts the system tray. This BLOCKS — call from main goroutine.
func (t *Tray) Run(onReady func()) {
	systray.Run(func() {
		systray.SetTitle("Quill")
		systray.SetTooltip("Quill — Hold " + t.hotkey + " to dictate")

		systray.SetIcon(generateIcon())

		t.statusItem = systray.AddMenuItem("Status: Idle", "Current status")
		t.statusItem.Disable()

		systray.AddSeparator()

		hotkeyItem := systray.AddMenuItem("Hotkey: "+t.hotkey, "Current hotkey")
		hotkeyItem.Disable()

		systray.AddSeparator()

		mQuit := systray.AddMenuItem("Quit", "Quit Quill")

		if onReady != nil {
			onReady()
		}

		go func() {
			<-mQuit.ClickedCh
			close(t.QuitCh)
			systray.Quit()
		}()
	}, func() {
		// onExit
	})
}

// SetStatus updates the status display.
func (t *Tray) SetStatus(s Status) {
	if t.statusItem != nil {
		t.statusItem.SetTitle("Status: " + s.String())
	}
	tooltip := "Quill — "
	switch s {
	case StatusRecording:
		tooltip += "🎙 Recording..."
	case StatusTranscribing:
		tooltip += "⏳ Transcribing..."
	case StatusDone:
		tooltip += "✅ Done"
	case StatusError:
		tooltip += "❌ Error"
	default:
		tooltip += "Hold " + t.hotkey + " to dictate"
	}
	systray.SetTooltip(tooltip)
}


