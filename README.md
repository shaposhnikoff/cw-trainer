# CW Trainer

Набор Linux-тренажёров кода Морзе для ключа с iambic paddle (VBand и совместимые).

- `cw-trainer` — декодирование в реальном времени + Koch режим
- `cw-groups` — проигрывание групп (по умолчанию 5 символов) и повтор группы целиком

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

# Собрать оба бинарника
make build
```

Будут собраны бинарники `cw-trainer` и `cw-groups`.

### Отдельные цели Makefile

```bash
make build-trainer
make build-groups
```

### Установка в PATH

```bash
go install ./cmd/cw-trainer
go install ./cmd/cw-groups
# бинарники окажутся в $GOPATH/bin/
```

## Запуск `cw-trainer`

```bash
# Базовый запуск (устройство по умолчанию /dev/input/event4)
./cw-trainer

# Указать устройство явно
./cw-trainer --device /dev/input/event3

# Изменить начальную скорость и частоту тона
./cw-trainer --wpm 20 --freq 600

# Режим iambic B
./cw-trainer --mode iambic-b

# Отладочный режим — вывод сырых событий без TUI
./cw-trainer --debug
```

### Все флаги

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `--device` | `/dev/input/event4` | Путь к evdev-устройству |
| `--wpm` | `15` | Начальная скорость (WPM) |
| `--freq` | `700` | Частота тона (Гц) |
| `--mode` | `iambic-a` | Режим: `iambic-a`, `iambic-b` |
| `--debug` | `false` | Отладочный режим |

## Запуск `cw-groups`

```bash
# Базовый запуск (группы по 5 символов, уровень 2)
./cw-groups

# Изменить размер группы, уровень, скорость и частоту
./cw-groups --group-size 7 --level 5 --wpm 22 --freq 650

# Режим iambic B для ввода с ключа
./cw-groups --mode iambic-b
```

### Флаги `cw-groups`

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `--device` | `/dev/input/event4` | Путь к evdev-устройству |
| `--wpm` | `20` | Скорость (WPM) |
| `--freq` | `700` | Частота тона (Гц) |
| `--mode` | `iambic-a` | Режим keyer: `iambic-a`, `iambic-b` |
| `--letter-space` | `4.0` | Порог межбуквенной паузы (× dit) |
| `--group-size` | `5` | Размер группы |
| `--level` | `2` | Koch уровень (`2..40`) |

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

Для `cw-groups`:

| Клавиша | Действие |
|---------|----------|
| `Q` / `Ctrl+C` | Выход |
| `R` | Сброс статистики сессии |
| `A-Z`, `0-9`, `.`, `,`, `?`, `/` | Ввод ответа с клавиатуры |

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
├── cmd/
│   ├── cw-trainer/main.go       # точка входа основного тренажёра
│   └── cw-groups/main.go        # точка входа группового тренажёра
├── internal/
│   ├── input/evdev.go           # чтение событий evdev → KeyEvent
│   ├── audio/tone.go            # генерация синусоиды (oto/v2)
│   ├── decoder/
│   │   ├── iambic.go            # iambic keyer FSM (Mode A/B)
│   │   ├── timing.go            # адаптивный декодер тайминга
│   │   └── morse_table.go       # таблица Морзе
│   ├── groups/trainer.go        # логика групп: генерация, проверка, статистика
│   └── ui/
│       ├── tui.go               # TUI основного режима
│       ├── koch_tui.go          # TUI режима Koch
│       └── groups_tui.go        # TUI режима групп
├── go.mod
└── go.sum
```
