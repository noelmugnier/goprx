package reverse_proxy

import (
	"context"
	"github.com/noelmugnier/goprx/internal/core"
	"log/slog"
	"net/http"
)

type ProxifiedApplication struct {
	logger   *slog.Logger
	sb       *core.ServiceBalancer
	Name     string
	matchers []Matcher
}

type Matcher interface {
	Match(r *http.Request) bool
}

func CreateApplication(name string, matchers []Matcher, sb *core.ServiceBalancer, logger *slog.Logger) *ProxifiedApplication {
	return &ProxifiedApplication{
		matchers: matchers,
		logger:   logger.With(slog.String("application_name", name)),
		Name:     name,
		sb:       sb,
	}
}

func (a *ProxifiedApplication) RegisterService(ctx context.Context, hostname string, port int) *core.Service {
	return a.sb.RegisterService(ctx, hostname, port)
}

func (a *ProxifiedApplication) UnregisterService(ctx context.Context, host string) error {
	return a.sb.UnregisterService(ctx, host)
}

func (a *ProxifiedApplication) Handler(w http.ResponseWriter, r *http.Request) {
	handler := core.CreateApplicationHandler(a.sb, a.logger)
	handler(w, r)
}

func (a *ProxifiedApplication) Match(r *http.Request) bool {
	for _, matcher := range a.matchers {
		if matcher.Match(r) {
			return true
		}
	}

	return false
}