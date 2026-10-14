package nativeview_test

import (
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zzzblxx/glance/internal/companion"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
	"github.com/zzzblxx/glance/internal/model"
)

// Compare every rendered banner pixel against its original JPEG, including on
// independent bucket pages. This also detects accidental scaling or cropping.
func TestBannerAssetsMatchSources(t *testing.T) {
	r, err := nativeview.New(helper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	d := companion.Demo(time.Date(2026, 10, 13, 10, 24, 0, 0, time.Local), false)
	d.Quota.ResetCredits = 0
	d.Quota.Windows = nil
	ids := []string{"sprint", "cruise", "snack", "sleep"}
	for i, used := range []float64{30, 30.000000001, 70.0000000001, 99.0000000001} {
		d.Quota.Windows = append(d.Quota.Windows, model.QuotaWindow{Bucket: ids[i], Name: "5 HOUR", UsedPercent: used})
	}
	for page, id := range ids {
		f, err := r.RenderPage(d, page)
		if err != nil {
			t.Fatal(err)
		}
		source, err := os.Open(filepath.Join("../../../../assets/banners", id+".jpg"))
		if err != nil {
			t.Fatal(err)
		}
		img, err := jpeg.Decode(source)
		source.Close()
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 176; y++ {
			for x := 0; x < 704; x++ {
				nx := 1871 - (512 + y)
				v := f.Raw[((64+x)*1872+nx)/2]
				if nx%2 == 0 {
					v >>= 4
				} else {
					v &= 15
				}
				g := color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y
				want := byte((int(g)*15 + 127) / 255)
				if v != want {
					t.Fatalf("%s pixel %d,%d: got %d want %d", id, x, y, v, want)
				}
			}
		}
	}
}
