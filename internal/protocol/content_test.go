package protocol

import (
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/model"
)

func TestContentHashIgnoresMinutesAndFractionalBars(t *testing.T) {
	now := time.Date(2026, 10, 14, 11, 21, 0, 0, time.Local)
	d := model.Dashboard{Display: model.Display{Clock: now}, QuotaStatus: model.SourceStatus{State: model.SourceFresh, LastSuccess: now}, ActivityStatus: model.SourceStatus{State: model.SourceFresh, LastSuccess: now}, Quota: &model.Quota{ResetCreditsKnown: true, Windows: []model.QuotaWindow{{Name: "5 HOUR", UsedPercent: 20.1}, {Name: "7 DAY", UsedPercent: 40.1}}}}
	hash := func() string {
		h, err := ContentHash(d)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := hash()
	d.Display.Clock = now.Add(time.Minute)
	d.QuotaStatus.LastSuccess = now.Add(time.Minute)
	d.ActivityStatus.LastSuccess = now.Add(time.Minute)
	d.Quota.Windows[0].UsedPercent = 20.4
	d.Quota.Windows[1].UsedPercent = 40.4
	if hash() != base {
		t.Fatal("minutes or sub-integer changes affected content")
	}
	if d.Quota.Windows[0].UsedPercent != 20.4 {
		t.Fatal("raw quota mutated")
	}
	for _, index := range []int{0, 1} {
		old := d.Quota.Windows[index].UsedPercent
		d.Quota.Windows[index].UsedPercent += 1
		if hash() == base {
			t.Fatal("integer quota change ignored", index)
		}
		d.Quota.Windows[index].UsedPercent = old
	}
	d.Display.Clock = now.AddDate(0, 0, 1)
	if hash() == base {
		t.Fatal("calendar change ignored")
	}
	s, err := Build(d)
	if err != nil || s.Display.Weekday != "星期四" {
		t.Fatal("incorrect weekday", s.Display, err)
	}
	d.Display.Clock = now
	d.QuotaStatus.State = model.SourceStale
	if hash() == base {
		t.Fatal("stale warning ignored")
	}
	stale := hash()
	d.QuotaStatus.LastSuccess = now.Add(time.Hour)
	if hash() == stale {
		t.Fatal("visible stale timestamp ignored")
	}
	d.QuotaStatus.State = model.SourceFresh
	d.Quota.Windows[0].UsedPercent = 30
	base = hash()
	d.Quota.Windows[0].UsedPercent = 30.00000001
	if hash() == base {
		t.Fatal("precise banner boundary ignored")
	}
}
