package reverse_proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/noelmugnier/goprx/internal/core"
	"github.com/stretchr/testify/assert"
	"io"
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
	pathMatcher := CreateTestPathPrefixMatcher("/simple-query")
	methodsMatcher := CreateTestMethodsMatcher(http.MethodPost)
	matchers := []Matcher{pathMatcher, methodsMatcher}

	testCases := []struct {
		Name, Method, Url  string
		ExpectedStatusCode int
	}{
		{"should forward when at least the path matcher succeed", http.MethodGet, "http://localhost/simple-query", http.StatusOK},
		{"should forward when at least the method matcher succeed", http.MethodPost, "http://localhost/another-query", http.StatusOK},
		{"should not forward when no matcher succeed", http.MethodGet, "http://localhost/another-query", http.StatusNotFound},
	}

	for _, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			// arrange
			reverseProxy := createTestReverseProxy()
			reverseProxy.registerTestApplicationAndWait(matchers, handlerWithStatusCode(http.StatusOK))

			request := httptest.NewRequest(test.Method, test.Url, nil)
			response := httptest.NewRecorder()

			// act
			reverseProxy.router.ServeHTTP(response, request)

			// assert
			assert.Equal(t, test.ExpectedStatusCode, response.Code)
		})
	}
}

func createTestReverseProxy() *ReverseProxy {
	opts := &slog.HandlerOptions{
		Level: slog.LevelError,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	logger := slog.New(handler)

	return CreateReverseProxy(logger)
}

func (r *ReverseProxy) registerTestApplicationAndWait(
	matchers []Matcher,
	handler func(w http.ResponseWriter, r *http.Request)) string {

	return registerTestApp(r, matchers, handler, true)
}

func (r *ReverseProxy) registerTestApplicationAndNoWait(
	matchers []Matcher,
	handler func(w http.ResponseWriter, r *http.Request)) string {

	return registerTestApp(r, matchers, handler, false)
}

func registerTestApp(
	reverseProxy *ReverseProxy,
	matchers []Matcher,
	handler func(w http.ResponseWriter, r *http.Request),
	waitForAvailableService bool) string {
	sbCfg := core.CreateRoundRobinServiceBalancerConfig(core.CreateDefaultHealthCheckConfig(1), 1, 1)

	logger := slog.Default()
	slog.SetLogLoggerLevel(slog.LevelError)

	factory := core.CreateHttpRequestForwarderFactory(logger)
	lb := core.CreateServiceBalancer(factory, sbCfg, logger)
	serviceCfg := createTestService(handler)
	ctx := context.Background()

	app := reverseProxy.MapApplication(ctx, uuid.NewString(), matchers, lb)
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

func handlerWithRequestAsResponseContent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		body, _ := io.ReadAll(r.Body)
		response := &HttpTestResponse{
			RequestBody:    body,
			Url:            fmt.Sprintf("http://%s%s", r.Host, r.URL.RequestURI()),
			Host:           r.Host,
			RequestUrl:     r.URL.RequestURI(),
			Method:         r.Method,
			RequestHeaders: r.Header,
			Cookies:        r.Cookies(),
		}

		content, _ := json.Marshal(response)
		_, _ = w.Write(content)
	}
}

func handlerWithStatusCode(returnedStatusCode int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(returnedStatusCode)
	}
}

type HttpTestResponse struct {
	RequestBody    []byte
	Url            string
	Host           string
	RequestUrl     string
	Method         string
	RequestHeaders http.Header
	Cookies        []*http.Cookie
}