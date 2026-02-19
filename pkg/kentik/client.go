package kentik

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config holds the credentials and region for authenticating with Kentik.
type Config struct {
	Email    string
	APIToken string
	Region   string // "US" (default) or "EU"
}

// Client is an HTTP client for the Kentik API.
type Client struct {
	email    string
	apiToken string
	v5Base   string
	v6Base   string
	http     *http.Client
}

// NewClient creates a new Kentik API client.
func NewClient(cfg Config) *Client {
	region := strings.ToUpper(cfg.Region)
	var v5Base, v6Base string
	if region == "EU" {
		v5Base = "https://api.kentik.eu/api/v5"
		v6Base = "https://grpc.api.kentik.eu"
	} else {
		v5Base = "https://api.kentik.com/api/v5"
		v6Base = "https://grpc.api.kentik.com"
	}
	return &Client{
		email:    cfg.Email,
		apiToken: cfg.APIToken,
		v5Base:   v5Base,
		v6Base:   v6Base,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) headers() map[string]string {
	return map[string]string{
		"X-CH-Auth-Email":     c.email,
		"X-CH-Auth-API-Token": c.apiToken,
		"Content-Type":        "application/json",
	}
}

func (c *Client) doRequest(method, url string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return json.RawMessage(respBody), nil
}

// V5 makes a request to the Kentik V5 REST API.
// path should start with "/" e.g. "/devices".
func (c *Client) V5(method, path string, body interface{}) (json.RawMessage, error) {
	url := c.v5Base + path
	return c.doRequest(method, url, body)
}

// V6 makes a request to the Kentik V6 gRPC-gateway API.
// path should be the full path e.g. "/synthetics/v202309/tests".
func (c *Client) V6(method, path string, body interface{}) (json.RawMessage, error) {
	url := c.v6Base + path
	return c.doRequest(method, url, body)
}
