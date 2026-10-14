package protocol

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/zzzblxx/glance/internal/model"
)

func TestBannerThresholdsAndCredits(t *testing.T) {
	for _, tc := range []struct {
		remaining float64
		want      string
	}{
		{100, "sprint"}, {70, "sprint"}, {69.999999999, "cruise"},
		{30, "cruise"}, {29.9999999999, "snack"}, {1, "snack"},
		{0.9999999999, "sleep"}, {0, "sleep"},
	} {
		for _, weeklyOnly := range []bool{false, true} {
			for _, credits := range []int{0, 1, 2, 2147483647} {
				windows := []model.QuotaWindow{{Name: "7 DAY", UsedPercent: 100 - tc.remaining}}
				if !weeklyOnly {
					windows = []model.QuotaWindow{{Name: "7 DAY", UsedPercent: 0}, {Name: "5 HOUR", UsedPercent: 100 - tc.remaining}}
				}
				d := model.Dashboard{QuotaStatus: model.SourceStatus{State: model.SourceFresh}, Quota: &model.Quota{Windows: windows, ResetCreditsKnown: true, ResetCredits: credits}}
				s, err := Build(d)
				if err != nil {
					t.Fatal(err)
				}
				want := tc.want
				if credits > 0 {
					want = "sprint"
				}
				if s.Display.BannerID != want || s.QuotaPairs[0].BannerID != want {
					t.Fatalf("remaining %.12f, weekly %v, credits %d: got %s want %s", tc.remaining, weeklyOnly, credits, s.Display.BannerID, want)
				}
				for i, w := range s.Windows {
					if w.Used != windows[i].UsedPercent {
						t.Fatal("credits changed quota bar")
					}
				}
				raw, _, err := Encode(d)
				if err != nil {
					t.Fatal(err)
				}
				var round Snapshot
				if err = json.Unmarshal(raw, &round); err != nil || round.Windows[0].Used != s.Windows[0].Used {
					t.Fatal("fraction lost in transport", err)
				}
			}
		}
	}
}

func TestBannerUnknownStaleAndBuckets(t *testing.T) {
	d := model.Dashboard{QuotaStatus: model.SourceStatus{State: model.SourceFresh}, Quota: &model.Quota{ResetCredits: 10, Windows: []model.QuotaWindow{{Bucket: "a", Name: "7 DAY", UsedPercent: 100}, {Bucket: "b", Name: "5 HOUR", UsedPercent: 50}, {Bucket: "b", Name: "7 DAY", UsedPercent: 100}}}}
	s, err := Build(d)
	if err != nil || s.QuotaPairs[0].BannerID != "sleep" || s.QuotaPairs[1].BannerID != "cruise" {
		t.Fatal("unknown reset count or cross-bucket pairing", s, err)
	}
	d.QuotaStatus.State = model.SourceStale
	s, _ = Build(d)
	if s.QuotaPairs[0].BannerID != "sleep" {
		t.Fatal("cached quota should retain banner")
	}
	d.QuotaStatus.State = model.SourceUnavailable
	s, _ = Build(d)
	if s.Display.BannerID != "unknown" || s.QuotaPairs[1].BannerID != "unknown" {
		t.Fatal("unavailable data invented a tier")
	}
	d.QuotaStatus.State = model.SourceFresh
	d.Quota.Windows = []model.QuotaWindow{{Name: "7 DAY"}, {Name: "primary"}}
	s, _ = Build(d)
	if s.Display.BannerID != "unknown" {
		t.Fatal("ambiguous five-hour absence assumed")
	}
	d.Quota = nil
	s, _ = Build(d)
	if s.Display.BannerID != "unknown" {
		t.Fatal("missing quota treated as empty")
	}
	for _, used := range []float64{math.NaN(), math.Inf(1), -0.01, 100.01} {
		d.Quota = &model.Quota{Windows: []model.QuotaWindow{{UsedPercent: used}}}
		if _, err = Build(d); err == nil {
			t.Fatal("invalid percent accepted", used)
		}
	}
}
