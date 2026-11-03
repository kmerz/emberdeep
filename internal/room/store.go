package room

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"emberdeep/internal/game"
	"emberdeep/internal/proto"
)

// Saved games live in Config.SaveDir as one JSON file per room code, so the invite link still works after a
// server restart: opening it brings the room back in the lobby with the save loaded.

type saveFile struct {
	Version string
	Code    string
	SavedAt time.Time
	Run     game.RunSave
}

// SaveMaxAge: a save nobody has opened for this long is deleted when the server starts.
const SaveMaxAge = 30 * 24 * time.Hour

func savePath(dir, code string) string { return filepath.Join(dir, code+".json") }

func loadSave(dir, code string) (*saveFile, error) {
	if dir == "" || code == "" || NormalizeCode(code) != code { // only real codes: never a path from outside
		return nil, os.ErrNotExist
	}
	b, err := os.ReadFile(savePath(dir, code))
	if err != nil {
		return nil, err
	}
	var s saveFile
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if s.Run.Heroes == nil {
		return nil, errors.New("save has no heroes")
	}
	return &s, nil
}

func saveExists(dir, code string) bool {
	if dir == "" || code == "" || NormalizeCode(code) != code {
		return false
	}
	_, err := os.Stat(savePath(dir, code))
	return err == nil
}

// writeSave writes atomically: a crash mid-write leaves the old save, never half a file.
func writeSave(dir, code string, run game.RunSave, now time.Time) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(saveFile{Version: proto.Version, Code: code, SavedAt: now, Run: run})
	if err != nil {
		return err
	}
	tmp := savePath(dir, code) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, savePath(dir, code))
}

func deleteSave(dir, code string) {
	if dir != "" {
		_ = os.Remove(savePath(dir, code))
	}
}

// pruneSaves deletes saves nobody has touched for longer than maxAge (and stray temp files).
func pruneSaves(dir string, maxAge time.Duration, now time.Time) {
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == fameFile || (filepath.Ext(name) != ".json" && filepath.Ext(name) != ".tmp") {
			continue
		}
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > maxAge {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}
