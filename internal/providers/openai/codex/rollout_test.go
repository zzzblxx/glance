package codex

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/model"
)

func TestRolloutObserverReadsOnlyLifecycleFields(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	root := filepath.Join(t.TempDir(), "sessions", "2026", "09", "16")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "rollout-original.jsonl")
	contents := "" +
		`{"type":"session_meta","payload":{"id":"thread-1","session_id":"session-1","cwd":"SECRET_PATH"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_started","started_at":1789559990,"prompt":"SECRET_PROMPT"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","content":"SECRET_RESPONSE"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete","completed_at":1789560000,"last_agent_message":"SECRET_RESULT"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	observer := RolloutObserver{CodexHome: filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path))))), Now: func() time.Time { return now }}
	events, err := observer.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want latest task lifecycle", len(events))
	}
	if events[len(events)-1].State != model.ActivityCompleted {
		t.Fatalf("last state = %q", events[len(events)-1].State)
	}
}

func TestCorrelateTasksFoldsChildIntoParent(t *testing.T) {
	now := time.Now()
	threads := []ThreadMetadata{
		{ID: "parent", SessionID: "parent-session", Title: "Parent task"},
		{ID: "child", SessionID: "child-session", Title: "Child agent", ParentThreadID: "parent"},
	}
	events := []LifecycleEvent{{ExternalID: "parent", State: model.ActivityRunning, At: now.Add(-time.Minute)}, {ExternalID: "child", State: model.ActivityRunning, At: now, SeenAt: now}}
	tasks := CorrelateTasks(threads, events)
	if len(tasks) != 1 || tasks[0].ID != "parent" || tasks[0].Title != "Parent task" || tasks[0].State != model.ActivityRunning {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestParseRolloutMarksMissingLifecycleTimestampIncompatible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout-bad.jsonl")
	contents := `{"type":"session_meta","payload":{"id":"thread-1"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_started"}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	events, incompatible, err := parseRollout(path)
	if err != nil || len(events) != 0 || !incompatible {
		t.Fatalf("events=%v incompatible=%v err=%v", events, incompatible, err)
	}
}
