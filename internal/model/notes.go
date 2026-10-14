package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"unicode"
	"unicode/utf8"
)

var ErrNotesConflict = errors.New("notes changed since editing started")
var ErrInvalidNotes = errors.New("four valid notes fitting four rows of seven full-width characters required")

const NoteRows = 4
const NoteRowUnits = 14 // Seven Chinese characters; ASCII occupies a half cell.

// NoteFits rejects overflow instead of saving text that the screen would elide.
// Explicit line breaks consume rows too. Non-ASCII is conservatively full-width.
func NoteFits(note string) bool {
	rows, width := 1, 0
	for _, r := range note {
		if r == '\n' {
			rows++
			width = 0
		} else {
			units := 2
			if r < 128 {
				units = 1
			}
			if width+units > NoteRowUnits {
				rows++
				width = 0
			}
			width += units
		}
		if rows > NoteRows {
			return false
		}
	}
	return true
}

func FourNotes(notes []string) []string {
	out := make([]string, 4)
	copy(out, notes)
	return out
}

func NotesRevision(notes []string) string {
	b, _ := json.Marshal(FourNotes(notes))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func ValidateEditedNotes(notes []string) error {
	if len(notes) != 4 {
		return ErrInvalidNotes
	}
	for _, note := range notes {
		if !utf8.ValidString(note) || !NoteFits(note) {
			return ErrInvalidNotes
		}
		for _, r := range note {
			if r != '\n' && (unicode.IsControl(r) || unicode.In(r, unicode.Cf)) {
				return ErrInvalidNotes
			}
		}
	}
	return nil
}
