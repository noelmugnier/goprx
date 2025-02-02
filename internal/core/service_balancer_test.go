package core

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/http"
	"testing"
)

func TestServiceBalancer(t *testing.T) {
	// arrange
	logger := slog.Default()
	slog.SetLogLoggerLevel(slog.LevelError)

	service1Cfg := createTestService(handlerWithStatusCode(http.StatusOK))
	service2Cfg := createTestService(handlerWithStatusCode(http.StatusOK))
	service3Cfg := createTestService(handlerWithStatusCode(http.StatusOK))

	healthCheckCfg := CreateDefaultHealthCheckConfig(1)

	t.Run("should dequeue in round robin way registered services", func(t *testing.T) {
		t.Parallel()

		cfg := CreateRoundRobinServiceBalancerConfig(healthCheckCfg, 1, 1)
		sb := CreateServiceBalancer(CreateHttpRequestForwarderFactory(logger), cfg, logger)

		services := make([]*Service, 0)
		svc := sb.RegisterService(context.Background(), service1Cfg)
		services = append(services, svc)
		svc = sb.RegisterService(context.Background(), service2Cfg)
		services = append(services, svc)
		svc = sb.RegisterService(context.Background(), service3Cfg)
		services = append(services, svc)

		ctx := context.Background()
		iteration := 3
		cursor := 0

		waitForAllServicesToBeAvailable(sb)

		for {
			if iteration == 0 {
				break
			}

			// act
			svc, err := sb.GetAvailableService(ctx)

			// assert
			require.NoError(t, err)
			assert.Equal(t, services[cursor].Hostname, svc.Hostname)

			// next
			cursor++
			if cursor == 3 {
				cursor = 0
				iteration--
			}

		}
	})

	t.Run("should dequeue in Weighted Round Robin way registered services", func(t *testing.T) {
		t.Parallel()

		cfg := CreateWeightedRoundRobinServiceBalancerConfig(healthCheckCfg, 1, 1)
		sb := CreateServiceBalancer(CreateHttpRequestForwarderFactory(logger), cfg, logger)

		service1Cfg.SetWeight(5)
		service2Cfg.SetWeight(2)
		service3Cfg.SetWeight(3)

		services := make([]*Service, 0)
		svc := sb.RegisterService(context.Background(), service1Cfg)
		services = append(services, svc)
		svc = sb.RegisterService(context.Background(), service2Cfg)
		services = append(services, svc)
		svc = sb.RegisterService(context.Background(), service3Cfg)
		services = append(services, svc)

		expectedIndexes := []int{0, 0, 0, 0, 0, 2, 2, 2, 1, 1}

		ctx := context.Background()

		waitForAllServicesToBeAvailable(sb)

		for _, index := range expectedIndexes {
			// act
			svc, err := sb.GetAvailableService(ctx)

			// assert
			require.NoError(t, err)
			assert.Equal(t, services[index].Hostname, svc.Hostname)
		}
	})

	t.Run("should dequeue in Interleaved Round Robin way registered services", func(t *testing.T) {
		t.Parallel()

		cfg := CreateInterleavedRoundRobinServiceBalancerConfig(healthCheckCfg, 1, 1)
		sb := CreateServiceBalancer(CreateHttpRequestForwarderFactory(logger), cfg, logger)

		service1Cfg.SetWeight(5)
		service2Cfg.SetWeight(2)
		service3Cfg.SetWeight(3)

		services := make([]*Service, 0)
		svc := sb.RegisterService(context.Background(), service1Cfg)
		services = append(services, svc)
		svc = sb.RegisterService(context.Background(), service2Cfg)
		services = append(services, svc)
		svc = sb.RegisterService(context.Background(), service3Cfg)
		services = append(services, svc)

		expectedIndexes := []int{0, 2, 1, 0, 2, 1, 0, 2, 0, 0}

		ctx := context.Background()

		waitForAllServicesToBeAvailable(sb)

		for _, index := range expectedIndexes {
			// act
			svc, err := sb.GetAvailableService(ctx)

			// assert
			require.NoError(t, err)
			assert.Equal(t, services[index].Hostname, svc.Hostname)
		}
	})
}

func waitForAllServicesToBeAvailable(sb *ServiceBalancer) {
	for {
		allServiceAvailable := true
		for _, service := range sb.Services {
			allServiceAvailable = allServiceAvailable && service.Available
		}

		if allServiceAvailable {
			break
		}
	}
}