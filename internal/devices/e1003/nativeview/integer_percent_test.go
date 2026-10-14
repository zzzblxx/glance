package nativeview_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/companion"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
)

func TestIntegerPercentageFrames(t *testing.T) {
	r, err := nativeview.New(helper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, weeklyOnly := range []bool{false, true} {
		d := companion.Demo(time.Date(2026, 10, 14, 11, 21, 0, 0, time.Local), false)
		if weeklyOnly {
			d.Quota.Windows = d.Quota.Windows[1:]
		}
		for _, page := range []int{0, 1} {
			for i := range d.Quota.Windows {
				d.Quota.Windows[i].UsedPercent = 20.1
			}
			base, err := r.RenderPage(d, page)
			if err != nil {
				t.Fatal(err)
			}
			for i := range d.Quota.Windows {
				d.Quota.Windows[i].UsedPercent = 20.4
			}
			same, err := r.RenderPage(d, page)
			if err != nil || !bytes.Equal(base.Raw, same.Raw) {
				t.Fatalf("sub-integer change moved pixels: weekly=%v page=%d err=%v", weeklyOnly, page, err)
			}
			for i := range d.Quota.Windows {
				d.Quota.Windows[i].UsedPercent = 20.6
			}
			changed, err := r.RenderPage(d, page)
			if err != nil || bytes.Equal(base.Raw, changed.Raw) {
				t.Fatalf("integer change not rendered: weekly=%v page=%d err=%v", weeklyOnly, page, err)
			}
		}
	}
}
