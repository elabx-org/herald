//go:build !embed_ui

package ui

import "io/fs"

// FS returns nil when built without embed_ui tag.
func FS() (fs.FS, error) { return nil, nil }
