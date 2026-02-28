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
