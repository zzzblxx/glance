package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/model"
)

func TestFramePixelsMatchPNGAndNibbleOrder(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			img.SetGray(x, y, color.Gray{Y: uint8((x*7 + y*13) % 256)})
		}
	}
	f, err := Encode(img)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Raw) != 1314144 {
		t.Fatalf("incorrect frame size: %d", len(f.Raw))
	}
	decoded, err := png.Decode(bytes.NewReader(f.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 1404, 1872) {
		t.Fatalf("preview not portrait: %v", decoded.Bounds())
	}
	// Decode native rows independently and rotate counterclockwise to recover
	// the portrait PNG. Exercise every pixel, including all edges and nibbles.
	for ny := 0; ny < 1404; ny++ {
		for nx := 0; nx < 1872; nx++ {
			v := f.Raw[(ny*1872+nx)/2]
			if nx%2 == 0 {
				v >>= 4
			} else {
				v &= 15
			}
			x, y := ny, 1871-nx
			want := uint8(((x*7+y*13)%256 + 8) / 17 * 17)
			pixel, _, _, _ := decoded.At(x, y).RGBA()
			if uint8(pixel>>8) != want || v*17 != want {
				t.Fatalf("preview/device mismatch at portrait (%d,%d)", x, y)
			}
		}
	}
	// Quantized pixels must produce exactly the same transport hash on re-encode.
	again, err := Encode(img)
	if err != nil || again.SHA256 != f.SHA256 {
		t.Fatal("quantization is not idempotent")
	}
}

func TestRejectLandscapePreview(t *testing.T) {
	if _, err := Encode(image.NewGray(image.Rect(0, 0, 1872, 1404))); err == nil {
		t.Fatal("landscape preview accepted")
	}
}

func TestPortraitQuotaLayouts(t *testing.T) {
	r, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, count := range []int{0, 1, 2, 3, 4, 6, 19, 20} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			d := model.Dashboard{Quota: &model.Quota{ResetCreditsKnown: true}}
			for i := 0; i < count; i++ {
				d.Quota.Windows = append(d.Quota.Windows, model.QuotaWindow{Name: "5 小时", Bucket: "codex", UsedPercent: float64(i)})
			}
			if _, err := r.Render(d); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestPrivacyProducesSamePixelsForDifferentTitles(t *testing.T) {
	r, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	now := time.Now()
	d := model.Dashboard{Privacy: true, Tasks: []model.Task{{ID: "a", Title: "SECRET_A", State: model.ActivityRunning, ActivityAt: now}}}
	a, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	d.Tasks[0].Title = "SECRET_B"
	b, err := r.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	if a.SHA256 != b.SHA256 {
		t.Fatal("private title influenced rendered pixels")
	}
}
