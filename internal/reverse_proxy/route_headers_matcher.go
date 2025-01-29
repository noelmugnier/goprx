package reverse_proxy

import (
	"net/http"
	"regexp"
)

type RouteHeadersMatcher struct {
	headers map[string]*regexp.Regexp
}

func CreateRouteHeadersMatcher(headers map[string]string) (*RouteHeadersMatcher, error) {
	matcher := &RouteHeadersMatcher{
		headers: make(map[string]*regexp.Regexp),
	}

	var err error = nil
	for headerName, headerValue := range headers {
		compiledRegex, err := regexp.Compile(headerValue)
		if err != nil {
			break
		}

		matcher.headers[headerName] = compiledRegex
	}

	return matcher, err
}

func (m *RouteHeadersMatcher) Match(r *http.Request) bool {
	match := false
	for headerName, headerRegex := range m.headers {
		headerValue := r.Header.Get(headerName)
		if headerValue == "" {
			break
		}

		match = headerRegex.MatchString(headerValue)
		if !match {
			break
		}
	}

	return match
}