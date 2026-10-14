package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zzzblxx/glance/internal/model"
)

func TestUpdateNotesPreservesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := WriteInitial(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var before map[string]json.RawMessage
	json.Unmarshal(raw, &before)
	before["future_setting"] = json.RawMessage(`{"keep":true}`)
	before["usb_port"] = json.RawMessage(`"/dev/example"`)
	before["usb_device_id"] = json.RawMessage(`"123456abcdef"`)
	raw, _ = json.Marshal(before)
	os.WriteFile(path, raw, 0600)
	notes := []string{"一\n二", "", "三", "四"}
	if err := UpdateNotes(path, notes, model.NotesRevision(model.DefaultNotes())); err != nil {
		t.Fatal(err)
	}
	updated, err := Load(path)
	if err != nil || model.NotesRevision(updated.Notes) != model.NotesRevision(notes) {
		t.Fatal("reload failed", err)
	}
	raw, _ = os.ReadFile(path)
	var after map[string]json.RawMessage
	json.Unmarshal(raw, &after)
	for k, v := range before {
		if k == "notes" {
			continue
		}
		var b, a any
		json.Unmarshal(v, &b)
		json.Unmarshal(after[k], &a)
		bb, _ := json.Marshal(b)
		aa, _ := json.Marshal(a)
		if !bytes.Equal(bb, aa) {
			t.Fatal("changed field", k)
		}
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("permissions")
	}
	if err := UpdateNotes(path, model.DefaultNotes(), model.NotesRevision(model.DefaultNotes())); !errors.Is(err, model.ErrNotesConflict) {
		t.Fatal("conflict lost", err)
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(raw, unchanged) {
		t.Fatal("failed save changed config")
	}
}
