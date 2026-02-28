package resolver

import (
	"fmt"
	"strings"
)

const opScheme = "op://"

// SecretRef represents a parsed op:// URI.
type SecretRef struct {
	Vault string
	Item  string
	Field string
	Raw   string
}

// IsOpURI returns true if the value starts with op://.
func IsOpURI(value string) bool {
	return strings.HasPrefix(value, opScheme)
}

// ParseOpURI parses an op:// URI into vault, item, and field components.
// Format: op://VaultName/ItemName/FieldName
func ParseOpURI(uri string) (*SecretRef, error) {
	if !IsOpURI(uri) {
		return nil, fmt.Errorf("not an op:// URI: %q", uri)
	}
	path := strings.TrimPrefix(uri, opScheme)
	parts := strings.SplitN(path, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, fmt.Errorf("invalid op:// URI %q: expected op://vault/item/field", uri)
	}
	return &SecretRef{
		Vault: parts[0],
		Item:  parts[1],
		Field: parts[2],
		Raw:   uri,
	}, nil
}
