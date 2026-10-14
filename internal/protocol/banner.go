package protocol

import "github.com/zzzblxx/glance/internal/model"

// Selection uses unrounded remaining quota. Each known available reset contributes
// 100 percentage points, but must never inflate the actual quota bars.
func quotaBanner(pair QuotaPair, state model.SourceState, quota *model.Quota) string {
	if state == model.SourceUnavailable {
		return "unknown"
	}
	w := pair.FiveHour
	if w == nil && pair.WeeklyOnly {
		w = pair.Weekly
	}
	if w == nil {
		return "unknown"
	}
	remaining := 100 - w.Used
	if (quota.ResetCreditsSupported == nil || *quota.ResetCreditsSupported) && quota.ResetCreditsKnown && quota.ResetCredits > 0 {
		remaining += float64(quota.ResetCredits) * 100
	}
	switch {
	case remaining >= 70:
		return "sprint"
	case remaining >= 30:
		return "cruise"
	case remaining >= 1:
		return "snack"
	default:
		return "sleep"
	}
}
