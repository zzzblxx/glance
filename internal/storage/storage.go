package storage

import (
	"encoding/json"
	"github.com/zzzblxx/glance/internal/model"
	"os"
	"path/filepath"
)

func Save(path string, d model.Dashboard) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return AtomicWrite(path, data)
}
func Load(path string) (model.Dashboard, error) {
	var d model.Dashboard
	file, err := os.Open(path)
	if err != nil {
		return d, err
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&d)
	return d, err
}
func AtomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".glance-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
