package koch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Progress struct {
	Level       int                 `json:"level"`
	WPM         int                 `json:"wpm"`
	SymbolStats map[rune]SymbolStat `json:"symbol_stats"`
	LastSession time.Time           `json:"last_session"`
}

func progressPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cw-trainer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "progress.json"), nil
}

func LoadProgress() (*Progress, error) {
	path, err := progressPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Progress{Level: MinLevel, WPM: 20, SymbolStats: make(map[rune]SymbolStat)}, nil
		}
		return nil, err
	}
	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.SymbolStats == nil {
		p.SymbolStats = make(map[rune]SymbolStat)
	}
	if p.Level < MinLevel {
		p.Level = MinLevel
	}
	return &p, nil
}

func SaveProgress(p *Progress) error {
	path, err := progressPath()
	if err != nil {
		return err
	}
	p.LastSession = time.Now()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
