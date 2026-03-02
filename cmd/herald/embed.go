//go:build embed_ui

package main

import (
	"io/fs"

	heraldui "github.com/elabx-org/herald/internal/ui"
)

func getUIFS() fs.FS {
	f, _ := heraldui.FS()
	return f
}
