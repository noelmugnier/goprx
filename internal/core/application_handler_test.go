package core

import (
	"context"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestApplicationHandler(t *testing.T) {
	logger := slog.Default()
	slog.SetLogLoggerLevel(slog.LevelError)

	endpointUrl := "http://localhost/simple-query"

	t.Run("should forward back cookies from upstream response", func(t *testing.T) {
		t.Parallel()

		// arrange
		sb := createServiceBalancer(handlerWritingResponseCookie(), true, logger)
		handler := CreateApplicationHandler(sb, logger)

		request := httptest.NewRequest(http.MethodGet, endpointUrl, nil)
		response := httptest.NewRecorder()

		// act
		handler(response, request)

		// assert
		assert.Equal(t, http.StatusOK, response.Code)
		cookie := response.Header().Get("Set-Cookie")
		assert.Equal(t, "cookie1=value1", cookie)
	})

	t.Run("should remove non secured headers from upstream response", func(t *testing.T) {
		t.Parallel()

		// arrange
		sb := createServiceBalancer(handlerWithNonSecuredResponseHeader(), true, logger)
		handler := CreateApplicationHandler(sb, logger)

		request := httptest.NewRequest(http.MethodGet, endpointUrl, nil)
		response := httptest.NewRecorder()

		// act
		handler(response, request)

		// assert
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "", response.Header().Get("Server"))
		assert.Equal(t, "", response.Header().Get("X-Powered-By"))
		assert.Equal(t, "", response.Header().Get("X-AspNet-Version"))
		assert.Equal(t, "", response.Header().Get("X-AspNetMvc-Version"))
		assert.Equal(t, "https://new.test.com", response.Header().Get("Location"))
		assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	})

	t.Run("should return 502 when no upstream is available", func(t *testing.T) {
		t.Parallel()

		// arrange
		sb := createServiceBalancer(handlerWithStatusCode(http.StatusGatewayTimeout), false, logger)
		handler := CreateApplicationHandler(sb, logger)

		request := httptest.NewRequest(http.MethodGet, endpointUrl, nil)
		response := httptest.NewRecorder()

		// act
		handler(response, request)

		// assert
		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

func createServiceBalancer(
	handler func(w http.ResponseWriter, r *http.Request),
	waitForAvailableService bool,
	logger *slog.Logger) *ServiceBalancer {
	sbCfg := CreateRoundRobinServiceBalancerConfig(CreateDefaultHealthCheckConfig(1), 1, 100)

	factory := CreateHttpRequestForwarderFactory(logger)
	serviceBalancer := CreateServiceBalancer(factory, sbCfg, logger)
	serviceConfig := createTestService(handler)
	ctx := context.Background()

	serviceBalancer.RegisterService(ctx, serviceConfig)

	if waitForAvailableService {
		for {
			_, err := serviceBalancer.GetAvailableService(ctx)
			if err == nil {
				break
			}
		}
	}

	return serviceBalancer
}

func createTestService(request func(w http.ResponseWriter, r *http.Request)) *ServiceConfig {
	router := http.NewServeMux()

	router.HandleFunc("/", request)

	fullUrl := httptest.NewServer(router).URL
	host, portStr, _ := net.SplitHostPort(strings.SplitAfter(fullUrl, "://")[1])
	port, _ := strconv.Atoi(portStr)
	return &ServiceConfig{Host: host, Port: port, Weight: 1}
}

func handlerWithNonSecuredResponseHeader() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Server", "TestServer")
		w.Header().Set("X-Powered-By", "Dotnet")
		w.Header().Set("X-AspNet-Version", "4.0.30319")
		w.Header().Set("X-AspNetMvc-Version", "5.2")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "https://new.test.com")

		w.WriteHeader(http.StatusOK)
	}
}

func handlerWritingResponseCookie() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "cookie1", Value: "value1"})
		w.WriteHeader(http.StatusOK)
	}
}

func handlerWithStatusCode(returnedStatusCode int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(returnedStatusCode)
	}
}