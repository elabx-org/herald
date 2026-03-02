//go:build !embed_ui

package main

import "io/fs"

func getUIFS() fs.FS { return nil }
