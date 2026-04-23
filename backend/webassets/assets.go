package webassets

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return distFS
	}
	return sub
}
