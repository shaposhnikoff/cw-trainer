# CW Trainer

Тренажёр кода Морзе для ключа с iambic paddle (VBand и совместимые).  
Подключается к устройству через `/dev/input`, воспроизводит тон, декодирует символы и отображает текст в TUI.

## Требования

- **Go** 1.24+
- **Linux** (используется `evdev`, только Linux)
- **ALSA** — для вывода звука (пакет `libasound2-dev` или эквивалент)
- Устройство CW ключа, видимое как `/dev/input/eventN`

### Установка зависимостей (Debian/Ubuntu)

```bash
sudo apt install libasound2-dev
```

## Сборка

```bash
git clone <repo-url>
cd cw-trainer

# Загрузить зависимости
go mod download

# Собрать бинарник
go build -o cw-trainer ./cmd/cw-trainer
```

Бинарник `cw-trainer` появится в текущей директории.

### Установка в PATH

```bash
go install ./cmd/cw-trainer
# бинарник окажется в $GOPATH/bin/cw-trainer
```

## Запуск

```bash
# Базовый запуск (устройство по умолчанию /dev/input/event4)
./cw-trainer

# Указать устройство явно
./cw-trainer --device /dev/input/event3

# Изменить начальную скорость и частоту тона
./cw-trainer --wpm 20 --freq 600

# Режим iambic B
./cw-trainer --mode iambic-b

# Прямой ключ (straight key)
./cw-trainer --mode straight

# Отладочный режим — вывод сырых событий без TUI
./cw-trainer --debug
```

### Все флаги

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `--device` | `/dev/input/event4` | Путь к evdev-устройству |
| `--wpm` | `15` | Начальная скорость (WPM) |
| `--freq` | `700` | Частота тона (Гц) |
| `--mode` | `iambic-a` | Режим: `iambic-a`, `iambic-b`, `straight` |
| `--debug` | `false` | Отладочный режим |

## Определение устройства

```bash
# Показать все input-устройства
ls /dev/input/event*

# Найти нужное по имени
cat /proc/bus/input/devices | grep -A5 -i "vband\|cw\|morse"

# Мониторинг событий конкретного устройства (evtest)
sudo evtest /dev/input/event4
```

## Права доступа

По умолчанию `/dev/input/event*` доступны только root.  
Чтобы запускать без `sudo`, добавьте пользователя в группу `input`:

```bash
sudo usermod -aG input $USER
# Перелогиниться или применить без перезахода:
newgrp input
```

## Управление в TUI

| Клавиша | Действие |
|---------|----------|
| `Q` / `Ctrl+C` | Выход |
| `+` / `=` | Увеличить частоту тона (+10 Гц) |
| `-` | Уменьшить частоту тона (−10 Гц) |
| `R` | Сбросить декодированный текст и статистику |

## Сборка для разработки

```bash
# Запустить тесты
go test ./...

# Запустить тесты с выводом
go test -v ./internal/decoder/...

# Линтер
go vet ./...
```

## Структура проекта

```
cw-trainer/
├── cmd/cw-trainer/main.go       # точка входа, CLI флаги
├── internal/
│   ├── input/evdev.go           # чтение событий evdev → KeyEvent
│   ├── audio/tone.go            # генерация синусоиды (oto/v2)
│   ├── decoder/
│   │   ├── iambic.go            # iambic keyer FSM (Mode A/B)
│   │   ├── timing.go            # адаптивный декодер тайминга
│   │   └── morse_table.go       # таблица Морзе
│   └── ui/tui.go                # TUI (bubbletea + lipgloss)
├── go.mod
└── go.sum
```
