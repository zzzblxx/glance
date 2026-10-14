package nativeview_test

import (
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/companion"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
)

func TestSingleLineResetLabelCenteredOnIcon(t *testing.T) {
	r, err := nativeview.New(helper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, state := range []string{"none", "never", "unknown"} {
		d := companion.Demo(time.Date(2026, 10, 13, 10, 24, 0, 0, time.Local), false)
		d.Quota.ResetCreditsExpireAt = time.Time{}
		d.Quota.ResetCreditsNeverExpire = state == "never"
		if state == "none" {
			d.Quota.ResetCredits = 0
		}
		f, err := r.Render(d)
		if err != nil {
			t.Fatal(err)
		}
		top, bottom := 700, 0
		for y := 512; y < 688; y++ {
			for x := 1006; x < 1340; x++ {
				nx := 1871 - y
				v := f.Raw[(x*1872+nx)/2]
				if nx%2 == 0 {
					v >>= 4
				} else {
					v &= 15
				}
				if v < 15 {
					top = min(top, y)
					bottom = max(bottom, y)
				}
			}
		}
		if top >= bottom || top+bottom < 1199 || top+bottom > 1201 {
			t.Fatalf("%s label ink %d..%d not centered on icon y=600", state, top, bottom)
		}
	}
}
