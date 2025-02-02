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
	Strategy                      ServiceBalancingStrategy
}

func CreateDefaultHealthCheckConfig(intervalInMs time.Duration) *HealthCheckConfig {
	return &HealthCheckConfig{
		Path:         "/healthz",
		IntervalInMs: intervalInMs,
	}
}

func CreateRoundRobinServiceBalancerConfig(healthCheck *HealthCheckConfig, upstreamResolutionTimeoutInMs int, upstreamRequestTimeoutInMs int) *ServiceBalancerConfig {
	return &ServiceBalancerConfig{
		HealthCheck:                   healthCheck,
		UpstreamResolutionTimeoutInMs: upstreamResolutionTimeoutInMs,
		UpstreamRequestTimeoutInMs:    upstreamRequestTimeoutInMs,
		Strategy:                      &RoundRobinStrategy{},
	}
}

func CreateWeightedRoundRobinServiceBalancerConfig(healthCheck *HealthCheckConfig, upstreamResolutionTimeoutInMs int, upstreamRequestTimeoutInMs int) *ServiceBalancerConfig {
	return &ServiceBalancerConfig{
		HealthCheck:                   healthCheck,
		UpstreamResolutionTimeoutInMs: upstreamResolutionTimeoutInMs,
		UpstreamRequestTimeoutInMs:    upstreamRequestTimeoutInMs,
		Strategy:                      &WeightedRoundRobinStrategy{},
	}
}

func CreateInterleavedRoundRobinServiceBalancerConfig(healthCheck *HealthCheckConfig, upstreamResolutionTimeoutInMs int, upstreamRequestTimeoutInMs int) *ServiceBalancerConfig {
	return &ServiceBalancerConfig{
		HealthCheck:                   healthCheck,
		UpstreamResolutionTimeoutInMs: upstreamResolutionTimeoutInMs,
		UpstreamRequestTimeoutInMs:    upstreamRequestTimeoutInMs,
		Strategy:                      &InterleavedRoundRobinStrategy{},
	}
}

type ServiceBalancer struct {
	logger   *slog.Logger
	factory  *HttpRequestForwarderFactory
	Config   *ServiceBalancerConfig
	Services []*Service
}

type HealthCheckConfig struct {
	Path         string
	IntervalInMs time.Duration
}

func (sc *ServiceConfig) SetWeight(weight int) {
	sc.Weight = weight
}

type ServiceConfig struct {
	Host   string
	Port   int
	Weight int
}

func CreateRoundRobinServiceConfig(host string, port int) *ServiceConfig {
	return &ServiceConfig{
		Host:   host,
		Port:   port,
		Weight: 1,
	}
}

func CreateWeightedRoundRobinServiceConfig(host string, port int, weight int) *ServiceConfig {
	return &ServiceConfig{
		Host:   host,
		Port:   port,
		Weight: weight,
	}
}

var (
	RRStrategy     = "round_robin"
	WRRStrategy    = "weighted_round_robin"
	IRRStrategy    = "interleaved_round_robin"
	IPHashStrategy = "ip_hash"
)

func (lb *ServiceBalancer) ElectNextService() (*Service, error) {
	return lb.Config.Strategy.ElectNextService(lb.Services)
}

type RoundRobinStrategy struct {
	currentIndex int
}

type WeightedRoundRobinStrategy struct {
	currentIndex int
}

type InterleavedRoundRobinStrategy struct {
	currentIndex int
}

type ServiceBalancingStrategy interface {
	ElectNextService(services []*Service) (*Service, error)
}

func (rrs *RoundRobinStrategy) ElectNextService(services []*Service) (*Service, error) {
	nextService := services[rrs.currentIndex]
	rrs.currentIndex = (rrs.currentIndex + 1) % len(services)

	if nextService.Available {
		return nextService, nil
	}

	return nil, nil
}

func (rrs *WeightedRoundRobinStrategy) ElectNextService(services []*Service) (*Service, error) {
	return nil, errors.New("not implemented")
}

func (rrs *InterleavedRoundRobinStrategy) ElectNextService(services []*Service) (*Service, error) {
	return nil, errors.New("not implemented")
}

func CreateServiceBalancer(factory *HttpRequestForwarderFactory, cfg *ServiceBalancerConfig, logger *slog.Logger) *ServiceBalancer {
	return &ServiceBalancer{
		logger:   logger,
		Services: make([]*Service, 0),
		factory:  factory,
		Config:   cfg,
	}
}

func (lb *ServiceBalancer) RegisterService(ctx context.Context, cfg *ServiceConfig) *Service {
	service := CreateService(lb.logger, cfg)

	lb.logger.Log(ctx, slog.LevelInfo, "registering service")
	service.Start(ctx, lb.Config.HealthCheck)
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

	timeCtx, cancel := context.WithTimeout(ctx, time.Duration(lb.Config.UpstreamResolutionTimeoutInMs)*time.Millisecond)
	defer cancel()

	for {
		select {
		case <-timeCtx.Done():
			return nil, fmt.Errorf("failed to retrieve an available service within the allocated time: %w", BadGatewayErr)
		default:
			service, err := lb.ElectNextService()
			if err != nil {
				return nil, fmt.Errorf("failed to elect next available upstream service: %w", err)
			}

			if service == nil {
				lb.logger.Log(ctx, slog.LevelDebug, "no available upstream service", slog.Any("error", err))
				continue
			}

			lb.logger.Log(ctx, slog.LevelDebug, "found an available upstream service")
			return service, nil
		}
	}
}