package webui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

type Handler struct{ files http.Handler }

func NewHandler() http.Handler {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return &Handler{files: http.FileServer(http.FS(sub))}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean(r.URL.Path)
	if clean == "/" {
		clean = "/index.html"
	}
	if strings.Contains(clean, "..") {
		http.NotFound(w, r)
		return
	}
	switch path.Ext(clean) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	default:
		http.NotFound(w, r)
		return
	}
	r.URL.Path = clean
	h.files.ServeHTTP(w, r)
}
