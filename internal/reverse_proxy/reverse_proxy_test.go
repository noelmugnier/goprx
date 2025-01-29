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
	endpointUrl := "http://localhost/simple-query"
	pathMatcher := CreateTestPathPrefixMatcher("/simple-query")

	t.Run("should forward when at least one matcher succeed", func(t *testing.T) {
		// arrange
		methodsMatcher := CreateTestMethodsMatcher(http.MethodPost)
		reverseProxy := createTestReverseProxy()
		reverseProxy.registerTestApplicationAndWait([]core.Matcher{pathMatcher, methodsMatcher}, handlerWithStatusCode(http.StatusOK))

		firstRequest := httptest.NewRequest(http.MethodGet, endpointUrl, nil)
		secondRequest := httptest.NewRequest(http.MethodPost, "http://localhost/another-query", nil)
		firstResponse := httptest.NewRecorder()
		secondResponse := httptest.NewRecorder()

		// act
		reverseProxy.router.ServeHTTP(firstResponse, firstRequest)
		reverseProxy.router.ServeHTTP(secondResponse, secondRequest)

		// assert
		assert.Equal(t, http.StatusOK, firstResponse.Code)
		assert.Equal(t, http.StatusOK, secondResponse.Code)
	})

	t.Run("should forward back cookies from upstream response", func(t *testing.T) {
		// arrange
		methodsMatcher := CreateTestMethodsMatcher(http.MethodPost)
		reverseProxy := createTestReverseProxy()
		reverseProxy.registerTestApplicationAndWait([]core.Matcher{pathMatcher, methodsMatcher}, handlerWritingResponseCookie())

		request := httptest.NewRequest(http.MethodGet, endpointUrl, nil)
		response := httptest.NewRecorder()

		// act
		reverseProxy.router.ServeHTTP(response, request)

		// assert
		assert.Equal(t, http.StatusOK, response.Code)
		cookie := response.Header().Get("Set-Cookie")
		assert.Equal(t, "cookie1=value1", cookie)
	})

	t.Run("should remove non secured headers from upstream response", func(t *testing.T) {
		// arrange
		methodsMatcher := CreateTestMethodsMatcher(http.MethodPost)
		reverseProxy := createTestReverseProxy()
		reverseProxy.registerTestApplicationAndWait([]core.Matcher{pathMatcher, methodsMatcher}, handlerWithNonSecuredResponseHeader())

		request := httptest.NewRequest(http.MethodGet, endpointUrl, nil)
		response := httptest.NewRecorder()

		// act
		reverseProxy.router.ServeHTTP(response, request)

		// assert
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "", response.Header().Get("Server"))
		assert.Equal(t, "", response.Header().Get("X-Powered-By"))
		assert.Equal(t, "", response.Header().Get("X-AspNet-Version"))
		assert.Equal(t, "", response.Header().Get("X-AspNetMvc-Version"))
		assert.Equal(t, "https://new.test.com", response.Header().Get("Location"))
		assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	})

	t.Run("should return 503 when no upstream available", func(t *testing.T) {
		// arrange
		methodsMatcher := CreateTestMethodsMatcher(http.MethodPost)
		reverseProxy := createTestReverseProxy()
		reverseProxy.registerTestApplicationAndNoWait([]core.Matcher{pathMatcher, methodsMatcher}, handlerWithStatusCode(http.StatusGatewayTimeout))

		request := httptest.NewRequest(http.MethodGet, endpointUrl, nil)
		response := httptest.NewRecorder()

		// act
		reverseProxy.router.ServeHTTP(response, request)

		// assert
		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	})
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
	matchers []core.Matcher,
	handler func(w http.ResponseWriter, r *http.Request)) string {

	return registerTestApp(r, matchers, handler, true)
}

func (r *ReverseProxy) registerTestApplicationAndNoWait(
	matchers []core.Matcher,
	handler func(w http.ResponseWriter, r *http.Request)) string {

	return registerTestApp(r, matchers, handler, false)
}

func registerTestApp(
	reverseProxy *ReverseProxy,
	matchers []core.Matcher,
	handler func(w http.ResponseWriter, r *http.Request),
	waitForAvailableService bool) string {
	healthCfg := &core.HealthCheckConfig{
		Path:     "/healthz",
		Interval: 1,
	}

	logger := slog.Default()
	slog.SetLogLoggerLevel(slog.LevelError)

	factory := core.CreateHttpRequestForwarderFactory(logger)
	lb := core.CreateServiceBalancer(factory, healthCfg, logger)
	serviceHost, servicePort := createTestService(handler)
	ctx := context.Background()

	app := reverseProxy.MapApplication(ctx, uuid.NewString(), matchers, lb)
	app.RegisterService(ctx, serviceHost, servicePort)

	if waitForAvailableService {
		for {
			_, err := lb.GetAvailableService(ctx)
			if err == nil {
				break
			}
		}
	}

	return fmt.Sprintf("%s:%d", serviceHost, servicePort)
}

func createTestService(request func(w http.ResponseWriter, r *http.Request)) (string, int) {
	router := http.NewServeMux()

	router.HandleFunc("/", request)

	fullUrl := httptest.NewServer(router).URL
	host, portStr, _ := net.SplitHostPort(strings.SplitAfter(fullUrl, "://")[1])
	port, _ := strconv.Atoi(portStr)
	return host, port
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

type HttpTestResponse struct {
	RequestBody    []byte
	Url            string
	Host           string
	RequestUrl     string
	Method         string
	RequestHeaders http.Header
	Cookies        []*http.Cookie
}