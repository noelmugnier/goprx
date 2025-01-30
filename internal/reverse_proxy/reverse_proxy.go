package reverse_proxy

import (
	"context"
	"fmt"
	"github.com/noelmugnier/goprx/internal/core"
	"log/slog"
	"net/http"
)

type ReverseProxy struct {
	applications []*ProxifiedApplication
	router       *http.ServeMux
	logger       *slog.Logger
}

func CreateReverseProxy(logger *slog.Logger) *ReverseProxy {
	reverseProxy := &ReverseProxy{
		applications: make([]*ProxifiedApplication, 0),
		logger:       logger,
		router:       http.NewServeMux(),
	}

	reverseProxy.router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		application, err := reverseProxy.getMatchingApplication(r)

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte("no matching application found"))

			if err != nil {
				logger.Log(r.Context(), slog.LevelError, "cannot write error to client", slog.Any("error", err))
			}

			return
		}

		application.Handler(w, r)
	})

	return reverseProxy
}

func (r *ReverseProxy) MapApplication(ctx context.Context, name string, matchers []Matcher, lb *core.ServiceBalancer) *ProxifiedApplication {
	application := CreateApplication(name, matchers, lb, r.logger)

	r.applications = append(r.applications, application)

	r.logger.Log(ctx, slog.LevelInfo, "application mapped")
	return application
}

func (r *ReverseProxy) getMatchingApplication(req *http.Request) (*ProxifiedApplication, error) {
	for _, app := range r.applications {
		if app.Match(req) {
			return app, nil
		}
	}

	err := fmt.Errorf("no matching application found")
	r.logger.Log(req.Context(), slog.LevelInfo, err.Error())

	return nil, err
}