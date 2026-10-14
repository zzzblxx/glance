// Package render produces identical grayscale pixels for PNG preview and the
// E1003 transport. It accepts only the privacy-filtered domain model.
package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/zzzblxx/glance/internal/model"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Width/Height describe the upright preview; the device still consumes its
// canonical landscape framebuffer. Rotate on the host, not in the firmware.
const Width = 1404
const Height = 1872
const NativeWidth = 1872
const NativeHeight = 1404
const FrameBytes = NativeWidth * NativeHeight / 2
const TaskRows = 9 // Preview proposal, pending physical readability acceptance.

type Frame struct {
	Raw    []byte
	PNG    []byte
	SHA256 string
}
type Renderer struct {
	face  *opentype.Font
	faces map[int]font.Face
}

func New(path string) (*Renderer, error) {
	var data []byte
	var err error
	if path != "" {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read display font: %w", err)
		}
	} else {
		for _, p := range []string{"/System/Library/Fonts/Hiragino Sans GB.ttc", "/System/Library/Fonts/Supplemental/Arial Unicode.ttf"} {
			data, err = os.ReadFile(p)
			if err == nil {
				break
			}
		}
		if len(data) == 0 {
			data = goregular.TTF
		}
	}
	collection, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse display font: %w", err)
	}
	f, err := collection.Font(0)
	if err != nil {
		return nil, err
	}
	return &Renderer{face: f, faces: make(map[int]font.Face)}, nil
}
func (r *Renderer) Close() {
	for _, f := range r.faces {
		_ = f.Close()
	}
}
func (r *Renderer) font(size int) font.Face {
	if f := r.faces[size]; f != nil {
		return f
	}
	f, _ := opentype.NewFace(r.face, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	r.faces[size] = f
	return f
}
func rect(img *image.Gray, x, y, w, h int, g uint8) {
	draw.Draw(img, image.Rect(x, y, x+w, y+h), image.NewUniform(color.Gray{Y: g}), image.Point{}, draw.Src)
}
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, s)
}
func (r *Renderer) text(img *image.Gray, s string, x, y, size, width int, g uint8) {
	face := r.font(size)
	s = clean(s)
	if font.MeasureString(face, s).Ceil() > width {
		runes := []rune(s)
		for len(runes) > 0 && font.MeasureString(face, string(runes)+"…").Ceil() > width {
			runes = runes[:len(runes)-1]
		}
		s = string(runes) + "…"
	}
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.Gray{Y: g}), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}
func timeLabel(t time.Time) string {
	if t.IsZero() {
		return "未提供"
	}
	return t.Local().Format("01-02 15:04")
}
func statusLabel(s model.SourceStatus) string {
	switch s.State {
	case model.SourceFresh:
		return "已同步"
	case model.SourceStale:
		return "数据已过期"
	default:
		return "暂不可用"
	}
}
func taskLabel(t model.Task) string {
	if t.Stale {
		return "状态已过期"
	}
	switch t.State {
	case model.ActivityRunning:
		return "执行中"
	case model.ActivityFailed:
		return "失败"
	case model.ActivityInterrupted:
		return "已中断"
	default:
		return "本轮完成"
	}
}

func (r *Renderer) Render(d model.Dashboard) (Frame, error) {
	// Defence in depth: a caller cannot accidentally bypass title filtering.
	if d.Privacy {
		d.Tasks = append([]model.Task(nil), d.Tasks...)
		for i := range d.Tasks {
			d.Tasks[i].Title = ""
		}
	}
	img := image.NewGray(image.Rect(0, 0, Width, Height))
	rect(img, 0, 0, Width, Height, 255)
	const contentWidth = Width - 128
	r.text(img, "Glance", 64, 112, 58, 860, 0)
	r.text(img, d.ProviderName, 1020, 104, 25, 320, 68)
	rect(img, 64, 150, contentWidth, 4, 0)
	r.text(img, "官方额度", 64, 224, 34, 800, 0)
	r.text(img, statusLabel(d.QuotaStatus), 1050, 222, 24, 290, 85)
	taskHeadingY := 620
	if d.Quota == nil || len(d.Quota.Windows) == 0 {
		r.text(img, "等待额度数据", 64, 370, 32, contentWidth, 68)
	} else {
		count := len(d.Quota.Windows)
		columns, step := min(3, count), 260
		if count > 3 {
			columns = 2
			step = min(104, 400/((count+1)/2))
			taskHeadingY = 760
		}
		if step < 40 {
			return Frame{}, fmt.Errorf("too many quota windows for this layout: %d", count)
		}
		cardWidth := (contentWidth - 32*(columns-1)) / columns
		for i, w := range d.Quota.Windows {
			x, y := 64+(i%columns)*(cardWidth+32), 294+(i/columns)*step
			name := w.Name
			if w.Bucket != "" {
				name = w.Bucket + " · " + name
			}
			if count <= 3 {
				r.text(img, name, x, y, 28, cardWidth, 51)
				r.text(img, fmt.Sprintf("%.0f%%", w.RemainingPercent()), x, y+94, 82, cardWidth-88, 0)
				r.text(img, "剩余", x+cardWidth-74, y+88, 27, 74, 85)
				r.text(img, fmt.Sprintf("已用 %.0f%%", w.UsedPercent), x, y+136, 25, cardWidth, 68)
				r.text(img, "重置 "+timeLabel(w.ResetsAt), x, y+173, 24, cardWidth, 68)
				rect(img, x, y+196, cardWidth, 12, 221)
				rect(img, x, y+196, int(float64(cardWidth)*w.RemainingPercent()/100), 12, 34)
			} else {
				nameSize, detailSize, detailOffset := 22, 19, 26
				if step < 54 {
					nameSize, detailSize, detailOffset = 18, 16, 21
				}
				r.text(img, name, x, y, nameSize, cardWidth, 34)
				r.text(img, fmt.Sprintf("剩余 %.0f%% / 已用 %.0f%% · 重置 %s", w.RemainingPercent(), w.UsedPercent, timeLabel(w.ResetsAt)), x, y+detailOffset, detailSize, cardWidth, 0)
			}
		}
		if d.Quota.ResetCreditsKnown {
			r.text(img, fmt.Sprintf("可用重置额度  %d", d.Quota.ResetCredits), 64, taskHeadingY-64, 25, contentWidth, 51)
		}
	}
	rect(img, 64, taskHeadingY-30, contentWidth, 2, 204)
	r.text(img, "任务动态", 64, taskHeadingY+28, 34, contentWidth, 0)
	subtitle := "最近 24 小时 · " + statusLabel(d.ActivityStatus)
	if d.Privacy {
		subtitle += " · 隐私模式"
	}
	r.text(img, subtitle, 64, taskHeadingY+76, 24, contentWidth, 85)
	firstTaskY := taskHeadingY + 140
	rowStep := (1660 - firstTaskY) / TaskRows
	if d.Privacy {
		states := []model.ActivityState{model.ActivityRunning, model.ActivityCompleted, model.ActivityFailed, model.ActivityInterrupted}
		for i, state := range states {
			n := 0
			stale := 0
			for _, t := range d.Tasks {
				if t.State == state {
					n++
					if t.Stale {
						stale++
					}
				}
			}
			y := firstTaskY + i*180
			r.text(img, taskLabel(model.Task{State: state}), 64, y, 30, 900, 0)
			r.text(img, fmt.Sprintf("%d", n), 1120, y+30, 70, 220, 0)
			if stale > 0 {
				r.text(img, fmt.Sprintf("其中 %d 项状态已过期", stale), 64, y+54, 23, 950, 85)
			}
			rect(img, 64, y+102, contentWidth, 2, 221)
		}
	} else if len(d.Tasks) == 0 {
		r.text(img, "暂无可验证的近期任务活动", 64, firstTaskY+32, 32, contentWidth, 68)
	} else {
		for i, t := range d.Tasks[:min(len(d.Tasks), TaskRows)] {
			y := firstTaskY + i*rowStep
			if t.State == model.ActivityRunning && !t.Stale {
				rect(img, 64, y-24, 8, 68, 0)
			}
			r.text(img, taskLabel(t), 92, y, 23, 400, 51)
			r.text(img, timeLabel(t.ActivityAt), 1070, y, 22, 270, 85)
			r.text(img, t.Title, 92, y+43, 30, contentWidth-28, 0)
			rect(img, 92, y+60, contentWidth-28, 1, 221)
		}
		if len(d.Tasks) > TaskRows {
			r.text(img, fmt.Sprintf("另有 %d 项 · 执行中、失败、中断优先", len(d.Tasks)-TaskRows), 64, 1696, 24, contentWidth, 68)
		}
	}
	rect(img, 64, 1740, contentWidth, 2, 0)
	footer := "本机采集 · 仅在可见内容变化时更新"
	if d.QuotaStatus.State != model.SourceFresh {
		footer = "额度最后成功：" + timeLabel(d.QuotaStatus.LastSuccess)
	}
	r.text(img, footer, 64, 1788, 22, 680, 68)
	activityFooter := "E1003 · 1404 × 1872 · 竖屏"
	if d.ActivityStatus.State != model.SourceFresh {
		activityFooter = "任务最后成功：" + timeLabel(d.ActivityStatus.LastSuccess)
	}
	r.text(img, activityFooter, 780, 1788, 22, 560, 68)
	r.text(img, "任务状态反映最近一轮执行，不代表整个任务已验收完成。", 64, 1838, 20, contentWidth, 102)
	return Encode(img)
}

// Encode quantizes the upright portrait preview and rotates it clockwise into
// the panel-native buffer. Logical (x,y) maps to native (1871-y,x), matching
// the pinned Seeed driver's rotation 1. Raw packing stays v1-compatible.
func Encode(img *image.Gray) (Frame, error) {
	if img.Bounds() != image.Rect(0, 0, Width, Height) {
		return Frame{}, fmt.Errorf("frame must be %dx%d", Width, Height)
	}
	raw := make([]byte, FrameBytes)
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			gray := uint8((int(img.GrayAt(x, y).Y) + 8) / 17)
			nativeX, nativeY := NativeWidth-1-y, x
			offset := (nativeY*NativeWidth + nativeX) / 2
			if nativeX%2 == 0 {
				raw[offset] |= gray << 4
			} else {
				raw[offset] |= gray
			}
			img.SetGray(x, y, color.Gray{Y: gray * 17})
		}
	}
	var p bytes.Buffer
	if err := png.Encode(&p, img); err != nil {
		return Frame{}, err
	}
	hash := sha256.Sum256(raw)
	return Frame{Raw: raw, PNG: p.Bytes(), SHA256: hex.EncodeToString(hash[:])}, nil
}

// DecodeNative presents device-rendered pixels upright without a second layout.
func DecodeNative(raw []byte) (Frame, error) {
	if len(raw) != FrameBytes {
		return Frame{}, fmt.Errorf("invalid native frame length")
	}
	img := image.NewGray(image.Rect(0, 0, Width, Height))
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			nx, ny := NativeWidth-1-y, x
			v := raw[(ny*NativeWidth+nx)/2]
			if nx%2 == 0 {
				v >>= 4
			} else {
				v &= 15
			}
			img.SetGray(x, y, color.Gray{Y: v * 17})
		}
	}
	var p bytes.Buffer
	if err := png.Encode(&p, img); err != nil {
		return Frame{}, err
	}
	hash := sha256.Sum256(raw)
	return Frame{Raw: raw, PNG: p.Bytes(), SHA256: hex.EncodeToString(hash[:])}, nil
}
