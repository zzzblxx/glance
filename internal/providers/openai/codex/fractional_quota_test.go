package codex

import (
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/protocol"
	"testing"
)

func TestFractionalQuotaSurvivesParsingAndTierSelection(t *testing.T) {
	for _, tc := range []struct{ used, want string }{
		{"30", "sprint"}, {"30.000000001", "cruise"},
		{"70", "cruise"}, {"70.0000000001", "snack"},
		{"99", "snack"}, {"99.0000000001", "sleep"},
	} {
		q, err := ParseQuota([]byte(`{"rateLimits":{"primary":{"usedPercent":` + tc.used + `,"windowDurationMins":300}}}`))
		if err != nil {
			t.Fatal(err)
		}
		s, err := protocol.Build(model.Dashboard{Quota: &q, QuotaStatus: model.SourceStatus{State: model.SourceFresh}})
		if err != nil || s.Display.BannerID != tc.want {
			t.Fatal(tc, s.Display.BannerID, err)
		}
	}
	for _, bad := range []string{"null", "-0.1", "100.1", "1e999"} {
		if _, err := ParseQuota([]byte(`{"rateLimits":{"primary":{"usedPercent":` + bad + `}}}`)); err == nil {
			t.Fatal("invalid quota accepted", bad)
		}
	}
}
