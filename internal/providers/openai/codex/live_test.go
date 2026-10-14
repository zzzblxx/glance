package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in integration probe. No titles, identifiers, paths or raw payloads are
// printed; it performs only account/rateLimits/read and thread/list.
func TestLiveLocalCodex(t *testing.T) {
	if os.Getenv("AUD_TEST_LIVE") != "1" {
		t.Skip("set AUD_TEST_LIVE=1 to probe local authenticated Codex")
	}
	home, _ := os.UserHomeDir()
	home = filepath.Join(home, ".codex")
	c := NewClient("codex", home)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	q, err := c.ReadQuota(ctx)
	if err != nil {
		t.Fatalf("quota unavailable: %v", err)
	}
	threads, err := c.ReadThreads(ctx)
	if err != nil {
		t.Fatalf("metadata unavailable: %v", err)
	}
	o := &RolloutObserver{CodexHome: home}
	events, err := o.ReadEvents()
	if err != nil {
		t.Fatalf("lifecycle unavailable: %v", err)
	}
	tasks := CorrelateTasks(threads, events)
	t.Logf("read-only probe: quota windows=%d, metadata=%d, local lifecycles=%d, matched tasks=%d", len(q.Windows), len(threads), len(events), len(tasks))
}
