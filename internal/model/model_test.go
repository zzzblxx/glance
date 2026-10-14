package model

import (
	"testing"
	"time"
)

func TestNormalizeRetentionOrderingAndPrivacy(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	dashboard := Dashboard{
		Privacy: true,
		Tasks: []Task{
			{ID: "done", Title: "private done", State: ActivityCompleted, ActivityAt: now.Add(-time.Hour)},
			{ID: "old", Title: "private old", State: ActivityFailed, ActivityAt: now.Add(-25 * time.Hour)},
			{ID: "run", Title: "private run", State: ActivityRunning, ActivityAt: now.Add(-48 * time.Hour), SeenAt: now},
			{ID: "fail", Title: "private fail", State: ActivityFailed, ActivityAt: now.Add(-2 * time.Hour)},
		},
	}

	got := dashboard.Normalize(now)
	if len(got.Tasks) != 3 {
		t.Fatalf("task count = %d, want 3", len(got.Tasks))
	}
	wantIDs := []string{"run", "fail", "done"}
	for i, want := range wantIDs {
		if got.Tasks[i].ID != want {
			t.Fatalf("task %d = %q, want %q", i, got.Tasks[i].ID, want)
		}
		if got.Tasks[i].Title != "" {
			t.Fatalf("privacy boundary retained title %q", got.Tasks[i].Title)
		}
	}
}

func TestVisibleHashIgnoresObservationTimestamps(t *testing.T) {
	now := time.Now()
	base := Dashboard{QuotaStatus: SourceStatus{State: SourceFresh, LastSuccess: now}}
	changedTime := base
	changedTime.QuotaStatus.LastSuccess = now.Add(time.Minute)
	changedTime.FrameUpdatedAt = now.Add(time.Hour)
	if base.VisibleHash() != changedTime.VisibleHash() {
		t.Fatal("observation-only timestamps changed visible hash")
	}
	changedState := base
	changedState.QuotaStatus.State = SourceStale
	if base.VisibleHash() == changedState.VisibleHash() {
		t.Fatal("visible source state did not change hash")
	}
}
