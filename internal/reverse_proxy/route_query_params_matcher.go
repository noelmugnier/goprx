package reverse_proxy

import (
	"net/http"
	"regexp"
)

type RouteQueryParamsMatcher struct {
	params map[string]*regexp.Regexp
}

func CreateRouteQueryParamsMatcher(params map[string]string) (*RouteQueryParamsMatcher, error) {
	matcher := &RouteQueryParamsMatcher{
		params: make(map[string]*regexp.Regexp),
	}

	var err error = nil
	for paramName, paramValue := range params {
		compiledRegex, err := regexp.Compile(paramValue)
		if err != nil {
			break
		}

		matcher.params[paramName] = compiledRegex
	}

	return matcher, err
}

func (m *RouteQueryParamsMatcher) Match(r *http.Request) bool {
	match := false
	for paramName, paramRegex := range m.params {
		paramValue := r.URL.Query().Get(paramName)
		if paramValue == "" {
			break
		}

		match = paramRegex.MatchString(paramValue)
		if !match {
			break
		}
	}

	return match
}