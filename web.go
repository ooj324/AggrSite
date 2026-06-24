package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed web/dist/*
var webAssets embed.FS

func MountWeb(r chi.Router) {
	dist, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		slog.Error("Failed to load embedded web assets", "err", err)
		return
	}

	fsHandler := http.FileServer(http.FS(dist))

	// Serve all frontend routes, falling back to index.html for SPA
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Don't intercept API routes
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Check if file exists
		f, err := dist.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			f.Close()
			fsHandler.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html
		r.URL.Path = "/"
		fsHandler.ServeHTTP(w, r)
	})
}
