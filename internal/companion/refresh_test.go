package companion

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/config"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/server"
)

func TestUSBRefreshOnlyForContentOrCalendarChanges(t *testing.T) {
	helper := os.Getenv("E1003_RENDERER_TEST")
	if helper == "" {
		t.Skip("requires shared renderer")
	}
	helper, err := filepath.Abs(helper)
	if err != nil {
		t.Fatal(err)
	}
	r, err := nativeview.New(helper)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg, err := config.Default()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 14, 11, 21, 0, 0, time.Local)
	e := &Engine{cfg: cfg, renderer: r, Store: &server.Store{}, state: Demo(now, false), demo: true}
	e.state.Tasks = nil
	e.state.Quota.Windows[0].UsedPercent = 20.1
	if err := e.publish(now); err != nil {
		t.Fatal(err)
	}
	initialHash := e.Store.Frame().SHA256
	initialTime := e.state.FrameUpdatedAt
	for _, elapsed := range []time.Duration{time.Minute, 5 * time.Minute, time.Hour} {
		e.state.Quota.Windows[0].UsedPercent = 20.4
		e.state.QuotaStatus.LastSuccess = now.Add(elapsed)
		e.state.ActivityStatus.LastSuccess = now.Add(elapsed)
		if err := e.publish(now.Add(elapsed)); err != nil {
			t.Fatal(err)
		}
		if e.Store.Frame().SHA256 != initialHash || !e.state.FrameUpdatedAt.Equal(initialTime) || !e.state.Display.Clock.Equal(now) {
			t.Fatal("unchanged content advanced frame/time")
		}
		// Firmware sees the newest raw snapshot, not just the cached preview.
		f, err := r.Render(e.Store.Status().Dashboard)
		if err != nil || f.SHA256 != initialHash {
			t.Fatal("USB snapshot would trigger physical refresh", err)
		}
	}
	for _, change := range []struct {
		name  string
		apply func()
	}{
		{"integer quota", func() { e.state.Quota.Windows[0].UsedPercent = 20.6 }},
		{"notes", func() { e.cfg.Notes = []string{"修改便签", "", "", ""} }},
		{"reset count", func() { e.state.Quota.ResetCredits++ }},
		{"quota warning", func() { e.state.QuotaStatus.State = model.SourceStale }},
		{"quota recovery", func() { e.state.QuotaStatus.State = model.SourceFresh }},
		{"calendar", func() { now = now.AddDate(0, 0, 1) }},
	} {
		before := e.Store.Frame().SHA256
		change.apply()
		now = now.Add(time.Minute)
		if err := e.publish(now); err != nil {
			t.Fatal(err)
		}
		if e.Store.Frame().SHA256 == before || !e.state.Display.Clock.Equal(now) || !e.state.FrameUpdatedAt.Equal(now) {
			t.Fatal("visible change not published", change.name)
		}
	}
}
