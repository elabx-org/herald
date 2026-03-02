package resolver_test

import (
	"testing"

	"github.com/elabx-org/herald/internal/resolver"
)

func TestParseRef_Herald(t *testing.T) {
	ref, err := resolver.ParseRef("herald://HomeLab/myapp/db_password")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Vault != "HomeLab" || ref.Item != "myapp" || ref.Field != "db_password" {
		t.Errorf("unexpected: %+v", ref)
	}
}

func TestParseRef_Op(t *testing.T) {
	ref, err := resolver.ParseRef("op://HomeLab/myapp/db_password")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Vault != "HomeLab" || ref.Item != "myapp" || ref.Field != "db_password" {
		t.Errorf("unexpected: %+v", ref)
	}
}

func TestParseRef_Invalid(t *testing.T) {
	_, err := resolver.ParseRef("not-a-ref")
	if err == nil {
		t.Error("expected error for invalid ref")
	}
}

func TestScanRefs_Inline(t *testing.T) {
	content := `DB_URL=postgres://admin:herald://Vault/item/field@host:5432/db
PLAIN=hello`
	refs := resolver.ScanRefs(content)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d: %v", len(refs), refs)
	}
	if refs[0].Raw != "herald://Vault/item/field" {
		t.Errorf("unexpected raw: %q", refs[0].Raw)
	}
}

func TestSubstituteRefs(t *testing.T) {
	content := "URL=https://example.com\nSECRET=herald://V/I/F"
	resolved := map[string]string{"herald://V/I/F": "mysecret"}
	result := resolver.SubstituteRefs(content, resolved)
	expected := "URL=https://example.com\nSECRET=mysecret"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
