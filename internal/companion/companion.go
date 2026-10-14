package companion

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zzzblxx/glance/internal/config"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/protocol"
	"github.com/zzzblxx/glance/internal/providers"
	"github.com/zzzblxx/glance/internal/providers/openai"
	"github.com/zzzblxx/glance/internal/render"
	"github.com/zzzblxx/glance/internal/server"
	"github.com/zzzblxx/glance/internal/storage"
)

type Engine struct {
	mu       sync.Mutex
	closed   bool
	demo     bool
	cfg      config.Config
	provider providers.Provider
	renderer Renderer
	Store    *server.Store
	state    model.Dashboard
	visible  string
}

type Renderer interface {
	Render(model.Dashboard) (render.Frame, error)
	Close()
}

func NewRenderer(cfg config.Config) (Renderer, error) {
	if cfg.DeviceModel != "" && cfg.DeviceModel != "e1003" {
		return nil, errors.New("unsupported device model")
	}
	if cfg.Transport == "usb" {
		return nativeview.New(cfg.RendererBinary)
	}
	return render.New(cfg.FontPath)
}

func New(cfg config.Config) (*Engine, error) {
	if cfg.Provider != "" && cfg.Provider != "openai" {
		return nil, errors.New("unsupported provider")
	}
	return NewWithProvider(cfg, openai.New(cfg.CodexBinary, cfg.CodexHome))
}

// NewWithProvider owns p and keeps the engine independent of its acquisition API.
func NewWithProvider(cfg config.Config, p providers.Provider) (*Engine, error) {
	if p == nil {
		return nil, errors.New("provider is required")
	}
	r, err := NewRenderer(cfg)
	if err != nil {
		_ = p.Close()
		return nil, err
	}
	e := &Engine{cfg: cfg, provider: p, renderer: r, Store: &server.Store{}}
	d, err := storage.Load(filepath.Join(cfg.DataDirectory, "state.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		e.Close()
		return nil, errors.New("cannot read saved dashboard state")
	}
	if (d.ProviderID != "" && d.ProviderID != p.Info().ID) || (d.ProviderID == "" && p.Info().ID != "openai") {
		d = model.Dashboard{}
	}
	e.state = d
	e.state.Privacy = cfg.PrivacyMode
	e.state = e.state.Normalize(time.Now())
	e.state.QuotaStatus = failed(d.QuotaStatus)
	e.state.ActivityStatus = failed(d.ActivityStatus)
	if err = e.publish(time.Now()); err != nil {
		e.Close()
		return nil, err
	}
	return e, nil
}
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	if e.provider != nil {
		_ = e.provider.Close()
	}
	e.renderer.Close()
}

func NewDemo(cfg config.Config) (*Engine, error) {
	r, err := NewRenderer(cfg)
	if err != nil {
		return nil, err
	}
	e := &Engine{cfg: cfg, renderer: r, Store: &server.Store{}, state: Demo(time.Now(), cfg.PrivacyMode), demo: true}
	if err := e.publish(time.Now()); err != nil {
		e.Close()
		return nil, err
	}
	return e, nil
}

func (e *Engine) Notes() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return model.FourNotes(e.cfg.Notes)
}

// Render before committing. A failed render or config write cannot replace
// the active notes/frame. USB reads the updated immutable Store generation.
func (e *Engine) SaveNotes(ctx context.Context, path string, notes []string, revision string) error {
	if err := model.ValidateEditedNotes(notes); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("service stopping")
	}
	if revision != model.NotesRevision(e.cfg.Notes) {
		return model.ErrNotesConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	d := e.state.Normalize(now)
	d.Display = model.Display{Clock: now.Truncate(time.Minute), Notes: model.FourNotes(notes), BannerID: e.cfg.BannerID}
	d, hash, f, err := e.prepare(d, now)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := config.UpdateNotes(path, notes, revision); err != nil {
		return err
	}
	e.cfg.Notes = model.FourNotes(notes)
	e.state = d
	e.visible = hash
	e.Store.Update(d, f)
	return nil
}
func failed(previous model.SourceStatus) model.SourceStatus {
	previous.State = model.SourceStale
	if previous.LastSuccess.IsZero() {
		previous.State = model.SourceUnavailable
	}
	previous.Reason = "source_unavailable"
	return previous
}
func (e *Engine) quota(ctx context.Context, now time.Time) bool {
	q, err := e.provider.ReadQuota(ctx)
	e.mu.Lock()
	defer e.mu.Unlock()
	if err != nil {
		e.state.QuotaStatus = failed(e.state.QuotaStatus)
		return false
	}
	e.state.Quota = &q
	e.state.QuotaStatus = model.SourceStatus{State: model.SourceFresh, LastSuccess: now}
	return true
}
func (e *Engine) activity(ctx context.Context, now time.Time) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tasks, err := e.provider.ReadActivity(ctx)
	if err != nil {
		e.activityFailed()
		return
	}
	for i := range tasks {
		mac := hmac.New(sha256.New, []byte(e.cfg.DeviceToken))
		mac.Write([]byte(tasks[i].ID))
		tasks[i].ID = hex.EncodeToString(mac.Sum(nil))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Tasks = tasks
	e.state.ActivityStatus = model.SourceStatus{State: model.SourceFresh, LastSuccess: now}
}
func (e *Engine) activityFailed() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.ActivityStatus = failed(e.state.ActivityStatus)
}

// prepare does not commit state: failed rendering/persistence must not advance
// the screen timestamp or consume a pending visual update.
func (e *Engine) prepare(d model.Dashboard, now time.Time) (model.Dashboard, string, *render.Frame, error) {
	if e.provider != nil {
		info := e.provider.Info()
		d.ProviderID, d.ProviderName = info.ID, info.DisplayName
	}
	hash := d.VisibleHash()
	if e.cfg.Transport == "usb" {
		var err error
		hash, err = protocol.ContentHash(d)
		if err != nil {
			return d, "", nil, err
		}
	}
	var frame *render.Frame
	if hash == e.visible {
		d.Display.Clock = e.state.Display.Clock
	} else {
		rendered, err := e.renderer.Render(d)
		if err != nil {
			return d, "", nil, err
		}
		if rendered.SHA256 != e.Store.Frame().SHA256 {
			frame = &rendered
			d.FrameUpdatedAt = now
		}
	}
	return d, hash, frame, nil
}

func (e *Engine) publish(now time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.state.Normalize(now)
	d.Display = model.Display{Clock: now.Truncate(time.Minute), Notes: append([]string(nil), e.cfg.Notes...), BannerID: e.cfg.BannerID}
	d, hash, frame, err := e.prepare(d, now)
	if err != nil {
		return err
	}
	if !e.demo {
		if err := storage.Save(filepath.Join(e.cfg.DataDirectory, "state.json"), d); err != nil {
			return errors.New("cannot persist dashboard state")
		}
	}
	e.state, e.visible = d, hash
	e.Store.Update(d, frame)
	return nil
}
func (e *Engine) Run(ctx context.Context) error {
	nextQuota := time.Time{}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			now := time.Now()
			e.activity(ctx, now)
			if !now.Before(nextQuota) {
				delay := e.cfg.QuotaPollInterval
				if !e.quota(ctx, now) {
					delay = 30 * time.Second
				}
				nextQuota = now.Add(delay)
			}
			if err := e.publish(time.Now()); err != nil {
				return err
			}
			timer.Reset(e.cfg.ActivityPollInterval)
		}
	}
}

func Demo(now time.Time, privacy bool) model.Dashboard {
	status := model.SourceStatus{State: model.SourceFresh, LastSuccess: now}
	d := model.Dashboard{Privacy: privacy, QuotaStatus: status, ActivityStatus: status, FrameUpdatedAt: now, Quota: &model.Quota{ResetCredits: 2, ResetCreditsKnown: true, Windows: []model.QuotaWindow{{Name: "5 小时", UsedPercent: 37, ResetsAt: now.Add(3 * time.Hour)}, {Name: "7 天", UsedPercent: 21, ResetsAt: now.Add(4 * 24 * time.Hour)}}}}
	d.Quota.ResetCreditsExpireAt = now.AddDate(0, 0, 17)
	d.ProviderID, d.ProviderName = "openai", "ChatGPT"
	d.Display = model.Display{Clock: now.Truncate(time.Minute), Notes: model.DefaultNotes(), BannerID: "auto"}
	titles := []string{"实现 Go 后端与本地数据采集", "检查电子纸帧格式和刷新策略", "整理项目文档与部署说明", "修复任务活动归属问题", "验证长标题自动截断，确保不会覆盖状态与时间信息", "测试隐私模式与局域网访问边界", "准备 E1003 实机验收", "完善断网恢复测试", "校对中文字体与灰度表现", "归档历史测试记录"}
	for i, title := range titles {
		state := model.ActivityCompleted
		if i < 2 {
			state = model.ActivityRunning
		}
		if i == 2 {
			state = model.ActivityFailed
		}
		if i == 3 {
			state = model.ActivityInterrupted
		}
		at := now.Add(-time.Duration(i) * 15 * time.Minute)
		d.Tasks = append(d.Tasks, model.Task{ID: fmtID(i), Title: title, State: state, ActivityAt: at, SeenAt: now})
	}
	return d.Normalize(now)
}
func fmtID(i int) string { return string(rune('a' + i)) }
