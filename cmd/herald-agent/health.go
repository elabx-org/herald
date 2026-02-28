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
	Short: "Check Herald service health (exits 0 if healthy, 1 if not)",
	RunE:  runHealth,
}

func init() {
	healthCmd.Flags().StringVar(&flagURL, "url", envOrDefault("HERALD_URL", "http://herald:8765"), "Herald service URL")
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(flagURL + "/v1/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "herald-agent: health check failed: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	status, _ := result["status"].(string)
	if resp.StatusCode == http.StatusOK && status == "ok" {
		fmt.Println("herald: ok")
		return nil
	}

	fmt.Fprintf(os.Stderr, "herald: unhealthy (status=%s http=%d)\n", status, resp.StatusCode)
	os.Exit(1)
	return nil
}
