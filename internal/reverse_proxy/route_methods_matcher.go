package reverse_proxy

import (
	"net/http"
	"slices"
)

type RouteMethodMatcher struct {
	methods []string
}

func CreateRouteMethodsMatcher(methods []string) (*RouteMethodMatcher, error) {
	matcher := &RouteMethodMatcher{
		methods: methods,
	}

	return matcher, nil
}

func (m *RouteMethodMatcher) Match(r *http.Request) bool {
	return slices.Contains(m.methods, r.Method)
}