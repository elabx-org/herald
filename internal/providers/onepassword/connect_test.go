//go:build integration

package onepassword_test

import (
	"context"
	"os"
	"testing"

	"github.com/elabx-org/herald/internal/providers/onepassword"
)

func TestConnect_Resolve(t *testing.T) {
	url := os.Getenv("OP_CONNECT_SERVER_URL")
	token := os.Getenv("OP_CONNECT_TOKEN")
	if url == "" || token == "" {
		t.Skip("OP_CONNECT_SERVER_URL and OP_CONNECT_TOKEN required")
	}
	p, err := onepassword.NewConnect("test-connect", url, token, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	ok, _, err := p.Healthy(context.Background())
	if err != nil || !ok {
		t.Fatalf("unhealthy: %v", err)
	}
}
