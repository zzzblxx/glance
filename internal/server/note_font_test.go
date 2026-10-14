package server

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func TestNoteEditorLocalHalfWidthFont(t *testing.T) {
	h := PreviewHandler(&Store{})
	for _, method := range []string{"GET", "HEAD", "POST"} {
		r := httptest.NewRequest(method, "http://127.0.0.1:8888/note-latin.ttf", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if method == "POST" {
			if w.Code != 405 {
				t.Fatal("font endpoint allows writes")
			}
			continue
		}
		if w.Code != 200 || w.Header().Get("Content-Type") != "font/ttf" {
			t.Fatal("font not served")
		}
		if method == "GET" && !bytes.Equal(w.Body.Bytes(), gomono.TTF) {
			t.Fatal("font differs")
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
	f, err := sfnt.Parse(gomono.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if f.UnitsPerEm() != 2048 {
		t.Fatal("update CSS font scaling")
	}
	for ch := rune(32); ch < 127; ch++ {
		g, err := f.GlyphIndex(nil, ch)
		if err != nil {
			t.Fatal(err)
		}
		w, err := f.GlyphAdvance(nil, g, fixed.I(2048), font.HintingNone)
		if err != nil || w.Round() != 1229 {
			t.Fatal("ASCII no longer fixed-width", ch, w, err)
		}
	}
}
