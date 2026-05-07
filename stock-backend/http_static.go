package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/*
var webFS embed.FS

type spaFileServer struct {
	root fs.FS
}

func (s *spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else {
		path = strings.TrimPrefix(path, "/")
	}

	// 尝试打开文件
	if f, err := s.root.Open(path); err == nil {
		f.Close()
		http.FileServer(http.FS(s.root)).ServeHTTP(w, r)
		return
	}

	// 文件不存在 → 回退到 index.html (SPA 路由)
	r.URL.Path = "/"
	http.FileServer(http.FS(s.root)).ServeHTTP(w, r)
}

func registerStaticRoutes() {
	// 前端文件在 web/ 子目录，需要剥离前缀
	webSub, _ := fs.Sub(webFS, "web")
	http.Handle("/", &spaFileServer{root: webSub})
}
