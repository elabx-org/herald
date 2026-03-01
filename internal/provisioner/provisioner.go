package provisioner

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"
)

// FieldSpec describes a field to create in a 1Password item.
type FieldSpec struct {
	Value     string // empty = generate a random password
	Concealed bool   // true = store as concealed (password) field
}

// ProvisionRequest is the input to Provision.
type ProvisionRequest struct {
	Vault    string               // vault name (e.g. "HomeLab")
	Item     string               // item title (e.g. "herald-test")
	Category string               // "login", "api_credentials", or "secure_note" (default: "login")
	Fields   map[string]FieldSpec // field name → spec
}

// ProvisionResult is returned after successful provisioning.
type ProvisionResult struct {
	VaultID string
	ItemID  string
	// Refs maps field name → op:// URI
	Refs map[string]string
}

// Provisionable is the interface for creating 1Password items.
type Provisionable interface {
	Provision(ctx context.Context, req ProvisionRequest) (ProvisionResult, error)
}

// Provisioner creates items in 1Password using the provision service account token.
type Provisioner struct {
	client *onepassword.Client
}

// New creates a Provisioner using OP_PROVISION_TOKEN from the environment.
func New() (*Provisioner, error) {
	token := os.Getenv("OP_PROVISION_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("OP_PROVISION_TOKEN is not set")
	}
	client, err := onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("herald-provisioner", "1.0.0"),
	)
	if err != nil {
		return nil, fmt.Errorf("create 1password provisioner client: %w", err)
	}
	return &Provisioner{client: client}, nil
}

// Provision creates or updates an item in the named vault with the given fields.
// If an item with the same title already exists, only missing fields are added (upsert).
func (p *Provisioner) Provision(ctx context.Context, req ProvisionRequest) (ProvisionResult, error) {
	// Resolve vault name → ID
	vaultID, err := p.resolveVaultID(ctx, req.Vault)
	if err != nil {
		return ProvisionResult{}, err
	}

	// Check if item with this title already exists
	existingID, err := p.findItemIDByTitle(ctx, vaultID, req.Item)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("check existing item: %w", err)
	}
	if existingID != "" {
		return p.upsertExistingItem(ctx, vaultID, existingID, req)
	}

	// Build item fields for new item
	var fields []onepassword.ItemField
	for name, spec := range req.Fields {
		value := spec.Value
		if value == "" {
			value, err = generatePassword(24)
			if err != nil {
				return ProvisionResult{}, fmt.Errorf("generate password for field %q: %w", name, err)
			}
		}

		fieldType := onepassword.ItemFieldTypeText
		if spec.Concealed || isLikelySecret(name) {
			fieldType = onepassword.ItemFieldTypeConcealed
		}

		fields = append(fields, onepassword.ItemField{
			ID:        name,
			Title:     name,
			FieldType: fieldType,
			Value:     value,
		})
	}

	category := resolveCategory(req.Category)

	params := onepassword.ItemCreateParams{
		Category: category,
		VaultID:  vaultID,
		Title:    req.Item,
		Fields:   fields,
	}

	item, err := p.client.Items().Create(ctx, params)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("create item %q in vault %q: %w", req.Item, req.Vault, err)
	}

	refs := make(map[string]string, len(item.Fields))
	for _, f := range item.Fields {
		if f.ID != "" {
			refs[f.ID] = fmt.Sprintf("op://%s/%s/%s", req.Vault, req.Item, f.ID)
		}
	}

	return ProvisionResult{
		VaultID: vaultID,
		ItemID:  item.ID,
		Refs:    refs,
	}, nil
}

// upsertExistingItem adds only the missing fields to an existing item and returns refs for all fields.
func (p *Provisioner) upsertExistingItem(ctx context.Context, vaultID, itemID string, req ProvisionRequest) (ProvisionResult, error) {
	item, err := p.client.Items().Get(ctx, vaultID, itemID)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("get existing item %q: %w", itemID, err)
	}

	// Build set of existing field IDs
	existingFields := make(map[string]struct{}, len(item.Fields))
	for _, f := range item.Fields {
		if f.ID != "" {
			existingFields[f.ID] = struct{}{}
		}
	}

	// Add only fields that don't already exist
	var addedAny bool
	for name, spec := range req.Fields {
		if _, exists := existingFields[name]; exists {
			continue
		}
		value := spec.Value
		if value == "" {
			var err error
			value, err = generatePassword(24)
			if err != nil {
				return ProvisionResult{}, fmt.Errorf("generate password for field %q: %w", name, err)
			}
		}
		fieldType := onepassword.ItemFieldTypeText
		if spec.Concealed || isLikelySecret(name) {
			fieldType = onepassword.ItemFieldTypeConcealed
		}
		item.Fields = append(item.Fields, onepassword.ItemField{
			ID:        name,
			Title:     name,
			FieldType: fieldType,
			Value:     value,
		})
		addedAny = true
	}

	if addedAny {
		item, err = p.client.Items().Put(ctx, item)
		if err != nil {
			return ProvisionResult{}, fmt.Errorf("update item %q: %w", itemID, err)
		}
	}

	refs := make(map[string]string, len(item.Fields))
	for _, f := range item.Fields {
		if f.ID != "" {
			refs[f.ID] = fmt.Sprintf("op://%s/%s/%s", req.Vault, req.Item, f.ID)
		}
	}

	return ProvisionResult{
		VaultID: vaultID,
		ItemID:  item.ID,
		Refs:    refs,
	}, nil
}

// findItemIDByTitle returns the ID of the first item matching the given title in the vault,
// or empty string if not found.
func (p *Provisioner) findItemIDByTitle(ctx context.Context, vaultID, title string) (string, error) {
	items, err := p.client.Items().List(ctx, vaultID)
	if err != nil {
		return "", fmt.Errorf("list items in vault %q: %w", vaultID, err)
	}
	for _, item := range items {
		if strings.EqualFold(item.Title, title) {
			return item.ID, nil
		}
	}
	return "", nil
}

// resolveVaultID finds the vault ID for a given vault name.
func (p *Provisioner) resolveVaultID(ctx context.Context, vaultName string) (string, error) {
	vaults, err := p.client.Vaults().List(ctx)
	if err != nil {
		return "", fmt.Errorf("list vaults: %w", err)
	}
	for _, v := range vaults {
		if strings.EqualFold(v.Title, vaultName) {
			return v.ID, nil
		}
	}
	return "", fmt.Errorf("vault %q not found", vaultName)
}

// generatePassword creates a URL-safe random password of the given length.
func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Use base64 URL encoding but trim to desired length
	s := base64.URLEncoding.EncodeToString(b)
	// Replace - and _ with alphanumeric for friendliness
	s = strings.ReplaceAll(s, "-", "A")
	s = strings.ReplaceAll(s, "_", "B")
	s = strings.ReplaceAll(s, "=", "")
	if len(s) > length {
		s = s[:length]
	}
	return s, nil
}

// isLikelySecret returns true if the field name suggests it holds a secret value.
func isLikelySecret(name string) bool {
	lower := strings.ToLower(name)
	secretWords := []string{"password", "passwd", "secret", "token", "key", "api_key", "apikey", "credential", "private"}
	for _, w := range secretWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// resolveCategory maps a user-supplied category string to the SDK constant.
func resolveCategory(cat string) onepassword.ItemCategory {
	switch strings.ToLower(cat) {
	case "api_credentials", "api-credentials", "apicredentials":
		return onepassword.ItemCategoryAPICredentials
	case "secure_note", "secure-note", "securenote", "note":
		return onepassword.ItemCategorySecureNote
	default:
		return onepassword.ItemCategoryLogin
	}
}
