package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagStack   string
	flagOut     string
	flagURL     string
	flagToken   string
	flagRetries int
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Resolve secrets for a stack and write to an env file",
	RunE:  runSync,
}

func init() {
	syncCmd.Flags().StringVar(&flagStack, "stack", "", "Stack name (required)")
	syncCmd.Flags().StringVar(&flagOut, "out", "", "Output env file path (default: /run/herald/{stack}.env)")
	syncCmd.Flags().StringVar(&flagURL, "url", envOrDefault("HERALD_URL", "http://herald:8765"), "Herald service URL")
	syncCmd.Flags().StringVar(&flagToken, "token", os.Getenv("HERALD_API_TOKEN"), "Herald API bearer token")
	syncCmd.Flags().IntVar(&flagRetries, "retries", 3, "Number of retries on failure")
	syncCmd.MarkFlagRequired("stack")
	rootCmd.AddCommand(syncCmd)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func runSync(cmd *cobra.Command, args []string) error {
	if flagOut == "" {
		flagOut = "/run/herald/" + flagStack + ".env"
	}

	payload := map[string]string{
		"stack":    flagStack,
		"out_path": flagOut,
	}

	var lastErr error
	for attempt := 0; attempt <= flagRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "herald-agent: retry %d/%d after error: %v\n", attempt, flagRetries, lastErr)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		if lastErr = doSync(payload); lastErr == nil {
			fmt.Fprintf(os.Stdout, "herald-agent: secrets written to %s\n", flagOut)
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, "herald-agent: failed after %d retries: %v\n", flagRetries, lastErr)
	return lastErr
}

func doSync(payload map[string]string) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, flagURL+"/v1/materialize/env", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if flagToken != "" {
		req.Header.Set("Authorization", "Bearer "+flagToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to herald: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("herald returned HTTP %d", resp.StatusCode)
	}
	return nil
}
