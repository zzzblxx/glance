package protocol

import (
	"bytes"
	"encoding/json"
	"github.com/zzzblxx/glance/internal/model"
	"strings"
	"testing"
	"time"
)

func TestSnapshotPrivacyAndBounds(t *testing.T) {
	d := model.Dashboard{Privacy: true, Tasks: []model.Task{{ID: "secret-id", Title: "secret-title", State: model.ActivityRunning, Stale: true}}, Quota: &model.Quota{}}
	b, _, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("secret")) {
		t.Fatal("private data leaked")
	}
	var s Snapshot
	if err = json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Tasks) != 0 || s.Counts[0].Count != 1 || s.Counts[0].Stale != 1 {
		t.Fatalf("wrong private counts: %+v", s)
	}
	d.Privacy = false
	d.Tasks = nil
	for i := 0; i < MaxTasks+9; i++ {
		d.Tasks = append(d.Tasks, model.Task{ID: "must-not-cross-usb", Title: strings.Repeat("中", 150) + "\n\x1b", State: model.ActivityCompleted, ActivityAt: time.Now()})
	}
	s, err = Build(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tasks) != MaxTasks || s.OmittedTasks != 9 || len([]rune(s.Tasks[0].Title)) != 96 {
		t.Fatal("bounds not applied")
	}
	b, _, err = Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > MaxPayload || bytes.Contains(b, []byte("must-not-cross-usb")) {
		t.Fatal("unsafe snapshot")
	}
	d.Quota.Windows = make([]model.QuotaWindow, 65)
	if _, err = Build(d); err == nil {
		t.Fatal("accepted too many windows")
	}
	d.Quota.Windows = []model.QuotaWindow{{UsedPercent: 101}}
	if _, err = Build(d); err == nil {
		t.Fatal("accepted invalid percentage")
	}
}

func TestControlCharactersAndSemanticHash(t *testing.T) {
	if got := clean("a\n\x1bb\u202ec", 96); got != "a  b c" {
		t.Fatalf("controls retained: %q", got)
	}
	d := model.Dashboard{}
	_, a, _ := Encode(d)
	d.FrameUpdatedAt = time.Now()
	_, b, _ := Encode(d)
	if a != b {
		t.Fatal("host-only update time changed wire snapshot")
	}
}
