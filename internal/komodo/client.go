package komodo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	url       string
	apiKey    string
	apiSecret string
	http      *http.Client
}

func NewClient(url, apiKey, apiSecret string) *Client {
	return &Client{
		url:       url,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("X-Api-Secret", c.apiSecret)
	return c.http.Do(req)
}

func (c *Client) DeployStack(ctx context.Context, stackName string) error {
	resp, err := c.do(ctx, http.MethodPost, "/execute/DeployStack", map[string]string{
		"stack": stackName,
	})
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("deploy stack %q: HTTP %d", stackName, resp.StatusCode)
	}
	return nil
}

func (c *Client) SendAlert(ctx context.Context, level, message string) error {
	resp, err := c.do(ctx, http.MethodPost, "/write/SendAlert", map[string]string{
		"level":   level,
		"message": message,
	})
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	return nil
}
