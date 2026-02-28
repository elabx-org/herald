package resolver_test

import (
	"testing"

	"github.com/elabx-org/herald/internal/resolver"
)

func TestParseOpURI(t *testing.T) {
	tests := []struct {
		input     string
		wantVault string
		wantItem  string
		wantField string
		wantErr   bool
	}{
		{"op://HomeLabVault/postgres-myapp/password", "HomeLabVault", "postgres-myapp", "password", false},
		{"op://vault/item/field", "vault", "item", "field", false},
		{"not-an-op-uri", "", "", "", true},
		{"op://only-vault", "", "", "", true},
		{"op://vault/item", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := resolver.ParseOpURI(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOpURI(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if ref.Vault != tt.wantVault || ref.Item != tt.wantItem || ref.Field != tt.wantField {
				t.Errorf("ParseOpURI(%q) = {%s, %s, %s}, want {%s, %s, %s}",
					tt.input, ref.Vault, ref.Item, ref.Field,
					tt.wantVault, tt.wantItem, tt.wantField)
			}
		})
	}
}

func TestIsOpURI(t *testing.T) {
	if !resolver.IsOpURI("op://vault/item/field") {
		t.Error("expected true for op:// URI")
	}
	if resolver.IsOpURI("DB_PASSWORD=changeme") {
		t.Error("expected false for non-op URI")
	}
}
