//go:build prod

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed dist/*
var webAssets embed.FS

type fallbackFileSystem struct {
	target fs.FS
}

func (f fallbackFileSystem) Open(name string) (fs.File, error) {
	file, err := f.target.Open(name)
	if os.IsNotExist(err) {
		return f.target.Open("index.html")
	}
	return file, err
}

func HandleWeb() http.Handler {
	publicFS, err := fs.Sub(webAssets, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fallbackFileSystem{target: publicFS}))
}
