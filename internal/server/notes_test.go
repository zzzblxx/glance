package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zzzblxx/glance/internal/model"
)

func TestNotesEditingBoundary(t *testing.T) {
	notes := model.DefaultNotes()
	writes := 0
	editor, err := NewNotesEditor(func() []string { return notes }, func(_ context.Context, n []string, revision string) error {
		notes = model.FourNotes(n)
		writes++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h := PreviewHandler(&Store{}, editor)
	response := request(h, "/api/notes", "", "")
	var state struct {
		Notes           []string
		Revision, Token string
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &state) != nil || len(state.Notes) != 4 || len(state.Token) != 64 {
		t.Fatal("initial state unavailable")
	}
	updated := []string{"新便签\n第二行", "", "<script>alert(1)</script>", "🙂"}
	body, _ := json.Marshal(map[string]any{"notes": updated, "revision": state.Revision})
	post := func(host, origin, token, media, site, body string, method string) int {
		r := httptest.NewRequest(method, host+"/api/notes", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Notes-Token", token)
		r.Header.Set("Content-Type", media)
		r.Header.Set("Sec-Fetch-Site", site)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for _, tc := range []struct {
		name, host, origin, token, media, site, body, method string
		want                                                 int
	}{
		{"no-token", "http://127.0.0.1:8888", "http://127.0.0.1:8888", "", "application/json", "", string(body), "POST", 403},
		{"wrong-origin", "http://127.0.0.1:8888", "https://evil.example", state.Token, "application/json", "", string(body), "POST", 403},
		{"wrong-port", "http://127.0.0.1:8888", "http://127.0.0.1:9999", state.Token, "application/json", "", string(body), "POST", 403},
		{"no-origin", "http://127.0.0.1:8888", "", state.Token, "application/json", "", string(body), "POST", 403},
		{"rebound", "http://evil.example", "http://evil.example", state.Token, "application/json", "", string(body), "POST", 403},
		{"cross-site", "http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "cross-site", string(body), "POST", 403},
		{"form", "http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "text/plain", "", string(body), "POST", 415},
		{"oversize", "http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "", strings.Repeat(" ", 8193), "POST", 413},
		{"trailing", "http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "", string(body) + "{}", "POST", 400},
		{"invalid-utf8", "http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "", "\xff", "POST", 400},
		{"wrong-fields", "http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "", `{"device_token":"new"}`, "POST", 400},
		{"method", "http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "", string(body), "DELETE", 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := post(tc.host, tc.origin, tc.token, tc.media, tc.site, tc.body, tc.method); got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
	if writes != 0 {
		t.Fatal("unauthorized write")
	}
	for _, note := range []string{strings.Repeat("中", 29), strings.Repeat("a", 57), "一\n二\n三\n四\n五"} {
		invalid, _ := json.Marshal(map[string]any{"notes": []string{note, "", "", ""}, "revision": state.Revision})
		if got := post("http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "same-origin", string(invalid), "POST"); got != 400 || writes != 0 {
			t.Fatal("screen overflow accepted", got, writes)
		}
	}
	if got := post("http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "same-origin", string(body), "POST"); got != 200 || writes != 1 {
		t.Fatal("save failed", got, writes)
	}
	if got := post("http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "", string(body), "POST"); got != 409 || writes != 1 {
		t.Fatal("stale writer accepted", got)
	}
	if request(DeviceHandler(&Store{}, testToken), "/api/notes", "Bearer "+testToken, "").Code != 404 {
		t.Fatal("notes exposed over device listener")
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unsafe cache/CORS")
	}
	// Internal paths/errors must not be returned to the page.
	editor.save = func(context.Context, []string, string) error { return errors.New("SECRET /private/path") }
	newBody, _ := json.Marshal(map[string]any{"notes": model.DefaultNotes(), "revision": model.NotesRevision(notes)})
	if got := post("http://127.0.0.1:8888", "http://127.0.0.1:8888", state.Token, "application/json", "", string(newBody), "POST"); got != http.StatusInternalServerError {
		t.Fatal(got)
	}
}
