package codex

import (
	"fmt"
	"testing"
)

func TestResetCreditExpiry(t *testing.T) {
	row := func(id, status, expiry string) string {
		return fmt.Sprintf(`{"id":%q,"resetType":"codexRateLimits","status":%q,"expiresAt":%s}`, id, status, expiry)
	}
	a, b := row("a", "available", "2000000000"), row("b", "available", "2100000000")
	for _, tc := range []struct {
		name, count, rows string
		at                int64
		never, partial    bool
	}{
		{"earliest-unsorted", "2", "[" + b + "," + a + "]", 2000000000, false, false},
		{"count-only", "2", "null", 0, false, true},
		{"empty-details", "2", "[]", 0, false, true},
		{"capped-details", "3", "[" + a + "]", 2000000000, false, true},
		{"used-and-expired", "1", "[" + row("used", "consumed", "1800000000") + "," + row("old", "expired", "1700000000") + "," + a + "]", 2000000000, false, false},
		{"never-expire", "1", "[" + row("a", "available", "null") + "]", 0, true, false},
		{"partial-never", "2", "[" + row("a", "available", "null") + "]", 0, false, true},
		{"mixed-never", "2", "[" + row("a", "available", "null") + "," + b + "]", 2100000000, false, false},
		{"missing-expiry", "1", `[{"id":"a","resetType":"codexRateLimits","status":"available"}]`, 0, false, true},
		{"duplicate", "2", "[" + a + "," + a + "]", 2000000000, false, true},
		{"zero-count", "0", "[" + a + "]", 0, false, false},
		{"unknown-count", "null", "[" + a + "]", 0, false, false},
		{"invalid-expiry", "1", "[" + row("a", "available", `"bad"`) + "]", 0, false, true},
		{"negative-expiry", "1", "[" + row("a", "available", "-1") + "]", 0, false, true},
		{"overflow-expiry", "1", "[" + row("a", "available", "9223372036854775807") + "]", 0, false, true},
		{"other-type", "1", `[{"id":"a","resetType":"other","status":"available","expiresAt":2000000000}]`, 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, err := ParseQuota([]byte(`{"rateLimits":{"primary":{"usedPercent":20}},"rateLimitResetCredits":{"availableCount":` + tc.count + `,"credits":` + tc.rows + `}}`))
			if err != nil {
				t.Fatal(err)
			}
			if tc.at == 0 && !q.ResetCreditsExpireAt.IsZero() || tc.at != 0 && q.ResetCreditsExpireAt.Unix() != tc.at || q.ResetCreditsNeverExpire != tc.never || q.ResetCreditsExpiryPartial != tc.partial {
				t.Fatalf("unexpected expiry: %+v", q)
			}
		})
	}
}
