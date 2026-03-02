package komodo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/elabx-org/herald/internal/integrations"
)

type Integration struct {
	name      string
	url       string
	apiKey    string
	apiSecret string
	client    *http.Client
}

func New(name, url, apiKey, apiSecret string) *Integration {
	return &Integration{
		name: name, url: url, apiKey: apiKey, apiSecret: apiSecret,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (k *Integration) Name() string { return k.name }
func (k *Integration) Type() string { return "komodo" }

func (k *Integration) Deploy(ctx context.Context, stack string) error {
	body, _ := json.Marshal(map[string]string{"stack": stack})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		k.url+"/execute/DeployStack",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", k.apiKey)
	req.Header.Set("X-Api-Secret", k.apiSecret)
	resp, err := k.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("komodo: deploy %q returned %d", stack, resp.StatusCode)
	}
	return nil
}

func (k *Integration) Notify(_ context.Context, _ integrations.Event) error { return nil }

func (k *Integration) Healthy(ctx context.Context) (bool, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, k.url+"/auth/me", nil)
	req.Header.Set("X-Api-Key", k.apiKey)
	req.Header.Set("X-Api-Secret", k.apiSecret)
	resp, err := k.client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	return resp.StatusCode == 200, nil
}
