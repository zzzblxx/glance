package companion

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/config"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/render"
	"github.com/zzzblxx/glance/internal/server"
)

type notesRenderer struct{ fail bool }

func (r *notesRenderer) Close() {}
func (r *notesRenderer) Render(d model.Dashboard) (render.Frame, error) {
	if r.fail {
		return render.Frame{}, errors.New("render failed")
	}
	return render.Frame{SHA256: model.NotesRevision(d.Display.Notes)}, nil
}

func TestSaveNotesPublishesAndPersistsWithoutRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.WriteInitial(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDirectory = t.TempDir()
	r := &notesRenderer{}
	e := &Engine{cfg: cfg, renderer: r, Store: &server.Store{}, state: Demo(time.Now(), false)}
	if err := e.publish(time.Now()); err != nil {
		t.Fatal(err)
	}
	initial := model.NotesRevision(e.Notes())
	notes := []string{"来自管理页\n第二行", "", "第三张", "第四张"}
	if err := e.SaveNotes(context.Background(), path, notes, initial); err != nil {
		t.Fatal(err)
	}
	if model.NotesRevision(e.Store.Status().Dashboard.Display.Notes) != model.NotesRevision(notes) || e.Store.Frame().SHA256 != model.NotesRevision(notes) {
		t.Fatal("store did not update")
	}
	// A normal collection publish must not replace edited notes with startup cfg.
	if err := e.publish(time.Now()); err != nil {
		t.Fatal(err)
	}
	if model.NotesRevision(e.Notes()) != model.NotesRevision(notes) {
		t.Fatal("publish restored old notes")
	}
	reloaded, err := config.Load(path)
	if err != nil || model.NotesRevision(reloaded.Notes) != model.NotesRevision(notes) {
		t.Fatal("restart config lost notes", err)
	}
	if err := e.SaveNotes(context.Background(), path, model.DefaultNotes(), initial); !errors.Is(err, model.ErrNotesConflict) {
		t.Fatal("stale writer", err)
	}
	oldFile, _ := os.ReadFile(path)
	oldHash := e.Store.Frame().SHA256
	r.fail = true
	if err := e.SaveNotes(context.Background(), path, model.DefaultNotes(), model.NotesRevision(notes)); err == nil {
		t.Fatal("render failure accepted")
	}
	newFile, _ := os.ReadFile(path)
	if !bytes.Equal(oldFile, newFile) || e.Store.Frame().SHA256 != oldHash {
		t.Fatal("failed render committed")
	}
	r.fail = false
	if err := e.SaveNotes(context.Background(), filepath.Join(t.TempDir(), "missing.json"), model.DefaultNotes(), model.NotesRevision(notes)); err == nil || e.Store.Frame().SHA256 != oldHash {
		t.Fatal("failed persist committed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.SaveNotes(ctx, path, model.DefaultNotes(), model.NotesRevision(notes)); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled save committed", err)
	}
	// Collection, status reads and management saves can run concurrently.
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			if err := e.publish(time.Now()); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			_ = e.Notes()
			_ = e.Store.Status()
			_ = e.Store.Frame()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			next := model.FourNotes(notes)
			if i%2 == 0 {
				next[0] = "交替修改"
			}
			if err := e.SaveNotes(context.Background(), path, next, model.NotesRevision(e.Notes())); err != nil {
				t.Error(err)
			}
		}
	}()
	wg.Wait()
}
