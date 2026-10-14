package protocol

import (
	"github.com/zzzblxx/glance/internal/model"
	"testing"
	"time"
)

func TestProviderPresentation(t *testing.T) {
	supported := false
	d := model.Dashboard{ProviderID: "test", ProviderName: "Other\nProvider",
		QuotaStatus: model.SourceStatus{State: model.SourceFresh},
		Quota: &model.Quota{ResetCreditsSupported: &supported, ResetCreditsKnown: true, ResetCredits: 3,
			Windows: []model.QuotaWindow{{DurationMinutes: 300, Name: "Session", UsedPercent: 80, ResetsAt: time.Now()},
				{DurationMinutes: 10080, Name: "Weekly", UsedPercent: 10}}}}
	s, err := Build(d)
	if err != nil {
		t.Fatal(err)
	}
	if s.Display.Title != "Other Provider" || s.ResetExpiry.State != "unsupported" || s.ResetCredits != nil {
		t.Fatalf("incorrect provider metadata: %+v", s)
	}
	if s.QuotaPairs[0].FiveHour == nil || s.QuotaPairs[0].Weekly == nil || s.QuotaPairs[0].BannerID != "snack" {
		t.Fatal("duration pairing or unsupported-reset scoring failed")
	}
	d.ProviderName = "Changed"
	a, _ := ContentHash(d)
	d.ProviderName = "Another"
	b, _ := ContentHash(d)
	if a == b {
		t.Fatal("title change must refresh")
	}
	d.Quota.Windows[0].DurationMinutes = 60
	d.Quota.Windows[0].Name = "5 小时"
	s, _ = Build(d)
	if s.QuotaPairs[0].FiveHour != nil {
		t.Fatal("actual duration must override legacy labels")
	}
}
