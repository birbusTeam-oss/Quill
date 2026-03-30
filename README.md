# Quill ✒️

**Voice dictation for Windows. Offline. Fast. Zero cloud.**

Hold a hotkey, speak, release — your words appear wherever your cursor is.

![MIT License](https://img.shields.io/badge/license-MIT-purple)
![Go](https://img.shields.io/badge/built%20with-Go-00ADD8)
![Windows](https://img.shields.io/badge/platform-Windows-0078D4)

## Download

Get the latest from [Releases](https://github.com/birbusTeam-oss/Quill/releases):
- **Quill.exe** — portable, just run it
- **Quill-v1.2.0-windows-x64.zip** — same thing, zipped

## How It Works

1. Launch Quill — a small purple dot appears at the bottom of your screen
2. Hold **Ctrl+Alt** and speak
3. Release — text appears at your cursor

That's it. First launch auto-downloads the speech engine (~150MB one-time). After that, everything runs locally.

## Features

- **Fully offline** — whisper.cpp runs locally, nothing leaves your machine
- **Hold-to-talk** — hold Ctrl+Alt, speak, release
- **Auto-setup** — downloads whisper.cpp + model on first launch
- **Floating overlay** — shows recording → transcribing → done status
- **System tray** — start with Windows option
- **Smart cleanup** — removes filler words (um, uh, er), trims silence
- **Fast** — model pre-warming, greedy decoding, silence trimming
- **Tiny** — ~6.6MB binary, zero dependencies

## Configuration

Config lives at `%APPDATA%\Quill\config.json`:

```json
{
  "hotkey": "Ctrl+Alt",
  "log_transcriptions": true,
  "remove_fillers": true
}
```

## Build from Source

```bash
git clone https://github.com/birbusTeam-oss/Quill.git
cd Quill
go install github.com/tc-hib/go-winres@latest
go-winres make
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui -s -w" -o Quill.exe ./cmd/quill
```

## Requirements

- Windows 10/11 (x64)
- A microphone
- ~150MB disk space (whisper + model, auto-downloaded)

## License

MIT

---

Built by [Birbus Team](https://github.com/birbusTeam-oss)
