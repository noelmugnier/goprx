package core

import (
	"context"
	"log/slog"
	"net/http"
)

type Application struct {
	logger       *slog.Logger
	loadBalancer *ServiceBalancer
	Name         string
	Matchers     []Matcher
}

type Matcher interface {
	Match(r *http.Request) bool
}

func CreateApplication(name string, matchers []Matcher, lb *ServiceBalancer, logger *slog.Logger) *Application {
	return &Application{
		Matchers:     matchers,
		logger:       logger.With(slog.String("application_name", name)),
		Name:         name,
		loadBalancer: lb,
	}
}

func (a *Application) RegisterService(ctx context.Context, hostname string, port int) *Service {
	return a.loadBalancer.RegisterService(ctx, hostname, port)
}

func (a *Application) UnregisterService(ctx context.Context, host string) error {
	return a.loadBalancer.UnregisterService(ctx, host)
}

func (a *Application) HandleRequest(ctx context.Context, r *http.Request) (*http.Response, error) {
	return a.loadBalancer.HandleRequest(ctx, r)
}