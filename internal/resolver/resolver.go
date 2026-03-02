package resolver

import (
	"fmt"
	"regexp"
	"strings"
)

var refPattern = regexp.MustCompile(`(?:herald|op)://([^/\s]+)/([^/\s]+)/([^\s"'@,;]+)`)

type Ref struct {
	Raw   string
	Vault string
	Item  string
	Field string
}

func ParseRef(s string) (Ref, error) {
	m := refPattern.FindStringSubmatch(s)
	if m == nil {
		return Ref{}, fmt.Errorf("resolver: %q is not a valid herald:// or op:// reference", s)
	}
	return Ref{Raw: m[0], Vault: m[1], Item: m[2], Field: m[3]}, nil
}

func ScanRefs(content string) []Ref {
	matches := refPattern.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	var refs []Ref
	for _, m := range matches {
		if !seen[m[0]] {
			seen[m[0]] = true
			refs = append(refs, Ref{Raw: m[0], Vault: m[1], Item: m[2], Field: m[3]})
		}
	}
	return refs
}

func SubstituteRefs(content string, resolved map[string]string) string {
	result := content
	for raw, val := range resolved {
		result = strings.ReplaceAll(result, raw, val)
	}
	return result
}
