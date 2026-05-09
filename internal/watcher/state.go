package watcher

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
)

type FileRepo struct {
	path string
}

func NewFileRepo(path string) *FileRepo {
	return &FileRepo{path: path}
}

func (r *FileRepo) Load() (map[string]int64, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]int64{}, nil
		}
		return nil, err
	}
	out := map[string]int64{}
	if len(data) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]int64{}, nil
	}
	return out, nil
}

func (r *FileRepo) Save(offsets map[string]int64) error {
	data, err := json.MarshalIndent(offsets, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}
