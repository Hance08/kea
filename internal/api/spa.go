// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"bytes"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	spaIndexFile      = "index.html"
	cacheControlShell = "no-cache"
	cacheControlAsset = "public, max-age=31536000, immutable"
)

func spaHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "..") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		fsPath := strings.TrimPrefix(r.URL.Path, "/")
		if fsPath == "" {
			fsPath = spaIndexFile
		}

		info, err := fs.Stat(fsys, fsPath)
		switch {
		case err == nil && info.Mode().IsRegular():
			serveFSFile(w, r, fsys, fsPath)
			return
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		serveIndex(w, r, fsys)
	})
}

func serveFSFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, fsPath string) {
	data, err := fs.ReadFile(fsys, fsPath)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ct := mime.TypeByExtension(path.Ext(fsPath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if strings.HasPrefix(fsPath, "assets/") {
		w.Header().Set("Cache-Control", cacheControlAsset)
	} else {
		w.Header().Set("Cache-Control", cacheControlShell)
	}
	http.ServeContent(w, r, fsPath, time.Time{}, bytes.NewReader(data))
}

func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, spaIndexFile)
	if err != nil {
		http.Error(w, "spa shell missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControlShell)
	http.ServeContent(w, r, spaIndexFile, time.Time{}, bytes.NewReader(data))
}
