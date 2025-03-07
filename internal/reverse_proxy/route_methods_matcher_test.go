package reverse_proxy

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteMethodsMatcher(t *testing.T) {
	endpointUrl := "http://localhost/simple-query"

	testCases := []struct {
		Name, Url      string
		Matcher        Matcher
		ExpectedStatus int
		Method         string
	}{
		{
			Name:           "should forward to application when method match",
			Matcher:        CreateTestMethodsMatcher(http.MethodGet),
			Url:            endpointUrl,
			Method:         http.MethodGet,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should forward to application when any method match",
			Matcher:        CreateTestMethodsMatcher(http.MethodGet, http.MethodPost),
			Url:            endpointUrl,
			Method:         http.MethodGet,
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should not forward to application when no method match",
			Matcher:        CreateTestMethodsMatcher(http.MethodPost),
			Url:            endpointUrl,
			Method:         http.MethodGet,
			ExpectedStatus: http.StatusNotFound,
		},
	}

	for _, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			// arrange
			reverseProxy := createTestReverseProxy()
			reverseProxy.registerTestApplicationAndWait([]Matcher{test.Matcher}, handlerWithRequestAsResponseContent())

			request := httptest.NewRequest(test.Method, test.Url, nil)
			response := httptest.NewRecorder()

			// act
			reverseProxy.router.ServeHTTP(response, request)

			// assert
			assert.Equal(t, test.ExpectedStatus, response.Code)
		})
	}
}

func CreateTestMethodsMatcher(methods ...string) Matcher {
	matcher, _ := CreateRouteMethodsMatcher(methods)
	return matcher
}