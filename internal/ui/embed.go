//go:build embed_ui

package ui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS returns the embedded ui/dist filesystem rooted at dist/.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
