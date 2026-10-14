// Package device defines display-safe data, independent of pixels and transport.
package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/zzzblxx/glance/internal/model"
)

const Protocol = 2
const MaxPayload = 192 * 1024
const MaxTasks = 256
const MaxWindows = 64

// Up to one home page per bucket, plus all task/detail pages.
const MaxPages = MaxWindows + (MaxTasks+8)/9

type Source struct {
	State       model.SourceState `json:"state"`
	LastSuccess string            `json:"last_success"`
}
type Window struct {
	Bucket   string  `json:"bucket"`
	Name     string  `json:"name"`
	Used     float64 `json:"used_percent"`
	ResetsAt string  `json:"resets_at"`
}
type Task struct {
	Title string              `json:"title"`
	State model.ActivityState `json:"state"`
	At    string              `json:"at"`
	Stale bool                `json:"stale"`
}
type Count struct {
	State model.ActivityState `json:"state"`
	Count int                 `json:"count"`
	Stale int                 `json:"stale"`
}
type Snapshot struct {
	Display        Display     `json:"display"`
	QuotaPairs     []QuotaPair `json:"quota_pairs"`
	Version        int         `json:"version"`
	Privacy        bool        `json:"privacy"`
	QuotaStatus    Source      `json:"quota_status"`
	ActivityStatus Source      `json:"activity_status"`
	Windows        []Window    `json:"windows"`
	ResetCredits   *int        `json:"reset_credits"`
	ResetExpiry    ResetExpiry `json:"reset_expiry"`
	Tasks          []Task      `json:"tasks"`
	Counts         []Count     `json:"counts"`
	OmittedTasks   int         `json:"omitted_tasks"`
}

type ResetExpiry struct {
	State   string `json:"state"`
	Date    string `json:"date"`
	Time    string `json:"time"`
	Partial bool   `json:"partial"`
}

type Display struct {
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	Time     string   `json:"time"`
	Weekday  string   `json:"weekday"`
	Notes    []string `json:"notes"`
	BannerID string   `json:"banner_id"`
}
type QuotaPair struct {
	BannerID   string  `json:"banner_id"`
	Bucket     string  `json:"bucket"`
	FiveHour   *Window `json:"five_hour"`
	Weekly     *Window `json:"weekly"`
	WeeklyOnly bool    `json:"weekly_only"`
}

func windowKind(w model.QuotaWindow) int {
	if w.DurationMinutes != 0 {
		switch w.DurationMinutes {
		case 300:
			return 5
		case 10080:
			return 7
		default:
			return 0
		}
	}
	// Compatibility for snapshots cached before durations were recorded.
	switch w.Name {
	case "5 HOUR", "5 小时":
		return 5
	case "7 DAY", "7 天":
		return 7
	default:
		return 0
	}
}

func label(t time.Time) string {
	if t.IsZero() {
		return "未提供"
	}
	return t.Local().Format("01-02 15:04")
}

func resetLabel(t time.Time) string {
	if t.IsZero() {
		return "未提供"
	}
	return t.Local().Format("2006-01-02 15:04")
}
func clean(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, s)
	r := []rune(s)
	if len(r) > limit {
		return string(r[:limit-1]) + "…"
	}
	return s
}
func source(s model.SourceStatus) Source {
	state := s.State
	if state != model.SourceFresh && state != model.SourceStale {
		state = model.SourceUnavailable
	}
	return Source{State: state, LastSuccess: label(s.LastSuccess)}
}

// Build must not expose IDs, paths, credentials, raw events or private titles.
// Callers pass a normalized dashboard; privacy is also enforced here.
func Build(d model.Dashboard) (Snapshot, error) {
	s := Snapshot{Version: Protocol, Privacy: d.Privacy, QuotaStatus: source(d.QuotaStatus), ActivityStatus: source(d.ActivityStatus), Windows: []Window{}, Tasks: []Task{}, Counts: []Count{}}
	s.ResetExpiry.State = "unknown"
	s.Display = Display{Date: "---- -- --", Time: "--:--", Weekday: "---", Notes: []string{}, BannerID: "unknown"}
	s.Display.Title = clean(d.ProviderName, 16)
	if s.Display.Title == "" {
		s.Display.Title = "Glance"
	}
	if !d.Display.Clock.IsZero() {
		s.Display.Date = d.Display.Clock.Local().Format("2006-01-02")
		s.Display.Time = d.Display.Clock.Local().Format("15:04")
		s.Display.Weekday = [...]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}[d.Display.Clock.Local().Weekday()]
	}
	if len(d.Display.Notes) > 4 {
		return s, errors.New("too many notes")
	}
	for _, note := range d.Display.Notes {
		if len([]rune(note)) > 160 {
			return s, errors.New("note too long")
		}
		// Preserve deliberate line breaks, strip all other controls/bidi overrides.
		note = strings.Map(func(r rune) rune {
			if r != '\n' && (unicode.IsControl(r) || unicode.In(r, unicode.Cf)) {
				return ' '
			}
			return r
		}, note)
		s.Display.Notes = append(s.Display.Notes, note)
	}
	for len(s.Display.Notes) < 4 {
		s.Display.Notes = append(s.Display.Notes, "")
	}
	if d.Display.BannerID != "" && d.Display.BannerID != "placeholder" && d.Display.BannerID != "auto" {
		return s, errors.New("unknown banner resource")
	}
	s.QuotaPairs = []QuotaPair{}
	if d.Quota != nil {
		if len(d.Quota.Windows) > MaxWindows {
			return s, errors.New("too many quota windows for device snapshot")
		}
		for _, w := range d.Quota.Windows {
			if math.IsNaN(w.UsedPercent) || math.IsInf(w.UsedPercent, 0) || w.UsedPercent < 0 || w.UsedPercent > 100 {
				return s, errors.New("invalid quota percentage")
			}
			s.Windows = append(s.Windows, Window{clean(w.Bucket, 48), clean(w.Name, 48), w.UsedPercent, resetLabel(w.ResetsAt)})
		}
		// Pair windows by exact bucket, never by list position. Unknown durations
		// remain on the detail pages. Missing windows never imply 100% or infinity.
		buckets := []string{}
		for _, w := range d.Quota.Windows {
			found := false
			for _, bucket := range buckets {
				if bucket == w.Bucket {
					found = true
					break
				}
			}
			if !found {
				buckets = append(buckets, w.Bucket)
			}
		}
		for _, bucket := range buckets {
			pair := QuotaPair{Bucket: clean(bucket, 48)}
			windowCount := 0
			for i := range s.Windows {
				w := &s.Windows[i]
				if d.Quota.Windows[i].Bucket != bucket {
					continue
				}
				windowCount++
				switch windowKind(d.Quota.Windows[i]) {
				case 5:
					pair.FiveHour = w
				case 7:
					pair.Weekly = w
				}
			}
			// A successful snapshot containing only the weekly window uses the
			// full row. Keep that layout for stale cached data; unknown/duplicate
			// windows or an unavailable source must not imply no five-hour limit.
			pair.WeeklyOnly = windowCount == 1 && pair.Weekly != nil && pair.FiveHour == nil && s.QuotaStatus.State != model.SourceUnavailable
			pair.BannerID = quotaBanner(pair, s.QuotaStatus.State, d.Quota)
			s.QuotaPairs = append(s.QuotaPairs, pair)
		}
		if len(s.QuotaPairs) > 0 {
			s.Display.BannerID = s.QuotaPairs[0].BannerID
		}
		if d.Quota.ResetCreditsSupported != nil && !*d.Quota.ResetCreditsSupported {
			s.ResetExpiry.State = "unsupported"
		} else if d.Quota.ResetCreditsKnown {
			if d.Quota.ResetCredits < 0 {
				return s, errors.New("invalid reset credits")
			}
			v := d.Quota.ResetCredits
			s.ResetCredits = &v
			switch {
			case v == 0:
				s.ResetExpiry.State = "none"
			case !d.Quota.ResetCreditsExpireAt.IsZero():
				expires := d.Quota.ResetCreditsExpireAt.Local()
				if expires.Year() < 1 || expires.Year() > 9999 {
					return s, errors.New("invalid reset expiry")
				}
				s.ResetExpiry = ResetExpiry{State: "dated", Date: expires.Format("2006-01-02"), Time: expires.Format("15:04"), Partial: d.Quota.ResetCreditsExpiryPartial}
			case d.Quota.ResetCreditsNeverExpire && !d.Quota.ResetCreditsExpiryPartial:
				s.ResetExpiry.State = "never"
			}
		}
	}
	for _, state := range []model.ActivityState{model.ActivityRunning, model.ActivityCompleted, model.ActivityFailed, model.ActivityInterrupted} {
		c := Count{State: state}
		for _, t := range d.Tasks {
			if t.State == state {
				c.Count++
				if t.Stale {
					c.Stale++
				}
			}
		}
		s.Counts = append(s.Counts, c)
	}
	if !d.Privacy {
		for _, t := range d.Tasks[:min(len(d.Tasks), MaxTasks)] {
			s.Tasks = append(s.Tasks, Task{clean(t.Title, 96), t.State, label(t.ActivityAt), t.Stale})
		}
		s.OmittedTasks = max(0, len(d.Tasks)-MaxTasks)
	}
	return s, nil
}

// ContentHash excludes the footer's update time and fresh polling timestamps.
// Keep the calendar date, visible states and exact banner selection; quantize
// only the percentages drawn by the renderer. Build owns these window copies.
func ContentHash(d model.Dashboard) (string, error) {
	s, err := Build(d)
	if err != nil {
		return "", err
	}
	s.Display.Time = ""
	if s.QuotaStatus.State == model.SourceFresh {
		s.QuotaStatus.LastSuccess = ""
	}
	if s.ActivityStatus.State == model.SourceFresh {
		s.ActivityStatus.LastSuccess = ""
	}
	for i := range s.Windows {
		s.Windows[i].Used = 100 - math.Round(100-s.Windows[i].Used)
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func Encode(d model.Dashboard) ([]byte, string, error) {
	s, err := Build(d)
	if err != nil {
		return nil, "", err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, "", err
	}
	if len(b) > MaxPayload {
		return nil, "", errors.New("device snapshot exceeds size limit")
	}
	h := sha256.Sum256(b)
	return b, hex.EncodeToString(h[:]), nil
}
