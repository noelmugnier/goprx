package load_balancer

import (
	"context"
	"github.com/noelmugnier/goprx/internal/core"
	"log/slog"
	"net/http"
)

type BalancedApplication struct {
	logger *slog.Logger
	sb     *core.ServiceBalancer
	Name   string
}

type Matcher interface {
	Match(r *http.Request) bool
}

func CreateApplication(name string, sb *core.ServiceBalancer, logger *slog.Logger) core.Application {
	return &BalancedApplication{
		logger: logger.With(slog.String("application_name", name)),
		Name:   name,
		sb:     sb,
	}
}

func (a *BalancedApplication) RegisterService(ctx context.Context, hostname string, port int) *core.Service {
	return a.sb.RegisterService(ctx, hostname, port)
}

func (a *BalancedApplication) UnregisterService(ctx context.Context, host string) error {
	return a.sb.UnregisterService(ctx, host)
}

func (a *BalancedApplication) Handler(w http.ResponseWriter, r *http.Request) {
	handler := core.CreateApplicationHandler(a.sb, a.logger)
	handler(w, r)
}