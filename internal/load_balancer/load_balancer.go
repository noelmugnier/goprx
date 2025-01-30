package load_balancer

import (
	"context"
	"github.com/noelmugnier/goprx/internal/core"
	"log/slog"
	"net/http"
)

type LoadBalancer struct {
	applications []core.Application
	router       *http.ServeMux
	logger       *slog.Logger
}

func CreateLoadBalancer(logger *slog.Logger) *LoadBalancer {
	return &LoadBalancer{
		applications: make([]core.Application, 0),
		logger:       logger,
		router:       http.NewServeMux(),
	}
}

func (lb *LoadBalancer) MapApplication(ctx context.Context, name string, pathPrefix string, sb *core.ServiceBalancer) core.Application {
	application := CreateApplication(name, sb, lb.logger)

	lb.router.HandleFunc(pathPrefix, application.Handler)

	lb.applications = append(lb.applications, application)

	lb.logger.Log(ctx, slog.LevelInfo, "application mapped")
	return application
}