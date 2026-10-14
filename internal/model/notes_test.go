package model

import (
	"strings"
	"testing"
)

func TestEditedNotesValidation(t *testing.T) {
	for _, tc := range []struct {
		notes []string
		valid bool
	}{
		{[]string{"", "", "", ""}, true},
		{[]string{strings.Repeat("中", 28), "\n", "🙂", "<b>text</b>"}, true},
		{[]string{strings.Repeat("中", 29), "", "", ""}, false},
		{[]string{strings.Repeat("a", 56), strings.Repeat("中a", 16), "一二三四五六七\n一二三四五六七\n一二三四五六七\n一二三四五六七", ""}, true},
		{[]string{strings.Repeat("a", 57), "", "", ""}, false},
		{[]string{"一\n二\n三\n四\n五", "", "", ""}, false},
		{[]string{"一二三四五六七八\n一二三四五六七\n一二三四五六七八", "", "", ""}, false},
		{DefaultNotes(), true},
		{[]string{"\x00", "", "", ""}, false},
		{[]string{"\u202e", "", "", ""}, false},
		{[]string{"\xff", "", "", ""}, false},
		{[]string{"one"}, false},
		{nil, false},
	} {
		if (ValidateEditedNotes(tc.notes) == nil) != tc.valid {
			t.Fatal("unexpected validation", tc.valid)
		}
	}
	if NotesRevision([]string{"a"}) != NotesRevision([]string{"a", "", "", ""}) {
		t.Fatal("padding changed revision")
	}
}
