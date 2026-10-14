package nativeview_test

import (
	"bytes"
	"encoding/json"
	"github.com/zzzblxx/glance/internal/companion"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/protocol"
	"github.com/zzzblxx/glance/internal/render"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func helper(t *testing.T) string {
	t.Helper()
	p := os.Getenv("E1003_RENDERER_TEST")
	if p == "" {
		t.Skip("set E1003_RENDERER_TEST to compiled shared renderer for integration tests")
	}
	p, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResetCreditRegion(t *testing.T) {
	r, err := nativeview.New(helper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	d := companion.Demo(time.Date(2026, 10, 12, 10, 24, 0, 0, time.Local), false)
	// Keep the banner in the same tier while checking reset-region clipping.
	d.Quota.Windows[0].UsedPercent = 10
	base, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, 1, 9, 10, 99, 100, 2147483647} {
		d.Quota.ResetCredits = count
		f, err := r.Render(d)
		if err != nil || f.SHA256 == base.SHA256 {
			t.Fatal("count not rendered", count, err)
		}
		// Only the reset region can change; large counts must not leak out.
		for x := 0; x < 1404; x++ {
			for y := 0; y < 1872; y++ {
				if x >= 816 && x <= 1340 && y >= 512 && y <= 688 {
					continue
				}
				i := (x*1872 + 1871 - y) / 2
				if base.Raw[i] != f.Raw[i] {
					t.Fatal("reset block overflow", count, x, y)
				}
			}
		}
	}
	d.Quota.ResetCredits = 2
	d.Quota.ResetCreditsExpireAt = d.Quota.ResetCreditsExpireAt.Add(time.Hour)
	f, err := r.Render(d)
	if err != nil || f.SHA256 == base.SHA256 {
		t.Fatal("expiry change not rendered", err)
	}
	d.Quota.ResetCreditsExpireAt = time.Time{}
	unknown, err := r.Render(d)
	if err != nil || unknown.SHA256 == f.SHA256 {
		t.Fatal("missing expiry not rendered", err)
	}
	d.Quota.ResetCreditsNeverExpire = true
	never, err := r.Render(d)
	if err != nil || never.SHA256 == unknown.SHA256 {
		t.Fatal("nonexpiring credits indistinguishable from unknown", err)
	}
	// Invalid metadata must fail validation rather than reach the renderer.
	s, _ := protocol.Build(d)
	s.ResetExpiry.Date = strings.Repeat("0", 11)
	invalid, _ := json.Marshal(s)
	p := helper(t)
	cmd := exec.Command(p, filepath.Join(filepath.Dir(p), "unifont16.bin"), "0")
	cmd.Stdin = bytes.NewReader(invalid)
	if err := cmd.Run(); err == nil {
		t.Fatal("invalid expiry accepted")
	}
}

func TestPresentationRegionsAndStates(t *testing.T) {
	r, err := nativeview.New(helper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	d := companion.Demo(time.Date(2026, 10, 12, 10, 24, 0, 0, time.Local), false)
	base, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	pixel := func(raw []byte, x, y int) byte {
		nx := 1871 - y
		v := raw[(x*1872+nx)/2]
		if nx%2 == 0 {
			return v >> 4
		}
		return v & 15
	}
	for y := 226; y < 480; y++ {
		// The new horizontal header rule crosses this region; a vertical
		// separator between the quota columns must still remain absent.
		want := byte(15)
		if y >= 228 && y < 231 {
			want = 0
		}
		if pixel(base.Raw, 701, y) != want || pixel(base.Raw, 702, y) != want {
			t.Fatal("old quota divider retained")
		}
	}
	for _, p := range [][2]int{{64, 512}, {767, 512}, {64, 687}, {767, 687}} {
		if pixel(base.Raw, p[0], p[1]) != 15 {
			t.Fatal("banner bounds changed")
		}
	}
	for y := 532; y < 660; y++ {
		if pixel(base.Raw, 800, y) != 15 || pixel(base.Raw, 801, y) != 15 {
			t.Fatal("reset divider retained")
		}
	}
	inkBounds := func(x0, x1 int) (int, int) {
		top, bottom := 200, 0
		for y := 20; y < 210; y++ {
			for x := x0; x < x1; x++ {
				if pixel(base.Raw, x, y) < 15 {
					top = min(top, y)
					bottom = max(bottom, y)
				}
			}
		}
		return top, bottom
	}
	titleTop, titleBottom := inkBounds(64, 784)
	clockTop, clockBottom := inkBounds(1100, 1340)
	if titleTop >= titleBottom || clockTop >= clockBottom || clockTop < titleTop-2 || clockTop > titleTop+2 || clockBottom < titleBottom-2 || clockBottom > titleBottom+2 {
		t.Fatalf("header ink misaligned: title %d..%d, clock %d..%d", titleTop, titleBottom, clockTop, clockBottom)
	}
	empty := d
	empty.Display.Notes = []string{"", "", "", ""}
	notes, err := r.Render(empty)
	if err != nil {
		t.Fatal(err)
	}
	for _, center := range [][2]int{{1037, 986}, {367, 1514}, {1037, 1514}} {
		for y := -270; y <= 260; y++ {
			for x := -318; x <= 318; x++ {
				if pixel(notes.Raw, 367+x, 986+y) != pixel(notes.Raw, center[0]+x, center[1]+y) {
					t.Fatal("note geometry/angle inconsistent", center, x, y)
				}
			}
		}
	}
	d.Display.Notes[0] = strings.Repeat("中文Codex", 20)
	changed, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	if base.SHA256 == changed.SHA256 {
		t.Fatal("text update did not change notes")
	}
	for y := 0; y < 1872; y++ {
		for x := 0; x < 1404; x++ {
			if y >= 708 && y <= 1264 && x >= 49 && x <= 685 {
				continue
			}
			if pixel(base.Raw, x, y) != pixel(changed.Raw, x, y) {
				t.Fatal("note text escaped card region", x, y)
			}
		}
	}
	d.Display.Clock = d.Display.Clock.Add(time.Minute)
	clock, err := r.Render(d)
	if err != nil || clock.SHA256 == changed.SHA256 {
		t.Fatal("clock did not change", err)
	}
	for y := 0; y < 1810; y++ {
		for x := 0; x < 1404; x++ {
			if pixel(changed.Raw, x, y) != pixel(clock.Raw, x, y) {
				t.Fatal("minute update escaped footer; calendar header must stay unchanged")
			}
		}
	}
	d.Quota.Windows = d.Quota.Windows[1:]
	missing, err := r.Render(d)
	if err != nil || missing.SHA256 == clock.SHA256 {
		t.Fatal("missing quota state did not change", err)
	}
	for _, p := range [][2]int{{100, 390}, {350, 390}, {630, 390}} {
		if pixel(missing.Raw, p[0], p[1]) != 0 {
			t.Fatal("weekly-only bar did not expand into left column")
		}
	}
	if pixel(missing.Raw, 701, 390) != 0 || pixel(missing.Raw, 1300, 390) != 15 || pixel(missing.Raw, 1338, 387) != 0 {
		t.Fatal("weekly-only progress fill/bounds incorrect")
	}
	d.QuotaStatus.State = model.SourceStale
	stale, err := r.Render(d)
	if err != nil || stale.SHA256 == missing.SHA256 {
		t.Fatal("stale status not shown", err)
	}
	// The source-status label occupies y=234..257 below the header rule.
	for y := 258; y < 490; y++ {
		for x := 64; x < 1340; x++ {
			if pixel(missing.Raw, x, y) != pixel(stale.Raw, x, y) {
				t.Fatal("stale cache changed quota layout")
			}
		}
	}
	d.Quota = nil
	d.QuotaStatus.State = model.SourceUnavailable
	if _, err = r.Render(d); err != nil {
		t.Fatal("missing quota cannot render", err)
	}
}
func TestSharedRenderer(t *testing.T) {
	p := helper(t)
	r, err := nativeview.New(p)
	if err != nil {
		t.Fatal(err)
	}
	d := companion.Demo(time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC), false)
	payload, _, err := protocol.Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("snapshot %d bytes vs native frame %d bytes", len(payload), render.FrameBytes)
	a, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.RenderPage(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if a.SHA256 == b.SHA256 || len(a.Raw) != render.FrameBytes {
		t.Fatal("paging/frame invalid")
	}
	// Fresh observation timestamps should not cause an e-paper refresh.
	d.QuotaStatus.LastSuccess = d.QuotaStatus.LastSuccess.Add(time.Minute)
	same, err := r.Render(d)
	if err != nil || same.SHA256 != a.SHA256 {
		t.Fatal("fresh timestamp changed pixels", err)
	}
	d.Privacy = true
	a, err = r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	for i := range d.Tasks {
		d.Tasks[i].Title = "PRIVATE DIFFERENT TITLE"
	}
	b, err = r.Render(d)
	if err != nil || a.SHA256 != b.SHA256 {
		t.Fatal("private titles affect device pixels", err)
	}
	d.Privacy = false
	d.Quota.Windows = make([]model.QuotaWindow, 64)
	d.Tasks = make([]model.Task, 256)
	for i := range d.Tasks {
		d.Tasks[i] = model.Task{Title: "中文测试", State: model.ActivityCompleted}
	}
	if _, err = r.RenderPage(d, 28); err != nil {
		t.Fatal("maximum snapshot failed", err)
	}
	s, _ := protocol.Build(d)
	s.Privacy = true // Intentionally leave individual tasks, forbidden by validator.
	invalid, _ := json.Marshal(s)
	cmd := exec.Command(p, filepath.Join(filepath.Dir(p), "unifont16.bin"), "0")
	cmd.Stdin = bytes.NewReader(invalid)
	if err = cmd.Run(); err == nil {
		t.Fatal("native validator accepted privacy leak")
	}
}
