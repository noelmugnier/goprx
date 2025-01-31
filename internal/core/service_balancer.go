package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

var ServiceUnavailableErr = errors.New("service unavailable")
var GatewayTimeoutErr = errors.New("gateway timed out")
var BadGatewayErr = errors.New("bad gateway")

type ServiceBalancerConfig struct {
	HealthCheck                   *HealthCheckConfig
	UpstreamResolutionTimeoutInMs int
	UpstreamRequestTimeoutInMs    int
}

type ServiceBalancer struct {
	logger       *slog.Logger
	factory      *HttpRequestForwarderFactory
	Services     []*Service
	currentIndex int
	config       *ServiceBalancerConfig
}

type HealthCheckConfig struct {
	Path         string
	IntervalInMs time.Duration
}

func CreateServiceBalancer(factory *HttpRequestForwarderFactory, cfg *ServiceBalancerConfig, logger *slog.Logger) *ServiceBalancer {
	return &ServiceBalancer{
		config:       cfg,
		logger:       logger,
		Services:     make([]*Service, 0),
		factory:      factory,
		currentIndex: 0,
	}
}

func (lb *ServiceBalancer) RegisterService(ctx context.Context, hostname string, port int) *Service {
	service := CreateService(lb.logger, hostname, port)

	lb.logger.Log(ctx, slog.LevelInfo, "registering service")
	service.Start(ctx, lb.config.HealthCheck)
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
		return nil, err
	}

	request, err := lb.factory.CreateForwardedRequestTo(req, service.Hostname)

	if err != nil {
		return nil, fmt.Errorf("failed to create forwarded request: %w", err)
	}

	lb.logger.Log(ctx, slog.LevelInfo, "forwarding request to upstream service")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to forward request to upstream service: %w", BadGatewayErr)
	}

	if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout {
		//TODO set service as unavailable for duration
	}

	return resp, err
}

func (lb *ServiceBalancer) GetAvailableService(ctx context.Context) (*Service, error) {
	lb.logger.Log(ctx, slog.LevelDebug, "retrieving an available service")

	timeCtx, cancel := context.WithTimeout(ctx, time.Duration(lb.config.UpstreamResolutionTimeoutInMs)*time.Millisecond)
	defer cancel()

	for {
		select {
		case <-timeCtx.Done():
			return nil, fmt.Errorf("failed to retrieve an available service within the allocated time: %w", BadGatewayErr)
		default:
			nextService := lb.Services[lb.currentIndex]
			lb.currentIndex = (lb.currentIndex + 1) % len(lb.Services)

			if nextService.Available {
				lb.logger.Log(ctx, slog.LevelDebug, "found an available service")
				return nextService, nil
			}
		}
	}
}