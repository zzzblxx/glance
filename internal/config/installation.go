package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/zzzblxx/glance/internal/storage"
)

// MigrateInstallation operates on the installer's private staging copy, never
// the live config. Unknown fields, credentials, notes and pairing are retained.
func MigrateInstallation(path, legacyRoot, targetRoot string) error {
	if !filepath.IsAbs(targetRoot) || (legacyRoot != "" && !filepath.IsAbs(legacyRoot)) {
		return errors.New("installation directories must be absolute")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := WriteInitial(path); err != nil {
			return err
		}
		// Fresh installs may be staged outside the eventual installation path.
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		fields["data_directory"], _ = json.Marshal(targetRoot)
		b, err := json.MarshalIndent(fields, "", "  ")
		if err != nil {
			return err
		}
		return storage.AtomicWrite(path, append(b, '\n'))
	}
	if _, err := Load(path); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, key := range []string{"data_directory", "renderer_binary", "font_path", "codex_binary", "codex_home"} {
		value, present := fields[key]
		if !present {
			continue
		}
		var old string
		if err := json.Unmarshal(value, &old); err != nil {
			return err
		}
		updated := old
		if legacyRoot != "" && filepath.IsAbs(old) {
			rel, err := filepath.Rel(legacyRoot, old)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				updated = filepath.Join(targetRoot, rel)
			}
		}
		if key == "renderer_binary" && updated == filepath.Join(targetRoot, "bin", "e1003-render") {
			updated = filepath.Join(targetRoot, "bin", "glance-render")
		}
		if updated != old {
			fields[key], _ = json.Marshal(updated)
		}
	}
	b, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	return storage.AtomicWrite(path, append(b, '\n'))
}
