# CW Trainer

A Linux Morse training toolkit for iambic paddles (VBand and compatible).

- `cw-trainer` — real-time decoder + Koch trainer mode
- `cw-groups` — plays fixed-size groups (default 5) and asks you to repeat the whole group

## Features

- **Iambic keyer** — Mode A and Mode B, squeeze keying, auto-repeating dit/dah
- **Real-time audio** — sine wave tone via ALSA/PulseAudio, 5ms fade envelope (no clicks)
- **Adaptive timing** — automatically adjusts to your sending speed (rolling 8-symbol average)
- **Koch Trainer mode** — structured learning method: start with 2 characters, advance when you hit 90% accuracy
- **Progress persistence** — Koch level and per-character stats saved to `~/.cw-trainer/progress.json`
- **Live TUI** — decoded text, paddle activity, WPM, session timer

## Requirements

- **Go** 1.24+
- **Linux** (evdev input, Linux only)
- **ALSA** — `libasound2-dev` or equivalent

### Install dependencies (Debian/Ubuntu)

```bash
sudo apt install libasound2-dev
```

## Build

```bash
git clone https://github.com/shaposhnikoff/cw-trainer
cd cw-trainer
make build
```

`make build` produces both binaries: `cw-trainer` and `cw-groups`.

### Separate Make targets

```bash
make build-trainer
make build-groups
```

### Install to PATH

```bash
go install ./cmd/cw-trainer
go install ./cmd/cw-groups
```

## Usage

```bash
# Default device /dev/input/event4
./cw-trainer

# Specify device explicitly
./cw-trainer --device /dev/input/event3

# Set speed and tone frequency
./cw-trainer --wpm 20 --freq 650

# Iambic Mode B
./cw-trainer --mode iambic-b

# Koch Trainer mode
./cw-trainer --koch --wpm 20

# Debug mode — prints decoded symbols, no TUI
./cw-trainer --debug --wpm 20
```

### `cw-groups` usage

```bash
# Default: group size 5, Koch level 2
./cw-groups

# Tune group size, level, speed, tone
./cw-groups --group-size 7 --level 5 --wpm 22 --freq 650

# Iambic mode B for paddle input
./cw-groups --mode iambic-b
```

### All flags

| Flag | Default | Description |
|------|---------|-------------|
| `--device` | `/dev/input/event4` | evdev device path |
| `--wpm` | `15` | Initial speed in WPM |
| `--freq` | `700` | Tone frequency in Hz |
| `--mode` | `iambic-a` | Keyer mode: `iambic-a`, `iambic-b` |
| `--letter-space` | `4.0` | Letter space threshold (× dit duration) |
| `--koch` | `false` | Koch Trainer mode |
| `--debug` | `false` | Debug mode: print symbols, no TUI |

### `cw-groups` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--device` | `/dev/input/event4` | evdev device path |
| `--wpm` | `20` | Speed in WPM |
| `--freq` | `700` | Tone frequency in Hz |
| `--mode` | `iambic-a` | Keyer mode: `iambic-a`, `iambic-b` |
| `--letter-space` | `4.0` | Letter-space threshold (× dit duration) |
| `--group-size` | `5` | Symbols in one group |
| `--level` | `2` | Koch level (`2..40`) |

## Device setup

```bash
# List all input devices
ls /dev/input/event*

# Find your paddle by name
cat /proc/bus/input/devices | grep -A5 -i "vband\|cw\|morse"

# Monitor events from a specific device
sudo evtest /dev/input/event4
```

**Device protocol (VBand CW Trainer):**
- `KEY_LEFTBRACE` (code 26) → dit (dot), left paddle
- `KEY_RIGHTBRACE` (code 27) → dah (dash), right paddle

## Permissions

`/dev/input/event*` requires the `input` group:

```bash
sudo usermod -aG input $USER
newgrp input   # apply without re-login
```

## TUI controls

| Key | Action |
|-----|--------|
| `Q` / `Ctrl+C` | Quit |
| `+` / `=` | Increase tone frequency (+10 Hz) |
| `-` | Decrease tone frequency (−10 Hz) |
| `R` | Reset decoded text and session stats |

`cw-groups` controls:

| Key | Action |
|-----|--------|
| `Q` / `Ctrl+C` | Quit |
| `R` | Reset session stats |
| `A-Z`, `0-9`, `.`, `,`, `?`, `/` | Type group answer from keyboard |

## Koch Trainer

The [Koch method](https://www.qsl.net/n1irz/finley.koch.html) teaches Morse at full speed from the start:

1. Begin with 2 characters (K and M)
2. Practice until you reach **90% accuracy** over 50 symbols
3. Add the next character from the Koch sequence
4. Repeat until you know all 40 characters

```bash
./cw-trainer --koch --wpm 20
```

Progress is saved automatically to `~/.cw-trainer/progress.json` between sessions.

**Koch character order:**
```
K M R S U A P T L O W I . N J E F 0 Y V , G 5 / Q 9 Z H 3 8 B ? 4 2 7 C 1 D 6 X
```

## Project structure

```
cw-trainer/
├── cmd/
│   ├── cw-trainer/main.go        # main trainer entry point
│   └── cw-groups/main.go         # groups trainer entry point
├── internal/
│   ├── input/evdev.go            # evdev reader → KeyEvent channel
│   ├── audio/tone.go             # PCM sine wave via oto/v2, io.Pipe
│   ├── decoder/
│   │   ├── iambic.go             # iambic keyer FSM (Mode A/B)
│   │   ├── timing.go             # adaptive timing decoder
│   │   ├── morse_table.go        # Morse code table A-Z, 0-9, punctuation
│   │   └── *_test.go             # tests for decoder and keyer
│   ├── koch/
│   │   ├── session.go            # Koch session logic
│   │   ├── progress.go           # JSON progress persistence
│   │   └── morse_map.go          # symbol→pattern lookup
│   ├── groups/
│   │   ├── trainer.go            # group generation/checking/stats logic
│   │   └── trainer_test.go       # tests for group logic
│   └── ui/
│       ├── tui.go                # main decoder TUI
│       ├── koch_tui.go           # Koch Trainer TUI screen
│       └── groups_tui.go         # groups mode TUI screen
├── go.mod
└── go.sum
```

## Architecture

```
evdev goroutine
    │  chan KeyEvent
    ▼
iambic FSM goroutine          (timer-driven, single-reader — no races)
    │  onElement(toneMs, gapMs)          onSymbol(sym, durationMs)
    ▼                                    ▼
audio goroutine               timing decoder
  io.Pipe → oto player          adaptive ditMs
  exact PCM per element         letter/word space detection
                                    │  onChar(rune)
                                    ▼
                              TUI goroutine (tea.Program)
```

**Key design decision — audio via `io.Pipe`:** instead of a streaming reader with an on/off flag (which causes oto to pre-buffer audio and ignore Stop() calls), each element writes an exact PCM block (tone + silence) to the pipe. oto reads at hardware rate, so timing is precise with no buffering artifacts.

## Running tests

```bash
go test ./...
go test -v ./internal/decoder/
```
