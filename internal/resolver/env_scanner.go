package resolver

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// opURIRegex matches op:// URIs with vault/item/field path segments.
// The character set (alphanumeric, underscore, hyphen) safely terminates at
// common delimiters like @, :, whitespace, and quotes that appear in
// surrounding strings, enabling inline substitution within larger values.
var opURIRegex = regexp.MustCompile(`op://[A-Za-z0-9_-]+/[A-Za-z0-9_-]+/[A-Za-z0-9_-]+`)

// ScanEnvFile reads an env file and returns all op:// URIs found, keyed by the
// raw URI string. Both standalone values (KEY=op://...) and inline embedded
// values (KEY=prefix:op://...:suffix) are detected. Duplicate URIs are
// deduplicated so each secret is fetched only once.
func ScanEnvFile(r io.Reader) (map[string]*SecretRef, error) {
	refs := make(map[string]*SecretRef)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		for _, rawURI := range opURIRegex.FindAllString(parts[1], -1) {
			if _, exists := refs[rawURI]; exists {
				continue
			}
			ref, err := ParseOpURI(rawURI)
			if err != nil {
				return nil, err
			}
			refs[rawURI] = ref
		}
	}
	return refs, scanner.Err()
}

// ResolveEnvContent returns the env file content with all op:// URIs replaced
// by their resolved values. resolvedByURI maps raw op:// URI → resolved value.
// Comments, blank lines, and values containing no op:// refs pass through unchanged.
func ResolveEnvContent(content string, resolvedByURI map[string]string) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			sb.WriteString(line + "\n")
			continue
		}
		resolved := opURIRegex.ReplaceAllStringFunc(line, func(uri string) string {
			if val, ok := resolvedByURI[uri]; ok {
				return val
			}
			return uri
		})
		sb.WriteString(resolved + "\n")
	}
	return sb.String()
}
