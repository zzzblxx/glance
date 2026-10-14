package companion

import (
	"context"
	"github.com/zzzblxx/glance/internal/config"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/providers"
	"testing"
	"time"
)

type fakeProvider struct{}

func (fakeProvider) Info() providers.Info { return providers.Info{ID: "test", DisplayName: "Test"} }
func (fakeProvider) ReadQuota(context.Context) (model.Quota, error) {
	return model.Quota{Windows: []model.QuotaWindow{{Name: "Weekly", DurationMinutes: 10080, UsedPercent: 20}}}, nil
}
func (fakeProvider) ReadActivity(context.Context) ([]model.Task, error) {
	return []model.Task{{ID: "private-id", State: model.ActivityRunning}}, nil
}
func (fakeProvider) Close() error { return nil }
func TestEngineCollectsThroughProvider(t *testing.T) {
	e := &Engine{provider: fakeProvider{}, cfg: config.Config{DeviceToken: "test-salt"}}
	now := time.Now()
	if !e.quota(context.Background(), now) {
		t.Fatal("quota failed")
	}
	e.activity(context.Background(), now)
	if e.state.Quota.Windows[0].UsedPercent != 20 || len(e.state.Tasks) != 1 || e.state.Tasks[0].ID == "private-id" {
		t.Fatal("provider data or privacy transform failed")
	}
}
