package companion

import (
	"bytes"
	"context"
	"github.com/zzzblxx/glance/internal/config"
	"github.com/zzzblxx/glance/internal/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestartStalePreservationPrivacyAndUnchangedFrame(t *testing.T) {
	cfg, _ := config.Default()
	cfg.Transport = "wifi" // Legacy renderer unit test; shared renderer has integration coverage.
	cfg.DataDirectory = t.TempDir()
	cfg.PrivacyMode = true
	e, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	now := time.Now()
	e.state = Demo(now, true)
	e.state.Tasks[0].Title = "SECRET_TITLE"
	if err = e.publish(now); err != nil {
		t.Fatal(err)
	}
	etag := e.Store.Frame().SHA256
	e.state.QuotaStatus.LastSuccess = now.Add(time.Second)
	if err = e.publish(now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if e.Store.Frame().SHA256 != etag {
		t.Fatal("successful polling refreshed frame")
	}
	saved, err := os.ReadFile(filepath.Join(cfg.DataDirectory, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(saved, []byte("SECRET_TITLE")) {
		t.Fatal("private title persisted")
	}
	restarted, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if restarted.state.Quota == nil || restarted.state.Quota.Windows[0].UsedPercent != 37 || restarted.state.QuotaStatus.State != model.SourceStale {
		t.Fatal("restart lost last-known quota or marked it fresh")
	}
}

func TestFailedSourcesPreserveIndependentLastGoodData(t *testing.T) {
	cfg, _ := config.Default()
	cfg.Transport = "wifi"
	cfg.DataDirectory = t.TempDir()
	cfg.CodexBinary = "/nonexistent/codex"
	cfg.CodexHome = filepath.Join(t.TempDir(), "missing")
	e, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	now := time.Now()
	e.state = Demo(now, false)
	e.quota(context.Background(), now)
	if e.state.Quota.Windows[0].UsedPercent != 37 || e.state.QuotaStatus.State != model.SourceStale || e.state.ActivityStatus.State != model.SourceFresh {
		t.Fatal("quota failure lost data or poisoned task source")
	}
	e.activity(context.Background(), now)
	if len(e.state.Tasks) == 0 || e.state.ActivityStatus.State != model.SourceStale || !e.state.ActivityStatus.LastSuccess.Equal(now) {
		t.Fatal("task failure lost last successful snapshot")
	}
}
