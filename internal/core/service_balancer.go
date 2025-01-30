package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

var ServiceUnavailable = errors.New("service unavailable")

type ServiceBalancer struct {
	healthCheckConfig *HealthCheckConfig
	logger            *slog.Logger
	factory           *HttpRequestForwarderFactory
	Services          []*Service
}

type HealthCheckConfig struct {
	Path     string
	Interval time.Duration
}

func CreateServiceBalancer(factory *HttpRequestForwarderFactory, cfg *HealthCheckConfig, logger *slog.Logger) *ServiceBalancer {
	return &ServiceBalancer{
		healthCheckConfig: cfg,
		logger:            logger,
		Services:          make([]*Service, 0),
		factory:           factory,
	}
}

func (lb *ServiceBalancer) RegisterService(ctx context.Context, hostname string, port int) *Service {
	service := CreateService(lb.logger, hostname, port)

	lb.logger.Log(ctx, slog.LevelInfo, "registering service")
	service.Start(ctx, lb.healthCheckConfig)
	lb.Services = append(lb.Services, service)
	lb.logger.Log(ctx, slog.LevelInfo, "service registered")

	return service
}

func (lb *ServiceBalancer) UnregisterService(ctx context.Context, host string) error {
	logger := lb.logger.With(slog.String("service_host", host))
	logger.Log(ctx, slog.LevelInfo, "unregistering service")

	serviceUnregistered := false
	serviceToUnregisterFound := false

	for i, serviceToUnregister := range lb.Services {
		if serviceToUnregister.Hostname != host {
			continue
		}

		serviceToUnregisterFound = true
		logger.Log(ctx, slog.LevelInfo, "stopping service")

		serviceToUnregister.Stop()
		lb.Services = append(lb.Services[:i], lb.Services[i+1:]...)
		serviceUnregistered = true

		logger.Log(ctx, slog.LevelInfo, "service stopped")
		break
	}

	if !serviceToUnregisterFound {
		return fmt.Errorf("service not found")
	}

	if !serviceUnregistered {
		return fmt.Errorf("failed to unregister service")
	}

	logger.Log(ctx, slog.LevelInfo, "service unregistered")
	return nil
}

func (lb *ServiceBalancer) HandleRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	lb.logger.Log(ctx, slog.LevelDebug, "handling request with load balancing strategy")

	service, err := lb.GetAvailableService(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ServiceUnavailable, err)
	}

	request, err := lb.factory.CreateForwardedRequestTo(req, service.Hostname)

	if err != nil {
		return nil, fmt.Errorf("failed to create forwarded request: %w", err)
	}

	lb.logger.Log(ctx, slog.LevelInfo, "forwarding request to upstream service")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout {
		//TODO set service as unavailable for duration
	}

	return resp, err
}

func (lb *ServiceBalancer) GetAvailableService(ctx context.Context) (*Service, error) {
	lb.logger.Log(ctx, slog.LevelDebug, "retrieving an available service")
	for _, service := range lb.Services {
		if service.Available {
			lb.logger.Log(ctx, slog.LevelDebug, "available service found")
			return service, nil
		}
	}

	return nil, fmt.Errorf("no available service found")
}