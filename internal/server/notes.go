package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sync"
	"unicode/utf8"

	"github.com/zzzblxx/glance/internal/model"
)

type NotesEditor struct {
	mu    sync.Mutex
	token string
	read  func() []string
	save  func(context.Context, []string, string) error
}

func NewNotesEditor(read func() []string, save func(context.Context, []string, string) error) (*NotesEditor, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	return &NotesEditor{token: hex.EncodeToString(nonce[:]), read: read, save: save}, nil
}

func (e *NotesEditor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "POST" {
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		http.Error(w, "same origin required", 403)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		http.Error(w, "same origin required", 403)
		return
	}
	if r.Method == "POST" {
		// No cookies or device credential exposed. Cross-origin sites cannot read
		// the per-process token and preflights are not allowed.
		if r.Header.Get("Origin") != "http://"+r.Host || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Notes-Token")), []byte(e.token)) != 1 {
			http.Error(w, "same origin token required", 403)
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			http.Error(w, "JSON required", 415)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			http.Error(w, "request too large", 413)
			return
		}
		if !utf8.Valid(body) {
			http.Error(w, "invalid UTF-8", 400)
			return
		}
		var input struct {
			Notes    []string `json:"notes"`
			Revision string   `json:"revision"`
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || len(input.Revision) != 64 || model.ValidateEditedNotes(input.Notes) != nil {
			http.Error(w, "需要四张便签，每张最多 28 个汉字（4 行 × 7 字，英文和数字占半字宽）；手动换行也占行，不能包含控制字符。", 400)
			return
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if input.Revision != model.NotesRevision(e.read()) {
			http.Error(w, "便签已在其他页面更新，请先重新载入。", 409)
			return
		}
		if err := e.save(r.Context(), input.Notes, input.Revision); err != nil {
			if errors.Is(err, model.ErrNotesConflict) {
				http.Error(w, "便签或配置已更新，请重新载入；手动修改配置后需重启服务。", 409)
				return
			}
			http.Error(w, "保存失败，原有便签未替换。请检查本地服务和配置文件。", 500)
			return
		}
	}
	notes := model.FourNotes(e.read())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != "HEAD" {
		_ = json.NewEncoder(w).Encode(map[string]any{"notes": notes, "revision": model.NotesRevision(notes), "token": e.token})
	}
}
