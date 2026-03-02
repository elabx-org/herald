package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagProvVault    string
	flagProvItem     string
	flagProvCategory string
	flagProvFields   []string
)

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Create or upsert a 1Password item via Herald",
	Long: `Create or upsert a 1Password item. Fields with no value are auto-generated.
If the item already exists, only missing fields are added — existing values are never overwritten.

Field format: --field name[:value][:concealed]
  --field db_password:concealed         auto-generate a concealed field
  --field api_key:value=known:concealed  set a known concealed value
  --field username:value=myapp-user      plain text field`,
	RunE: runProvision,
}

func init() {
	provisionCmd.Flags().StringVar(&flagProvVault, "vault", "", "1Password vault name (required)")
	provisionCmd.Flags().StringVar(&flagProvItem, "item", "", "1Password item name (required)")
	provisionCmd.Flags().StringVar(&flagProvCategory, "category", "login", "Item category: login, api_credentials, secure_note")
	provisionCmd.Flags().StringArrayVar(&flagProvFields, "field", nil, "Field spec: name[:value=VALUE][:concealed]")
	provisionCmd.Flags().StringVar(&flagURL, "url", envOrDefault("HERALD_URL", "http://herald:8765"), "Herald service URL")
	provisionCmd.Flags().StringVar(&flagToken, "token", "", "Herald API bearer token")
	provisionCmd.MarkFlagRequired("vault")
	provisionCmd.MarkFlagRequired("item")
	provisionCmd.MarkFlagRequired("field")
	rootCmd.AddCommand(provisionCmd)
}

type provisionField struct {
	Value     string `json:"value,omitempty"`
	Concealed bool   `json:"concealed,omitempty"`
}

type provisionRequest struct {
	Vault    string                    `json:"vault"`
	Item     string                    `json:"item"`
	Category string                    `json:"category,omitempty"`
	Fields   map[string]provisionField `json:"fields"`
}

type provisionResponse struct {
	VaultID string            `json:"vault_id"`
	ItemID  string            `json:"item_id"`
	Refs    map[string]string `json:"refs"`
}

func runProvision(cmd *cobra.Command, args []string) error {
	fields, err := parseProvisionFields(flagProvFields)
	if err != nil {
		return err
	}

	payload := provisionRequest{
		Vault:    flagProvVault,
		Item:     flagProvItem,
		Category: flagProvCategory,
		Fields:   fields,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, flagURL+"/v1/provision", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if flagToken != "" {
		req.Header.Set("Authorization", "Bearer "+flagToken)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to herald: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("herald returned HTTP %d", resp.StatusCode)
	}

	var pr provisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("vault_id: %s\nitem_id:  %s\n\nrefs:\n", pr.VaultID, pr.ItemID)
	for name, ref := range pr.Refs {
		fmt.Printf("  %-20s  %s\n", name, ref)
	}
	return nil
}

// parseProvisionFields parses --field specs into a map.
// Format: name[:value=VALUE][:concealed]
func parseProvisionFields(specs []string) (map[string]provisionField, error) {
	fields := make(map[string]provisionField, len(specs))
	for _, spec := range specs {
		parts := strings.Split(spec, ":")
		name := parts[0]
		if name == "" {
			return nil, fmt.Errorf("invalid field spec %q: name cannot be empty", spec)
		}
		f := provisionField{}
		for _, opt := range parts[1:] {
			switch {
			case opt == "concealed":
				f.Concealed = true
			case strings.HasPrefix(opt, "value="):
				f.Value = strings.TrimPrefix(opt, "value=")
			default:
				return nil, fmt.Errorf("invalid field option %q in spec %q (use 'concealed' or 'value=...')", opt, spec)
			}
		}
		fields[name] = f
	}
	return fields, nil
}
