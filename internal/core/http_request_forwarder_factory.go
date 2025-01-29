package core

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/http"
)

type HttpRequestForwarderFactory struct {
	logger *slog.Logger
}

func CreateHttpRequestForwarderFactory(logger *slog.Logger) *HttpRequestForwarderFactory {
	return &HttpRequestForwarderFactory{
		logger: logger,
	}
}

func (r *HttpRequestForwarderFactory) CreateForwardedRequestTo(req *http.Request, host string) (*http.Request, error) {
	newRequestURL := fmt.Sprintf("http://%s%s", host, req.URL.Path)

	if len(req.URL.RawQuery) > 0 {
		newRequestURL = fmt.Sprintf("%s?%s", newRequestURL, req.URL.RawQuery)
	}

	r.logger.Log(req.Context(), slog.LevelDebug, "creating new request to", slog.String("request_url", newRequestURL), slog.String("request_method", req.Method))
	newRequest, err := http.NewRequest(req.Method, newRequestURL, bufio.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	r.forwardRequestHeaders(req, newRequest)
	r.forwardRequestCookies(req, newRequest)

	return newRequest, nil
}

func (r *HttpRequestForwarderFactory) forwardRequestHeaders(req *http.Request, newRequest *http.Request) {
	for headerName := range req.Header {
		if headerName == "Cookie" {
			continue
		}

		r.logger.Log(req.Context(), slog.LevelDebug, "adding header from original request", slog.String("header_name", headerName))
		newRequest.Header.Set(headerName, req.Header.Get(headerName))
	}

	r.logger.Log(req.Context(), slog.LevelDebug, "adding X-Forwarded-* headers to the request",
		slog.String("header_x_forwarded_proto", req.URL.Scheme),
		slog.String("header_x_forwarded_host", req.Host),
		slog.String("header_x_forwarded_for", req.RemoteAddr),
	)

	newRequest.Header.Set("X-Forwarded-Host", req.Host)
	newRequest.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
	newRequest.Header.Set("X-Forwarded-For", req.RemoteAddr)
}

func (r *HttpRequestForwarderFactory) forwardRequestCookies(req *http.Request, newRequest *http.Request) {
	for _, cookie := range req.Cookies() {
		r.logger.Log(req.Context(), slog.LevelDebug, "adding cookie from original request", slog.String("cookie_name", cookie.Name))
		newRequest.AddCookie(cookie)
	}
}