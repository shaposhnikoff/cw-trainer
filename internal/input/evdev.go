//go:build linux

package input

import (
	"context"
	"fmt"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

type Key int

const (
	DitKey Key = iota
	DahKey
)

type Action int

const (
	Press Action = iota
	Release
)

type KeyEvent struct {
	Key       Key
	Action    Action
	Timestamp time.Time
}

const (
	keyCodeDit = 26 // KEY_LEFTBRACE
	keyCodeDah = 27 // KEY_RIGHTBRACE
)

func ReadEvents(ctx context.Context, devicePath string, out chan<- KeyEvent) error {
	dev, err := evdev.Open(devicePath)
	if err != nil {
		return fmt.Errorf("open device %s: %w", devicePath, err)
	}
	defer dev.Close()

	if err := dev.Grab(); err != nil {
		return fmt.Errorf("grab device %s: %w", devicePath, err)
	}

	go func() {
		<-ctx.Done()
		dev.Close()
	}()

	for {
		event, err := dev.ReadOne()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("read event: %w", err)
			}
		}

		// Only care about EV_KEY events
		if event.Type != evdev.EV_KEY {
			continue
		}

		// Ignore key repeat (value 2)
		if event.Value == 2 {
			continue
		}

		var key Key
		switch int(event.Code) {
		case keyCodeDit:
			key = DitKey
		case keyCodeDah:
			key = DahKey
		default:
			continue
		}

		var action Action
		switch event.Value {
		case 1:
			action = Press
		case 0:
			action = Release
		default:
			continue
		}

		ke := KeyEvent{
			Key:       key,
			Action:    action,
			Timestamp: time.Now(),
		}

		select {
		case out <- ke:
		case <-ctx.Done():
			return nil
		}
	}
}
