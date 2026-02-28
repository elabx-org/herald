package resolver

import (
	"bufio"
	"io"
	"strings"
)

// ScanEnvFile reads an env file and returns a map of variable name → SecretRef
// for every value that is an op:// URI.
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
		key, val := parts[0], parts[1]
		if IsOpURI(val) {
			ref, err := ParseOpURI(val)
			if err != nil {
				return nil, err
			}
			refs[key] = ref
		}
	}
	return refs, scanner.Err()
}

// ResolveEnvContent returns the env file content with op:// refs replaced by
// their resolved values. Comments, blank lines, and non-secret lines are
// preserved unchanged.
func ResolveEnvContent(content string, resolved map[string]string) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			sb.WriteString(line + "\n")
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			if val, ok := resolved[parts[0]]; ok {
				sb.WriteString(parts[0] + "=" + val + "\n")
				continue
			}
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
