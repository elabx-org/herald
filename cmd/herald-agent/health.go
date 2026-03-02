package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check Herald service health and provider status (exits 1 if degraded)",
	RunE:  runHealth,
}

func init() {
	healthCmd.Flags().StringVar(&flagURL, "url", envOrDefault("HERALD_URL", "http://herald:8765"), "Herald service URL")
	healthCmd.Flags().StringVar(&flagToken, "token", os.Getenv("HERALD_API_TOKEN"), "Herald API bearer token")
	rootCmd.AddCommand(healthCmd)
}

type healthResponse struct {
	Status        string           `json:"status"`
	Provisioner   string           `json:"provisioner,omitempty"`
	UptimeSeconds int64            `json:"uptime_seconds"`
	Providers     []providerStatus `json:"providers"`
}

type providerStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runHealth(cmd *cobra.Command, args []string) error {
	req, err := http.NewRequest(http.MethodGet, flagURL+"/v1/health", nil)
	if err != nil {
		return err
	}
	if flagToken != "" {
		req.Header.Set("Authorization", "Bearer "+flagToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "herald-agent: cannot reach herald: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	var h healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("status: %s  uptime: %ds", h.Status, h.UptimeSeconds)
	if h.Provisioner != "" {
		fmt.Printf("  provisioner: %s", h.Provisioner)
	}
	fmt.Println()
	for _, p := range h.Providers {
		line := fmt.Sprintf("  %-20s  %-16s  %s", p.Name, p.Type, p.Status)
		if p.LatencyMs > 0 {
			line += fmt.Sprintf("  (%dms)", p.LatencyMs)
		}
		if p.Error != "" {
			line += "  error: " + p.Error
		}
		fmt.Println(line)
	}

	if h.Status != "ok" {
		return fmt.Errorf("herald status: %s", h.Status)
	}
	return nil
}
