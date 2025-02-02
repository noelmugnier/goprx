package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

type Application interface {
	RegisterService(ctx context.Context, cfg *ServiceConfig) *Service
	UnregisterService(ctx context.Context, host string) error
	Handler(w http.ResponseWriter, r *http.Request)
}

func CreateApplicationHandler(sb *ServiceBalancer, logger *slog.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		resp, err := sb.HandleRequest(ctx, r)
		if err != nil {
			if errors.Is(err, ServiceUnavailableErr) {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else if errors.Is(err, BadGatewayErr) {
				w.WriteHeader(http.StatusBadGateway)
			} else if errors.Is(err, GatewayTimeoutErr) {
				w.WriteHeader(http.StatusGatewayTimeout)
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
}

func writeCookiesToResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response, logger *slog.Logger) {
	for _, cookie := range resp.Cookies() {
		http.SetCookie(w, cookie)
		logger.Log(ctx, slog.LevelDebug, "writing cookie to the response", slog.String("cookie_name", cookie.Name))
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

func writeBodyToResponse(ctx context.Context, w http.ResponseWriter, resp *http.Response, logger *slog.Logger) {
	logger.Log(ctx, slog.LevelDebug, "writing request's response body to response")
	_, err := io.Copy(w, resp.Body)

	if err != nil {
		logger.Log(ctx, slog.LevelError, "cannot write upstream's response to client", slog.Any("error", err))
	}
}