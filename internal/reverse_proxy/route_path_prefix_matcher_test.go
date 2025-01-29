package reverse_proxy

import (
	"fmt"
	"github.com/noelmugnier/goprx/internal/core"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutePathPrefixMatcher(t *testing.T) {
	endpointUrl := "http://localhost/simple-query"

	pathTests := []struct {
		Name, Url      string
		Matcher        core.Matcher
		ExpectedStatus int
	}{
		{
			Name:           "should forward to application when path match",
			Matcher:        CreateTestPathPrefixMatcher("/simple-query$"),
			Url:            endpointUrl,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should forward to application when path prefix match",
			Matcher:        CreateTestPathPrefixMatcher("/simple-query"),
			Url:            fmt.Sprintf("%s%s", endpointUrl, "/sub/path"),
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should not forward to application when path prefix not match",
			Matcher:        CreateTestPathPrefixMatcher("/another-query"),
			Url:            endpointUrl,
			ExpectedStatus: http.StatusNotFound,
		},
	}

	for _, test := range pathTests {
		t.Run(test.Name, func(t *testing.T) {
			// arrange
			reverseProxy := createTestReverseProxy()
			reverseProxy.registerTestApplicationAndWait([]core.Matcher{test.Matcher}, handlerWithRequestAsResponseContent())

			request := httptest.NewRequest(http.MethodGet, test.Url, nil)
			response := httptest.NewRecorder()

			// act
			reverseProxy.router.ServeHTTP(response, request)

			// assert
			assert.Equal(t, test.ExpectedStatus, response.Code)
		})
	}
}

func CreateTestPathPrefixMatcher(value string) core.Matcher {
	matcher, _ := CreateRoutePathPrefixMatcher(value)
	return matcher
}