package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	flagEnvFile string
	flagDryRun  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Resolve secrets for a stack and write resolved env to stdout or a file",
	RunE:  runSync,
}

func init() {
	syncCmd.Flags().StringVar(&flagStack, "stack", "", "Stack name (required)")
	syncCmd.Flags().StringVar(&flagOut, "out", "-", "Output path: '-' prints to stdout, or an absolute file path")
	syncCmd.Flags().StringVar(&flagURL, "url", envOrDefault("HERALD_URL", "http://herald:8765"), "Herald service URL")
	syncCmd.Flags().StringVar(&flagToken, "token", os.Getenv("HERALD_API_TOKEN"), "Herald API bearer token")
	syncCmd.Flags().IntVar(&flagRetries, "retries", 3, "Number of retries on failure")
	syncCmd.Flags().StringVar(&flagEnvFile, "env-file", "", "Path to env file to scan for op:// refs (use - for stdin)")
	syncCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Resolve secrets and report stats without writing output")
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
	envContent, err := readEnvContent(flagEnvFile)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}

	outPath := flagOut
	if outPath == "-" || flagDryRun {
		outPath = ""
	}

	payload := map[string]interface{}{
		"stack":        flagStack,
		"out_path":     outPath,
		"env_content":  envContent,
		"bypass_cache": true,
	}

	var lastErr error
	var resp syncResponse
	for attempt := 0; attempt <= flagRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "herald-agent: retry %d/%d after error: %v\n", attempt, flagRetries, lastErr)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		resp, lastErr = doSync(payload)
		if lastErr == nil {
			break
		}
		var permErr *permanentError
		if errors.As(lastErr, &permErr) {
			fmt.Fprintf(os.Stderr, "herald-agent: permanent error (no retry): %v\n", lastErr)
			break
		}
	}
	if lastErr != nil {
		fmt.Fprintf(os.Stderr, "herald-agent: failed after %d retries: %v\n", flagRetries, lastErr)
		return lastErr
	}

	if flagDryRun {
		fmt.Fprintf(os.Stderr, "herald-agent: dry run — resolved=%d cache_hits=%d stale_hits=%d failed=%d duration_ms=%d\n",
			resp.Resolved, resp.CacheHits, resp.StaleHits, resp.Failed, resp.DurationMs)
		if resp.Failed > 0 {
			fmt.Fprintf(os.Stderr, "herald-agent: WARNING: %d secret(s) failed to resolve\n", resp.Failed)
			return fmt.Errorf("%d secret(s) failed to resolve", resp.Failed)
		}
		return nil
	}

	if flagOut == "-" {
		fmt.Print(resp.Content)
	} else {
		if err := os.WriteFile(flagOut, []byte(resp.Content), 0600); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "herald-agent: secrets written to %s\n", flagOut)
	}
	return nil
}

// readEnvContent reads env file content from a path, stdin ("-"), or returns empty string.
func readEnvContent(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type syncResponse struct {
	Content    string `json:"content"`
	Resolved   int    `json:"resolved"`
	CacheHits  int    `json:"cache_hits"`
	StaleHits  int    `json:"stale_hits"`
	Failed     int    `json:"failed"`
	DurationMs int64  `json:"duration_ms"`
}

// permanentError wraps errors that should not be retried (e.g. 4xx responses).
type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func doSync(payload map[string]interface{}) (syncResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return syncResponse{}, fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, flagURL+"/v1/materialize/env", bytes.NewReader(body))
	if err != nil {
		return syncResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if flagToken != "" {
		req.Header.Set("Authorization", "Bearer "+flagToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return syncResponse{}, fmt.Errorf("connect to herald: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("herald returned HTTP %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return syncResponse{}, &permanentError{err: err}
		}
		return syncResponse{}, err
	}

	var sr syncResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return syncResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return sr, nil
}
