package core

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestFactory(t *testing.T) {
	t.Run("should forward request with new host", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		originalRequest := httptest.NewRequest(http.MethodGet, "http://test.com", nil)
		expectedHost := "127.0.0.1:8080"

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, expectedHost)

		// assert
		require.NoError(t, err)
		assert.Equal(t, expectedHost, newRequest.Host)
	})

	t.Run("should forward request with original path", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		expectedPath := "/super/test"
		originalRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://test.com%s", expectedPath), nil)

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, "127.0.0.1:8080")

		// assert
		require.NoError(t, err)
		assert.Equal(t, expectedPath, newRequest.URL.Path)
	})

	t.Run("should forward request with original query params", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		expectedQueryParams := "my-params=test"
		originalRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://test.com/super/test?%s", expectedQueryParams), nil)

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, "127.0.0.1:8080")

		// assert
		require.NoError(t, err)
		assert.Equal(t, expectedQueryParams, newRequest.URL.RawQuery)
	})

	t.Run("should switch https to http scheme", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		originalRequest := httptest.NewRequest(http.MethodGet, "https://test.com", nil)

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, "127.0.0.1:8080")

		// assert
		require.NoError(t, err)
		assert.Equal(t, "http", newRequest.URL.Scheme)
	})

	t.Run("should add x-forwarded headers", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		originalRequest := httptest.NewRequest(http.MethodGet, "https://test.com/test", nil)

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, "127.0.0.1:8080")

		// assert
		require.NoError(t, err)
		assert.Equal(t, "test.com", newRequest.Header.Get("X-Forwarded-Host"))
		assert.Equal(t, "https", newRequest.Header.Get("X-Forwarded-Proto"))
		assert.Equal(t, "192.0.2.1:1234", newRequest.Header.Get("X-Forwarded-For"))
	})

	t.Run("should forward existing headers", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		originalRequest := httptest.NewRequest(http.MethodGet, "https://test.com", nil)
		originalRequest.Header.Set("Content-Type", "application/json")
		originalRequest.Header.Set("Authorization", "Bearer token")
		originalRequest.Header.Set("Custom-Header", "custom-value")

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, "127.0.0.1:8080")

		// assert
		require.NoError(t, err)
		assert.Equal(t, "application/json", newRequest.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer token", newRequest.Header.Get("Authorization"))
		assert.Equal(t, "custom-value", newRequest.Header.Get("Custom-Header"))
	})

	t.Run("should forward existing cookies", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		originalRequest := httptest.NewRequest(http.MethodGet, "https://test.com", nil)
		originalRequest.AddCookie(&http.Cookie{Name: "cookie1", Value: "value1"})

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, "127.0.0.1:8080")

		// assert
		require.NoError(t, err)
		cookies := newRequest.Cookies()
		assert.Len(t, cookies, 1)
		assert.Equal(t, "cookie1", newRequest.Cookies()[0].Name)
		assert.Equal(t, "value1", newRequest.Cookies()[0].Value)
	})

	t.Run("should forward body content", func(t *testing.T) {
		t.Parallel()

		// arrange
		factory := CreateHttpRequestForwarderFactory(slog.Default())
		requestData := []byte(`{"title": "foo", "body": "bar", "userId": 1}`)
		originalRequest := httptest.NewRequest(http.MethodPost, "https://test.com", bytes.NewReader(requestData))

		// act
		newRequest, err := factory.CreateForwardedRequestTo(originalRequest, "127.0.0.1:8080")

		// assert
		require.NoError(t, err)
		content, err := io.ReadAll(newRequest.Body)
		require.NoError(t, err)
		assert.Equal(t, requestData, content)
	})
}