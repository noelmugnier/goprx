package reverse_proxy

import (
	"fmt"
	"net/http"
	"regexp"
)

type RoutePathPrefixMatcher struct {
	prefixRegex *regexp.Regexp
}

func CreateRoutePathPrefixMatcher(prefix string) (*RoutePathPrefixMatcher, error) {
	compiledRegex, err := regexp.Compile(fmt.Sprintf("^%s", prefix))
	if err != nil {
		return nil, err
	}
	return &RoutePathPrefixMatcher{
		prefixRegex: compiledRegex,
	}, nil
}

func (m *RoutePathPrefixMatcher) Match(r *http.Request) bool {
	return m.prefixRegex.MatchString(r.URL.Path)
}