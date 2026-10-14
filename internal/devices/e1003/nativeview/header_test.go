package nativeview_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/companion"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
	"github.com/zzzblxx/glance/internal/protocol"
)

func TestHomeDoesNotDisplayBucketSubtitle(t *testing.T) {
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
		base, err := r.Render(d)
		if err != nil {
			t.Fatal(err)
		}
		for i := range d.Quota.Windows {
			d.Quota.Windows[i].Bucket = "codex"
		}
		s, err := protocol.Build(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.QuotaPairs) != 1 || s.QuotaPairs[0].Bucket != "codex" {
			t.Fatal("bucket must remain available for quota grouping")
		}
		named, err := r.Render(d)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(base.Raw, named.Raw) {
			t.Fatalf("bucket name changed home appearance (weekly only: %v)", weeklyOnly)
		}
	}
}

func TestHomeHeaderDivider(t *testing.T) {
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
		frame, err := r.Render(d)
		if err != nil {
			t.Fatal(err)
		}
		for y := 227; y <= 231; y++ {
			for x := 0; x < 1404; x++ {
				nx := 1871 - y
				pixel := frame.Raw[(x*1872+nx)/2]
				if nx%2 == 0 {
					pixel >>= 4
				} else {
					pixel &= 15
				}
				want := byte(15)
				if x >= 64 && x < 1340 && y >= 228 && y < 231 {
					want = 0
				}
				if pixel != want {
					t.Fatalf("divider at (%d,%d): got %d, want %d (weekly only %v)", x, y, pixel, want, weeklyOnly)
				}
			}
		}
	}
}

func TestProviderTitleAndUnsupportedResets(t *testing.T) {
	r, err := nativeview.New(helper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	d := companion.Demo(time.Now(), false)
	base, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	d.ProviderName = "Other"
	changed, err := r.Render(d)
	if err != nil || bytes.Equal(base.Raw, changed.Raw) {
		t.Fatal("title was not data-driven", err)
	}
	for x := 0; x < 1404; x++ {
		for y := 0; y < 1872; y++ {
			if x >= 64 && x < 864 && y >= 24 && y < 208 {
				continue
			}
			i := (x*1872 + 1871 - y) / 2
			if base.Raw[i] != changed.Raw[i] {
				t.Fatal("title changed pixels outside header")
			}
		}
	}
	supported := false
	d.Quota.ResetCreditsSupported = &supported
	frame, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	for x := 816; x < 1340; x++ {
		for y := 512; y < 688; y++ {
			if frame.Raw[(x*1872+1871-y)/2] != 255 {
				t.Fatal("unsupported resets must be hidden")
			}
		}
	}
}
