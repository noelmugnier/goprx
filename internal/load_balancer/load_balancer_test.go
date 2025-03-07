package load_balancer

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/noelmugnier/goprx/internal/core"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestReverseProxy(t *testing.T) {
}

func createTestLoadBalancer() *LoadBalancer {
	opts := &slog.HandlerOptions{
		Level: slog.LevelError,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	logger := slog.New(handler)

	return CreateLoadBalancer(logger)
}

func (lb *LoadBalancer) registerTestApplicationAndWait(
	pathPrefix string,
	handler func(w http.ResponseWriter, r *http.Request)) string {

	return registerTestApp(lb, pathPrefix, handler, true)
}

func (lb *LoadBalancer) registerTestApplicationAndNoWait(
	pathPrefix string,
	handler func(w http.ResponseWriter, r *http.Request)) string {

	return registerTestApp(lb, pathPrefix, handler, false)
}

func registerTestApp(
	loadBalancer *LoadBalancer,
	pathPrefix string,
	handler func(w http.ResponseWriter, r *http.Request),
	waitForAvailableService bool) string {
	sbCfg := core.CreateRoundRobinServiceBalancerConfig(core.CreateDefaultHealthCheckConfig(1), 1, 1)

	logger := slog.Default()
	slog.SetLogLoggerLevel(slog.LevelError)

	factory := core.CreateHttpRequestForwarderFactory(logger)
	lb := core.CreateServiceBalancer(factory, sbCfg, logger)
	serviceCfg := createTestService(handler)
	ctx := context.Background()

	app := loadBalancer.MapApplication(ctx, uuid.NewString(), pathPrefix, lb)
	app.RegisterService(ctx, serviceCfg)

	if waitForAvailableService {
		for {
			_, err := lb.GetAvailableService(ctx)
			if err == nil {
				break
			}
		}
	}

	return fmt.Sprintf("%s:%d", serviceCfg.Host, serviceCfg.Port)
}

func createTestService(request func(w http.ResponseWriter, r *http.Request)) *core.ServiceConfig {
	router := http.NewServeMux()

	router.HandleFunc("/", request)

	fullUrl := httptest.NewServer(router).URL
	host, portStr, _ := net.SplitHostPort(strings.SplitAfter(fullUrl, "://")[1])
	port, _ := strconv.Atoi(portStr)
	return &core.ServiceConfig{Host: host, Port: port}
}