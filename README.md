# Quill

Offline voice dictation for Windows. Hold a hotkey, speak, text appears wherever your cursor is.

**4.4MB binary. Zero dependencies. Zero cloud. Zero API keys.**

Written in Go. Single static binary. No installers, no runtime, no DLL hell.

## Download

Go to [Releases](https://github.com/birbusTeam-oss/Quill/releases/latest) and download:
- `quill.exe` — the app (4.4MB)
- `whisper.exe` — speech engine
- `ggml-base.en.bin` — English model (~150MB, first download only)

Put all three in the same folder and run `quill.exe`.

## How to Use

1. Run `quill.exe` — appears in your system tray
2. Open any text field (Notepad, browser, chat, anywhere)
3. **Hold Ctrl+Alt** and speak
4. **Release** — text appears at your cursor

## Features

- **100% offline** — no internet required after model download
- **Filler word removal** — "um", "uh", "like" auto-stripped
- **Snippet library** — voice shortcuts (say "my email" → expands to your email)
- **Transcription history** — searchable local log
- **Customizable hotkey** — Ctrl+Alt, Ctrl+Shift, Alt+Shift
- **Zero data collection** — everything stays on your machine

## Config

Settings stored at `%APPDATA%\Quill\config.json`:

```json
{
  "hotkey": "ctrl+alt",
  "model_path": "ggml-base.en.bin",
  "remove_fillers": true,
  "log_transcriptions": false
}
```

## Build from Source

```bash
# Requires Go 1.21+
git clone https://github.com/birbusTeam-oss/Quill
cd Quill

# Build for Windows (from any OS)
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui -s -w" -o quill.exe ./cmd/quill
```

## Architecture

```
cmd/quill/main.go       — Entry point, wires everything together
internal/
  hotkey/hotkey.go       — GetAsyncKeyState polling (Windows API, no hooks)
  audio/recorder.go      — winmm.dll mic recording (16kHz WAV)
  transcriber/           — Shells out to whisper.cpp CLI
  injector/injector.go   — Clipboard + SendInput paste (Windows API)
  tray/tray.go           — System tray icon + menu
  config/config.go       — JSON config at %APPDATA%/Quill/
  history/history.go     — Transcription history
  snippets/snippets.go   — Voice-triggered text expansion
```

Zero CGO. Pure Go + Windows syscalls. Cross-compiles from Linux/Mac.

## Requirements

- Windows 10 or 11
- Microphone
- `whisper.exe` + model file (included in release)

## License

MIT
