package reverse_proxy

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteHeadersMatcher(t *testing.T) {
	endpointUrl := "http://localhost/simple-query"

	testCases := []struct {
		Name, Url      string
		Matcher        Matcher
		ExpectedStatus int
		Headers        map[string]string
	}{
		{
			Name:           "should forward to application when header match",
			Matcher:        CreateTestHeadersMatcher(map[string]string{"name": "^valid$"}),
			Url:            endpointUrl,
			Headers:        map[string]string{"name": "valid"},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should forward to application when header match with another not required",
			Matcher:        CreateTestHeadersMatcher(map[string]string{"name": "^valid$"}),
			Url:            endpointUrl,
			Headers:        map[string]string{"name": "valid", "non-required": "ok"},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should forward to application when all headers match",
			Matcher:        CreateTestHeadersMatcher(map[string]string{"name": "^valid$", "otherName": "^also-valid$"}),
			Url:            endpointUrl,
			Headers:        map[string]string{"name": "valid", "otherName": "also-valid"},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "should not forward to application when one header not match",
			Matcher:        CreateTestHeadersMatcher(map[string]string{"name": "^valid$", "otherName": "^also-valid$"}),
			Url:            endpointUrl,
			Headers:        map[string]string{"name": "valid", "otherName": "not-valid"},
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
			for headerName, headerValue := range test.Headers {
				request.Header.Add(headerName, headerValue)
			}
			response := httptest.NewRecorder()

			// act
			reverseProxy.router.ServeHTTP(response, request)

			// assert
			assert.Equal(t, test.ExpectedStatus, response.Code)
		})
	}
}

func CreateTestHeadersMatcher(headers map[string]string) Matcher {
	matcher, _ := CreateRouteHeadersMatcher(headers)
	return matcher
}