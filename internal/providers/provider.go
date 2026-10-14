// Package providers defines the data-source boundary. It does not know devices.
package providers

import (
	"context"
	"github.com/zzzblxx/glance/internal/model"
)

type Info struct {
	ID          string
	DisplayName string
}

type Provider interface {
	Info() Info
	ReadQuota(context.Context) (model.Quota, error)
	ReadActivity(context.Context) ([]model.Task, error)
	Close() error
}
