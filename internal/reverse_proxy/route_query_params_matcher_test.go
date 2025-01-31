package reverse_proxy

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteQueryParamsMatcher(t *testing.T) {
	endpointUrl := "http://localhost/simple-query"

	testCases := []struct {
		Name, Url      string
		Matcher        Matcher
		ExpectedStatus int
	}{
		{
			Name:           "should forward to application when query param match",
			Matcher:        CreateTestQueryParamsMatcher(map[string]string{"name": "^valid$"}),
			Url:            fmt.Sprintf("%s?name=valid", endpointUrl),
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should forward to application when query param match with other non required",
			Matcher:        CreateTestQueryParamsMatcher(map[string]string{"name": "^valid$"}),
			Url:            fmt.Sprintf("%s?name=valid&nonrequired=false", endpointUrl),
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should forward to application when any method match",
			Matcher:        CreateTestQueryParamsMatcher(map[string]string{"name": "^valid$", "otherName": "^also-valid$"}),
			Url:            fmt.Sprintf("%s?name=valid&otherName=also-valid", endpointUrl),
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should not forward to application when a param not match",
			Matcher:        CreateTestQueryParamsMatcher(map[string]string{"name": "^valid$", "otherName": "^also-valid$"}),
			Url:            fmt.Sprintf("%s?name=valid&otherName=not-valid", endpointUrl),
			ExpectedStatus: http.StatusNotFound,
		},
	}

	for _, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			// arrange
			reverseProxy := createTestReverseProxy()
			reverseProxy.registerTestApplicationAndWait([]Matcher{test.Matcher}, handlerWithRequestAsResponseContent())

			request := httptest.NewRequest(http.MethodGet, test.Url, nil)
			response := httptest.NewRecorder()

			// act
			reverseProxy.router.ServeHTTP(response, request)

			// assert
			assert.Equal(t, test.ExpectedStatus, response.Code)
		})
	}
}

func CreateTestQueryParamsMatcher(params map[string]string) Matcher {
	matcher, _ := CreateRouteQueryParamsMatcher(params)
	return matcher
}