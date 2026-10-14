package protocol

import (
	"encoding/json"
	"github.com/zzzblxx/glance/internal/model"
	"strings"
	"testing"
	"time"
)

func TestResetExpiryPresentation(t *testing.T) {
	at := time.Date(2026, 10, 23, 4, 44, 0, 0, time.Local)
	for _, tc := range []struct {
		name  string
		quota model.Quota
		state string
	}{
		{"dated", model.Quota{ResetCreditsKnown: true, ResetCredits: 2, ResetCreditsExpireAt: at}, "dated"},
		{"partial", model.Quota{ResetCreditsKnown: true, ResetCredits: 3, ResetCreditsExpireAt: at, ResetCreditsExpiryPartial: true}, "dated"},
		{"unknown", model.Quota{ResetCreditsKnown: true, ResetCredits: 2}, "unknown"},
		{"zero", model.Quota{ResetCreditsKnown: true, ResetCredits: 0, ResetCreditsExpireAt: at}, "none"},
		{"never", model.Quota{ResetCreditsKnown: true, ResetCredits: 2, ResetCreditsNeverExpire: true}, "never"},
		{"partial-never", model.Quota{ResetCreditsKnown: true, ResetCredits: 2, ResetCreditsNeverExpire: true, ResetCreditsExpiryPartial: true}, "unknown"},
		{"unknown-count", model.Quota{ResetCreditsExpireAt: at}, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Build(model.Dashboard{Quota: &tc.quota})
			if err != nil || s.ResetExpiry.State != tc.state {
				t.Fatal(s.ResetExpiry, err)
			}
			if tc.state == "dated" && (s.ResetExpiry.Date != "2026-10-23" || s.ResetExpiry.Time != "04:44" || s.ResetExpiry.Partial != tc.quota.ResetCreditsExpiryPartial) {
				t.Fatal(s.ResetExpiry)
			}
			if tc.state != "dated" && (s.ResetExpiry.Date != "" || s.ResetExpiry.Time != "") {
				t.Fatal("invented date")
			}
			// Persisted cache keeps the expiry alongside the last-known quota.
			raw, _ := json.Marshal(tc.quota)
			var cached model.Quota
			if err := json.Unmarshal(raw, &cached); err != nil || !cached.ResetCreditsExpireAt.Equal(tc.quota.ResetCreditsExpireAt) {
				t.Fatal("expiry lost on roundtrip", err)
			}
		})
	}
}

func TestQuotaPairingWithoutInventingMissingWindows(t *testing.T) {
	five := model.QuotaWindow{Bucket: "codex", Name: "5 HOUR", UsedPercent: 37}
	week := model.QuotaWindow{Bucket: "codex", Name: "7 DAY", UsedPercent: 21}
	for _, tc := range []struct {
		name        string
		state       model.SourceState
		windows     []model.QuotaWindow
		missingFive bool
	}{
		{"limited-reversed-order", model.SourceFresh, []model.QuotaWindow{week, five}, false},
		{"weekly-only", model.SourceFresh, []model.QuotaWindow{week}, true},
		{"stale-weekly-only", model.SourceStale, []model.QuotaWindow{week}, true},
		{"failed", model.SourceUnavailable, []model.QuotaWindow{week}, true},
		{"unknown-duration", model.SourceFresh, []model.QuotaWindow{week, {Bucket: "codex", Name: "primary"}}, true},
		{"duplicate-weekly", model.SourceFresh, []model.QuotaWindow{week, week}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Build(model.Dashboard{Quota: &model.Quota{Windows: tc.windows}, QuotaStatus: model.SourceStatus{State: tc.state}})
			if err != nil {
				t.Fatal(err)
			}
			if len(s.QuotaPairs) != 1 || (s.QuotaPairs[0].FiveHour == nil) != tc.missingFive {
				t.Fatal("missing window fabricated", s.QuotaPairs)
			}
			wantWeeklyOnly := tc.name == "weekly-only" || tc.name == "stale-weekly-only"
			if s.QuotaPairs[0].WeeklyOnly != wantWeeklyOnly {
				t.Fatal("incorrect weekly-only layout", s.QuotaPairs[0])
			}
			if tc.name == "limited-reversed-order" && (s.QuotaPairs[0].FiveHour.Used != 37 || s.QuotaPairs[0].Weekly.Used != 21) {
				t.Fatal("paired by list position")
			}
		})
	}
	five.Bucket = "other"
	s, err := Build(model.Dashboard{Quota: &model.Quota{Windows: []model.QuotaWindow{week, five}}, QuotaStatus: model.SourceStatus{State: model.SourceFresh}})
	if err != nil || len(s.QuotaPairs) != 2 || s.QuotaPairs[0].FiveHour != nil || s.QuotaPairs[1].Weekly != nil {
		t.Fatal("mixed different buckets", err)
	}
	five.Bucket = strings.Repeat("x", 60) + "five"
	week.Bucket = strings.Repeat("x", 60) + "week"
	s, err = Build(model.Dashboard{Quota: &model.Quota{Windows: []model.QuotaWindow{week, five}}})
	if err != nil || len(s.QuotaPairs) != 2 {
		t.Fatal("truncated bucket labels merged distinct identities", err)
	}
	s, err = Build(model.Dashboard{})
	if err != nil || len(s.QuotaPairs) != 0 {
		t.Fatal("missing data treated as quota", err)
	}
}

func TestResetDateIncludesYear(t *testing.T) {
	d := model.Dashboard{Quota: &model.Quota{Windows: []model.QuotaWindow{{Name: "5 HOUR", ResetsAt: time.Date(2026, 10, 12, 12, 48, 0, 0, time.Local)}}}}
	s, err := Build(d)
	if err != nil || s.QuotaPairs[0].FiveHour.ResetsAt != "2026-10-12 12:48" {
		t.Fatal("reset timestamp missing year", err)
	}
	b, _, err := Encode(d)
	if err != nil || strings.Contains(string(b), "unlimited") {
		t.Fatal("obsolete infinity field retained", err)
	}
}

func TestNotesClockAndBannerBoundary(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 24, 0, 0, time.Local)
	d := model.Dashboard{Display: model.Display{Clock: now, Notes: []string{"中文\nCodex\u202e\x1b"}}}
	s, err := Build(d)
	if err != nil {
		t.Fatal(err)
	}
	if s.Display.Time != "10:24" || s.Display.Date != "2026-10-12" || len(s.Display.Notes) != 4 || s.Display.Notes[0] != "中文\nCodex  " {
		t.Fatal("incorrect display payload")
	}
	d.Display.Notes = []string{strings.Repeat("字", 161)}
	if _, err = Build(d); err == nil {
		t.Fatal("oversized note accepted")
	}
	d.Display.Notes = []string{"a", "b", "c", "d", "e"}
	if _, err = Build(d); err == nil {
		t.Fatal("fifth note accepted")
	}
	d.Display.Notes = nil
	d.Display.BannerID = "not-yet-installed"
	if _, err = Build(d); err == nil {
		t.Fatal("unknown image accepted")
	}
}

func TestMaximumPresentationPayload(t *testing.T) {
	d := model.Dashboard{Display: model.Display{Notes: []string{strings.Repeat("中", 160), strings.Repeat("中", 160), strings.Repeat("中", 160), strings.Repeat("中", 160)}}, Quota: &model.Quota{}}
	for i := 0; i < MaxWindows; i++ {
		d.Quota.Windows = append(d.Quota.Windows, model.QuotaWindow{Bucket: strings.Repeat("桶", 47) + string(rune('a'+i)), Name: strings.Repeat("窗", 48)})
	}
	for i := 0; i < MaxTasks; i++ {
		d.Tasks = append(d.Tasks, model.Task{Title: strings.Repeat("文", 96), State: model.ActivityRunning})
	}
	payload, _, err := Encode(d)
	if err != nil || len(payload) > MaxPayload {
		t.Fatal("bounded presentation exceeds wire limit", len(payload), err)
	}
}
