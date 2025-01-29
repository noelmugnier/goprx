package reverse_proxy

import (
	"context"
	"errors"
	"github.com/noelmugnier/goprx/internal/core"
	"io"
	"log/slog"
	"net/http"
)

type ReverseProxy struct {
	applications []*core.Application
	router       *http.ServeMux
	logger       *slog.Logger
}

func CreateReverseProxy(logger *slog.Logger) *ReverseProxy {
	reverseProxy := &ReverseProxy{
		applications: make([]*core.Application, 0),
		logger:       logger,
		router:       http.NewServeMux(),
	}

	reverseProxy.router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var application *core.Application = nil
	out:
		for _, app := range reverseProxy.applications {
			for _, matcher := range app.Matchers {
				if matcher.Match(r) {
					application = app
					break out
				}
			}
		}

		if application != nil {
			ctx := r.Context()
			resp, err := application.HandleRequest(ctx, r)
			if err != nil {
				if errors.Is(err, core.ServiceUnavailable) {
					w.WriteHeader(http.StatusServiceUnavailable)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}

				_, err := w.Write([]byte(err.Error()))

				if err != nil {
					logger.Log(ctx, slog.LevelError, "an error occurred while calling application service", slog.Any("error", err))
				}

				return
			}

			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					logger.Log(ctx, slog.LevelError, "an error occurred while closing the forwarded response body")
				}
			}(resp.Body)

			writeCookiesToResponse(ctx, w, resp, logger)
			writeHeadersToResponse(ctx, w, resp, logger)

			w.WriteHeader(resp.StatusCode)

			writeBodyToResponse(ctx, w, resp, logger)

			return
		}

		logger.Log(r.Context(), slog.LevelInfo, "no matching application found")

		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte("no matching application found"))

		if err != nil {
			logger.Log(r.Context(), slog.LevelError, "cannot write error to client", slog.Any("error", err))
		}
	})

	return reverseProxy
}

func (r *ReverseProxy) MapApplication(ctx context.Context, name string, matchers []core.Matcher, lb *core.ServiceBalancer) *core.Application {
	application := core.CreateApplication(name, matchers, lb, r.logger)

	r.applications = append(r.applications, application)

	r.logger.Log(ctx, slog.LevelInfo, "application mapped")
	return application
}

func writeBodyToResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response, logger *slog.Logger) {
	logger.Log(ctx, slog.LevelDebug, "writing request's response body to response")
	_, err := io.Copy(w, resp.Body)

	if err != nil {
		logger.Log(ctx, slog.LevelError, "cannot write upstream's response to client", slog.Any("error", err))
	}
}

func writeHeadersToResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response, logger *slog.Logger) {
	for headerKey := range resp.Header {
		if headerKey == "Server" || headerKey == "X-Powered-By" || headerKey == "X-Aspnet-Version" || headerKey == "X-Aspnetmvc-Version" {
			logger.Log(ctx, slog.LevelDebug, "skipping header", slog.String("header_key", headerKey))
			continue
		}

		logger.Log(ctx, slog.LevelDebug, "writing header to the response", slog.String("header_key", headerKey), slog.String("header_value", headerKey))
		w.Header().Set(headerKey, resp.Header.Get(headerKey))
	}
}

func writeCookiesToResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response, logger *slog.Logger) {
	for _, cookie := range resp.Cookies() {
		http.SetCookie(w, cookie)
		logger.Log(ctx, slog.LevelDebug, "writing cookie to the response", slog.String("cookie_name", cookie.Name))
	}
}