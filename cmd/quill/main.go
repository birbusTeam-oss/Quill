package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/birbusTeam-oss/quill/internal/audio"
	"github.com/birbusTeam-oss/quill/internal/config"
	"github.com/birbusTeam-oss/quill/internal/history"
	"github.com/birbusTeam-oss/quill/internal/hotkey"
	"github.com/birbusTeam-oss/quill/internal/injector"
	"github.com/birbusTeam-oss/quill/internal/snippets"
	"github.com/birbusTeam-oss/quill/internal/transcriber"
	"github.com/birbusTeam-oss/quill/internal/tray"
)

func main() {
	// Setup logging
	dataDir, _ := config.DataDir()
	if dataDir != "" {
		logPath := filepath.Join(dataDir, "quill.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			log.SetOutput(logFile)
			defer logFile.Close()
		}
	}
	log.Println("Quill starting...")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: config load error: %v (using defaults)", err)
	}

	// Initialize components
	rec := audio.NewRecorder()
	trans := transcriber.New(cfg.WhisperPath, cfg.ModelPath, cfg.RemoveFillers)
	hist := history.New()
	snips := snippets.New()
	hk := hotkey.New(cfg.GetHotkey())
	tr := tray.New(cfg.GetHotkey())

	// Main logic: wire hotkey events to record/transcribe/inject pipeline
	var recording bool

	go func() {
		for evt := range hk.Events {
			switch evt {
			case hotkey.EventStart:
				if recording {
					continue
				}
				recording = true
				log.Println("Recording started")
				tr.SetStatus(tray.StatusRecording)

				if err := rec.Start(); err != nil {
					log.Printf("Record start error: %v", err)
					tr.SetStatus(tray.StatusError)
					recording = false
				}

			case hotkey.EventStop:
				if !recording {
					continue
				}
				recording = false
				log.Println("Recording stopped, transcribing...")
				tr.SetStatus(tray.StatusTranscribing)

				wavPath, err := rec.Stop()
				if err != nil {
					log.Printf("Record stop error: %v", err)
					tr.SetStatus(tray.StatusError)
					continue
				}

				// Transcribe in background
				go func(wav string) {
					defer os.Remove(wav) // cleanup temp file

					text, err := trans.Transcribe(wav)
					if err != nil {
						log.Printf("Transcription error: %v", err)
						tr.SetStatus(tray.StatusError)
						return
					}

					if text == "" {
						log.Println("Empty transcription")
						tr.SetStatus(tray.StatusIdle)
						return
					}

					// Apply snippet expansion
					text = snips.Expand(text)

					// Log to history
					if cfg.LogTranscriptions {
						hist.Add(text)
						log.Printf("Transcribed: %s", text)
					}

					// Inject into active window
					if err := injector.InjectText(text); err != nil {
						log.Printf("Injection error: %v", err)
						tr.SetStatus(tray.StatusError)
						return
					}

					tr.SetStatus(tray.StatusDone)
					log.Println("Text injected successfully")

					// Reset to idle after a moment
					time.Sleep(2 * time.Second)
					tr.SetStatus(tray.StatusIdle)
				}(wavPath)
			}
		}
	}()

	// Start hotkey listener
	hk.Start()
	defer hk.Stop()

	// Run tray (blocks until quit)
	tr.Run(func() {
		log.Println("Quill ready — tray active")
		fmt.Println("Quill is running. Use the system tray to quit.")
	})

	log.Println("Quill shutting down")
}
