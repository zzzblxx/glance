// Package openai adapts Codex app-server data to Glance's provider interface.
package openai

import (
	"context"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/providers"
	"github.com/zzzblxx/glance/internal/providers/openai/codex"
)

type Provider struct {
	client   *codex.Client
	observer *codex.RolloutObserver
}

var _ providers.Provider = (*Provider)(nil)

func New(binary, home string) *Provider {
	return &Provider{codex.NewClient(binary, home), &codex.RolloutObserver{CodexHome: home}}
}
func (*Provider) Info() providers.Info { return providers.Info{ID: "openai", DisplayName: "ChatGPT"} }
func (p *Provider) Close() error       { return p.client.Close() }
func (p *Provider) ReadQuota(ctx context.Context) (model.Quota, error) {
	return p.client.ReadQuota(ctx)
}
func (p *Provider) ReadActivity(ctx context.Context) ([]model.Task, error) {
	// Local lifecycle evidence is required before querying task titles.
	events, err := p.observer.ReadEvents()
	if err != nil {
		return nil, err
	}
	threads, err := p.client.ReadThreads(ctx)
	if err != nil {
		return nil, err
	}
	archived, err := p.client.ReadArchivedThreads(ctx)
	if err != nil {
		return nil, err
	}
	return codex.CorrelateTasks(append(threads, archived...), events), nil
}
