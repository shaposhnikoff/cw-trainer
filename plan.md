# План разработки CW Trainer на Go

## Контекст

**Устройство:** VBand CW Trainer, `/dev/input/event4`
**Протокол:**
- `KEY_LEFTBRACE` (code 26) → **dit** (точка), левый paddle
- `KEY_RIGHTBRACE` (code 27) → **dah** (тире), правый paddle
- value 1 = press, value 0 = release

---

## Стек

```
github.com/holoplot/go-evdev        — чтение /dev/input/event4
github.com/hajimehoshi/oto/v2       — аудио (PCM синусоида)
github.com/charmbracelet/bubbletea  — TUI framework
github.com/charmbracelet/lipgloss   — стили TUI
```

---

## Структура проекта

```
cw-trainer/
├── cmd/cw-trainer/
│   └── main.go                  # точка входа, CLI флаги
├── internal/
│   ├── input/
│   │   └── evdev.go             # чтение событий evdev → KeyEvent
│   ├── audio/
│   │   └── tone.go              # генерация синусоиды, play/stop
│   ├── decoder/
│   │   ├── iambic.go            # iambic keyer FSM (Mode A/B)
│   │   ├── timing.go            # dit/dah/пауза → символ Морзе
│   │   └── morse_table.go       # таблица Морзе → буква/цифра
│   └── ui/
│       └── tui.go               # bubbletea Model, View, Update
├── go.mod
├── go.sum
└── README.md
```

---

## Фазы

### Phase 1 — Input layer (`internal/input/evdev.go`)

**Задача:** читать события устройства и отдавать их в channel.

```go
type KeyEvent struct {
    Key       Key       // DitKey | DahKey
    Action    Action    // Press | Release
    Timestamp time.Time
}
```

- Открыть `/dev/input/event4` через `go-evdev`
- Фильтровать только `EV_KEY`, code 26 и 27, value 0/1
- Игнорировать key repeat (value 2)
- Отдавать события в `chan KeyEvent`
- Поддержка graceful shutdown через `context.Context`

**Acceptance criteria:**
- `go run ./cmd/cw-trainer --device /dev/input/event4 --debug` печатает `DIT press/release` и `DAH press/release` с таймстампами
- Нет паники при отключении устройства

---

### Phase 2 — Audio layer (`internal/audio/tone.go`)

**Задача:** воспроизводить тон CW пока нажата клавиша.

```go
type Tone struct { /* oto context + stream */ }
func NewTone(freq int, sampleRate int) (*Tone, error)
func (t *Tone) Start()
func (t *Tone) Stop()
func (t *Tone) Close()
```

- Генерация синусоиды: `sin(2π * freq * t / sampleRate)`
- Частота: 700 Hz по умолчанию (configurable)
- Плавный старт/стоп (envelope 5ms) — без щелчков
- `Start()` → начать писать PCM в oto stream
- `Stop()` → fade out + тишина

**Acceptance criteria:**
- Тон звучит только пока нажата клавиша
- Нет щелчков на старте и стопе
- Нет артефактов при быстрых нажатиях

---

### Phase 3 — Decoder layer

#### 3a. `internal/decoder/timing.go` — timing engine

**Задача:** по длительности пресс/релиз определять dit/dah/паузы.

```go
type Symbol int
const ( Dit Symbol = iota; Dah; LetterSpace; WordSpace )
```

- WPM → длительность dit в мс: `ditMs = 1200 / WPM`
- Thresholds (стандарт ITU):
  - `< 2× dit` → **dit**
  - `≥ 2× dit` → **dah**
  - пауза `1× dit` → inter-element (внутри буквы)
  - пауза `3× dit` → **letter space**
  - пауза `7× dit` → **word space**
- Adaptive timing: подстраивать dit-длину по последним 8 символам

#### 3b. `internal/decoder/iambic.go` — iambic keyer FSM

**Задача:** обрабатывать paddle-специфичную логику.

```
States: Idle → DitSending → DahSending → DitQueued → DahQueued
```

- Mode A: завершить текущий элемент, потом проверить paddle
- Mode B: если оба нажаты в конце dit → вставить dah (и наоборот)
- Squeeze keying: оба контакта зажаты → автоматическое чередование dit/dah

#### 3c. `internal/decoder/morse_table.go` — таблица декодирования

```go
var morseTable = map[string]string{
    ".-": "A", "-...": "B", // ...
    "-----": "0", ".----": "1", // ...
    "..--..": "?", ".-.-.-": ".", // ...
}
```

- Полная таблица: A-Z, 0-9, основные знаки пунктуации
- `Decode(symbols []Symbol) (rune, bool)`

**Acceptance criteria Phase 3:**
- `--debug` режим печатает `DIT`/`DAH`/`[space]`/`[word]` в stdout
- Правильно декодирует SOS (`... --- ...`) при ~15 WPM
- Adaptive timing не ломается при резкой смене скорости

---

### Phase 4 — TUI (`internal/ui/tui.go`)

**Задача:** интерактивный терминальный интерфейс на bubbletea.

```
┌─────────────────────────────────────────────┐
│  CW Trainer  UT3UDX          [Q] quit       │
├─────────────────────────────────────────────┤
│                                             │
│  ██ ██████  ██                              │
│  DIT DAH    DIT         ← live визуализация │
│                                             │
├─────────────────────────────────────────────┤
│  Decoded:                                   │
│  CQ CQ CQ DE UT3UDX_                        │
├─────────────────────────────────────────────┤
│  Speed: 18 WPM   Freq: 700 Hz               │
│  Accuracy: 94%   Session: 00:04:32          │
└─────────────────────────────────────────────┘
```

**bubbletea Model:**
```go
type Model struct {
    decoded    []rune
    currentSeq string      // текущая последовательность dit/dah
    wpm        float64
    freq       int
    ditActive  bool
    dahActive  bool
    sessionDur time.Duration
    charCount  int
    wordCount  int
}
```

- `tea.Tick` каждые 50ms для обновления таймера и WPM
- Визуализация активного paddle в реальном времени
- Прокрутка decoded text (последние N строк)
- `+`/`-` клавиши — регулировка частоты тона
- `r` — сброс decoded text
- `q` / `Ctrl+C` — выход

**Acceptance criteria:**
- TUI обновляется без мерцания
- Decoded text не теряет символы при быстром наборе

---

### Phase 5 — Main & CLI (`cmd/cw-trainer/main.go`)

```go
flags:
  --device   string   путь к evdev устройству (default: /dev/input/event4)
  --wpm      int      начальная скорость (default: 15)
  --freq     int      частота тона Hz (default: 700)
  --mode     string   iambic-a | iambic-b | straight (default: iambic-a)
  --debug    bool     печатать сырые события без TUI
```

- Запуск всех горутин: input reader, audio engine, decoder, TUI
- Graceful shutdown: `SIGINT`/`SIGTERM` → закрыть evdev, flush audio, выйти из TUI
- Канальная архитектура:

```
evdev goroutine
    │  chan KeyEvent
    ▼
iambic FSM goroutine
    │  chan Symbol (Dit/Dah/Space)
    ▼
timing+decoder goroutine
    │  chan DecodedChar
    ▼
TUI goroutine (tea.Program)
    │
audio goroutine (слушает KeyEvent напрямую для минимальной латентности)
```

**Acceptance criteria:**
- `Ctrl+C` завершает программу чисто, без горящих горутин
- `--debug` режим работает без TUI (для тестирования без дисплея)

---

## Порядок имплементации для Claude Code

```
1. go.mod + структура директорий
2. internal/input/evdev.go  + smoke test (--debug)
3. internal/audio/tone.go   + ручная проверка звука
4. internal/decoder/morse_table.go
5. internal/decoder/timing.go
6. internal/decoder/iambic.go
7. internal/ui/tui.go
8. cmd/cw-trainer/main.go   (связывает всё вместе)
```

---

## Дополнительные замечания для Claude Code

- **Права доступа:** evdev требует `input` группы или sudo. В README добавить: `sudo usermod -aG input $USER`
- **Тестирование без железа:** в `--debug` режиме можно симулировать события через stdin (`s` = dit, `d` = dah)
- **Build tag:** `//go:build linux` на evdev файле — устройство Linux-only
- **Audio на Linux:** oto/v2 использует ALSA/PulseAudio, может потребоваться `libasound2-dev`
