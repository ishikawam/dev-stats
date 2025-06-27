package common

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient provides a common HTTP client interface
type HTTPClient struct {
	client  *http.Client
	headers map[string]string
}

// NewHTTPClient creates a new HTTP client with common settings
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 120 * time.Second, // Increase timeout to 2 minutes
		},
		headers: make(map[string]string),
	}
}

// SetHeader sets a default header for all requests
func (c *HTTPClient) SetHeader(key, value string) {
	c.headers[key] = value
}

// SetTimeout sets the HTTP client timeout
func (c *HTTPClient) SetTimeout(timeout time.Duration) {
	c.client.Timeout = timeout
}

// Get performs a GET request
func (c *HTTPClient) Get(url string, headers map[string]string) ([]byte, error) {
	return c.makeRequest("GET", url, nil, headers)
}

// Post performs a POST request
func (c *HTTPClient) Post(url string, body string, headers map[string]string) ([]byte, error) {
	return c.makeRequest("POST", url, strings.NewReader(body), headers)
}

// makeRequest performs an HTTP request with common error handling
func (c *HTTPClient) makeRequest(method, url string, body io.Reader, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, WrapError(err, "failed to create %s request to %s", method, url)
	}

	// Set default headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// Set request-specific headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, WrapError(err, "failed to execute %s request to %s", method, url)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, WrapError(err, "failed to read response body")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NewError("HTTP %d error for %s %s: %s", resp.StatusCode, method, url, string(responseBody))
	}

	return responseBody, nil
}
