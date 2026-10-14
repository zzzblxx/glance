package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"
	"time"
)

const EndedTaskRetention = 24 * time.Hour
const RunningFreshness = 2 * time.Minute

type SourceState string

const (
	SourceFresh       SourceState = "fresh"
	SourceStale       SourceState = "stale"
	SourceUnavailable SourceState = "unavailable"
)

type SourceStatus struct {
	State       SourceState `json:"state"`
	LastSuccess time.Time   `json:"last_success,omitempty"`
	Reason      string      `json:"reason,omitempty"`
}

type QuotaWindow struct {
	DurationMinutes int       `json:"duration_minutes,omitempty"`
	Bucket          string    `json:"bucket,omitempty"`
	Name            string    `json:"name"`
	UsedPercent     float64   `json:"used_percent"`
	ResetsAt        time.Time `json:"resets_at"`
}

func (w QuotaWindow) RemainingPercent() float64 {
	remaining := 100 - w.UsedPercent
	if remaining < 0 {
		return 0
	}
	if remaining > 100 {
		return 100
	}
	return remaining
}

type Quota struct {
	// Nil preserves older cached data; false means the provider has no reset feature.
	ResetCreditsSupported     *bool         `json:"reset_credits_supported,omitempty"`
	Windows                   []QuotaWindow `json:"windows"`
	ResetCredits              int           `json:"reset_credits,omitempty"`
	ResetCreditsKnown         bool          `json:"reset_credits_known"`
	ResetCreditsExpireAt      time.Time     `json:"reset_credits_expire_at,omitempty"`
	ResetCreditsNeverExpire   bool          `json:"reset_credits_never_expire,omitempty"`
	ResetCreditsExpiryPartial bool          `json:"reset_credits_expiry_partial,omitempty"`
}

type ActivityState string

const (
	ActivityRunning     ActivityState = "running"
	ActivityCompleted   ActivityState = "completed"
	ActivityFailed      ActivityState = "failed"
	ActivityInterrupted ActivityState = "interrupted"
)

func (s ActivityState) Label() string {
	switch s {
	case ActivityRunning:
		return "RUNNING"
	case ActivityCompleted:
		return "COMPLETED"
	case ActivityFailed:
		return "FAILED"
	case ActivityInterrupted:
		return "INTERRUPTED"
	default:
		return "UNKNOWN"
	}
}

func (s ActivityState) priority() int {
	switch s {
	case ActivityRunning:
		return 0
	case ActivityFailed:
		return 1
	case ActivityInterrupted:
		return 2
	default:
		return 3
	}
}

type Task struct {
	ID         string        `json:"id"`
	Title      string        `json:"title,omitempty"`
	State      ActivityState `json:"state"`
	ActivityAt time.Time     `json:"activity_at"`
	SeenAt     time.Time     `json:"seen_at"`
	Stale      bool          `json:"stale,omitempty"`
}

type Dashboard struct {
	ProviderID     string       `json:"provider_id,omitempty"`
	ProviderName   string       `json:"provider_name,omitempty"`
	Display        Display      `json:"display"`
	Quota          *Quota       `json:"quota,omitempty"`
	QuotaStatus    SourceStatus `json:"quota_status"`
	Tasks          []Task       `json:"tasks,omitempty"`
	ActivityStatus SourceStatus `json:"activity_status"`
	Privacy        bool         `json:"privacy"`
	FrameUpdatedAt time.Time    `json:"frame_updated_at"`
}

// Display contains user-authored notes and a host-supplied minute clock. No images
// cross USB; BannerID is the selection mode (auto; legacy placeholder is an alias).
type Display struct {
	Clock    time.Time `json:"clock"`
	Notes    []string  `json:"notes"`
	BannerID string    `json:"banner_id"`
}

func DefaultNotes() []string {
	return []string{"写点代码，\n让想法落地。", "多用 Codex，\n把重复的工作\n交给 AI。", "保持专注，\n做更有创造力\n的事。", "今天也要\n比昨天更进一步\n:)"}
}

// Normalize enforces retention, ordering, and the privacy boundary before any
// renderer or transport can observe task titles.
func (d Dashboard) Normalize(now time.Time) Dashboard {
	out := d
	out.Display.Notes = append([]string(nil), d.Display.Notes...)
	out.Tasks = append([]Task(nil), d.Tasks...)
	cutoff := now.Add(-EndedTaskRetention)
	out.Tasks = slices.DeleteFunc(out.Tasks, func(task Task) bool {
		return task.ActivityAt.Before(cutoff) && task.SeenAt.Before(cutoff)
	})
	for i := range out.Tasks {
		if out.Tasks[i].State == ActivityRunning {
			seen := out.Tasks[i].SeenAt
			if seen.IsZero() {
				seen = out.Tasks[i].ActivityAt
			}
			out.Tasks[i].Stale = now.Sub(seen) > RunningFreshness || d.ActivityStatus.State != SourceFresh
		}
	}
	sort.SliceStable(out.Tasks, func(i, j int) bool {
		left, right := out.Tasks[i], out.Tasks[j]
		if left.State.priority() != right.State.priority() {
			return left.State.priority() < right.State.priority()
		}
		if left.ActivityAt.Equal(right.ActivityAt) {
			return left.ID < right.ID
		}
		return left.ActivityAt.After(right.ActivityAt)
	})
	if out.Privacy {
		for i := range out.Tasks {
			out.Tasks[i].Title = ""
		}
	}
	if out.Quota != nil {
		quota := *out.Quota
		quota.Windows = append([]QuotaWindow(nil), out.Quota.Windows...)
		out.Quota = &quota
	}
	return out
}

// VisibleHash intentionally omits source success timestamps and frame time.
// Successful polls with unchanged user-visible data must not refresh e-paper.
// Display.Clock is deliberately included: advancing the displayed minute is a
// visible change, independent of quota/task polling timestamps.
func (d Dashboard) VisibleHash() string {
	clone := d
	clone.Tasks = append([]Task(nil), d.Tasks...)
	for i := range clone.Tasks {
		clone.Tasks[i].SeenAt = time.Time{}
	}
	clone.FrameUpdatedAt = time.Time{}
	if clone.QuotaStatus.State == SourceFresh {
		clone.QuotaStatus.LastSuccess = time.Time{}
	}
	if clone.ActivityStatus.State == SourceFresh {
		clone.ActivityStatus.LastSuccess = time.Time{}
	}
	encoded, _ := json.Marshal(clone)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
