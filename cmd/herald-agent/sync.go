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

	// When outputting to stdout, don't ask server to write a file
	outPath := flagOut
	if outPath == "-" {
		outPath = ""
	}

	payload := map[string]interface{}{
		"stack":       flagStack,
		"out_path":    outPath,
		"env_content": envContent,
	}

	var lastErr error
	var content string
	for attempt := 0; attempt <= flagRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "herald-agent: retry %d/%d after error: %v\n", attempt, flagRetries, lastErr)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		content, lastErr = doSync(payload)
		if lastErr == nil {
			break
		}
		// Don't retry permanent errors (e.g. 4xx responses)
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

	if flagOut == "-" {
		fmt.Print(content)
	} else {
		if err := os.WriteFile(flagOut, []byte(content), 0600); err != nil {
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
	Content string `json:"content"`
}

// permanentError wraps errors that should not be retried (e.g. 4xx responses).
type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func doSync(payload map[string]interface{}) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, flagURL+"/v1/materialize/env", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if flagToken != "" {
		req.Header.Set("Authorization", "Bearer "+flagToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect to herald: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("herald returned HTTP %d", resp.StatusCode)
		// 4xx errors are permanent — don't retry
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return "", &permanentError{err: err}
		}
		return "", err
	}

	var sr syncResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return sr.Content, nil
}
