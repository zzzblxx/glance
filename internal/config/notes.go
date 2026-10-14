package config

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/storage"
)

// UpdateNotes preserves pairing, secrets, custom settings and unknown fields.
// The running companion serializes writers. External edits to notes conflict.
func UpdateNotes(path string, notes []string, revision string) error {
	if err := model.ValidateEditedNotes(notes); err != nil {
		return err
	}
	if _, err := Load(path); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return errors.New("invalid config object")
	}
	previous := model.DefaultNotes()
	if value, ok := fields["notes"]; ok {
		if err := json.Unmarshal(value, &previous); err != nil {
			return err
		}
	}
	if model.NotesRevision(previous) != revision {
		return model.ErrNotesConflict
	}
	fields["notes"], err = json.Marshal(notes)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	return storage.AtomicWrite(path, append(encoded, '\n'))
}
