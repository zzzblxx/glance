package codex

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/model"
)

func TestQuotaCreditsOnlyKnownWhenCountProvided(t *testing.T) {
	for _, tc := range []struct {
		credits string
		known   bool
	}{
		{`null`, false},
		{`{}`, false},
		{`{"availableCount":null}`, false},
		{`{"availableCount":0}`, true},
	} {
		q, err := ParseQuota([]byte(`{"rateLimitsByLimitId":{"codex":{"primary":{"usedPercent":20}}},"rateLimitResetCredits":` + tc.credits + `}`))
		if err != nil || q.ResetCreditsKnown != tc.known {
			t.Fatalf("credits %s: known=%v, error=%v", tc.credits, q.ResetCreditsKnown, err)
		}
	}
}

func TestQuotaMultiBucketNullAndMetadata(t *testing.T) {
	raw := []byte(`{"rateLimits":{"limitId":"codex","planType":"pro","primary":{"usedPercent":99}},"rateLimitsByLimitId":{"codex":{"limitName":"Codex","primary":{"usedPercent":20,"windowDurationMins":300,"resetsAt":2000000000},"secondary":null,"credits":{"balance":"123"}},"spark":{"primary":{"usedPercent":12,"windowDurationMins":null,"resetsAt":null}}}}`)
	q, err := ParseQuota(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Windows) != 2 || q.Windows[0].UsedPercent != 20 || q.Windows[1].Bucket != "spark" || !q.Windows[1].ResetsAt.IsZero() {
		t.Fatalf("wrong multi-bucket view: %+v", q)
	}
}
func TestChildCannotFinishParentOrAliasSession(t *testing.T) {
	now := time.Now()
	threads := []ThreadMetadata{{ID: "p", SessionID: "shared", Title: "parent"}, {ID: "c", SessionID: "shared", Title: "child", ParentThreadID: "p"}}
	events := []LifecycleEvent{{ExternalID: "p", State: model.ActivityRunning, At: now}, {ExternalID: "c", State: model.ActivityCompleted, At: now.Add(time.Minute)}}
	tasks := CorrelateTasks(threads, events)
	if len(tasks) != 1 || tasks[0].State != model.ActivityRunning {
		t.Fatalf("child ended parent: %+v", tasks)
	}
}
func TestIncrementalPartialLineAndSameTimestampTerminal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sessions")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "rollout-t.jsonl")
	os.WriteFile(path, []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"p\"}}\n{\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\",\"started_at\":2000000000}}\n{\"type\":"), 0600)
	o := &RolloutObserver{CodexHome: root}
	a, err := o.ReadEvents()
	if err != nil || len(a) != 1 || a[0].State != model.ActivityRunning {
		t.Fatalf("first read: %+v %v", a, err)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString("\"event_msg\",\"payload\":{\"type\":\"task_complete\",\"completed_at\":2000000000}}\n")
	f.Close()
	b, err := o.ReadEvents()
	if err != nil || len(b) != 1 || b[0].State != model.ActivityCompleted {
		t.Fatalf("second read: %+v %v", b, err)
	}
}
