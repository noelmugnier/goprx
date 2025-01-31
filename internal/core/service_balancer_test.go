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

	service1Host, service1Port := createTestService(handlerWithStatusCode(http.StatusOK))
	service2Host, service2Port := createTestService(handlerWithStatusCode(http.StatusOK))
	service3Host, service3Port := createTestService(handlerWithStatusCode(http.StatusOK))

	sbCfg := &ServiceBalancerConfig{
		HealthCheck: &HealthCheckConfig{
			Path:         "/healthz",
			IntervalInMs: 1,
		},
		UpstreamResolutionTimeoutInMs: 1,
		UpstreamRequestTimeoutInMs:    100,
	}

	sb := CreateServiceBalancer(CreateHttpRequestForwarderFactory(logger), sbCfg, logger)

	services := make([]*Service, 0)
	svc := sb.RegisterService(context.Background(), service1Host, service1Port)
	services = append(services, svc)
	svc = sb.RegisterService(context.Background(), service2Host, service2Port)
	services = append(services, svc)
	svc = sb.RegisterService(context.Background(), service3Host, service3Port)
	services = append(services, svc)

	t.Run("should dequeue in round robin way registered services", func(t *testing.T) {
		t.Parallel()

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
			assert.Equal(t, services[cursor].host, svc.host)
			assert.Equal(t, services[cursor].port, svc.port)

			// next
			cursor++
			if cursor == 3 {
				cursor = 0
				iteration--
			}

		}
	})

	t.Run("should dequeue in round robin way registered services", func(t *testing.T) {
		t.Parallel()

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
			assert.Equal(t, services[cursor].host, svc.host)
			assert.Equal(t, services[cursor].port, svc.port)

			// next
			cursor++
			if cursor == 3 {
				cursor = 0
				iteration--
			}
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