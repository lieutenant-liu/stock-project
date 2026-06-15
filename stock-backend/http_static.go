package main

// ============================================================
// http_static.go - 静态文件服务
// ============================================================
// 这个文件负责提供前端静态文件的访问。
//
// 【什么是 SPA？】
// SPA (Single Page Application) 是单页应用：
// - 所有前端代码打包成一个 index.html
// - 路由由前端 JavaScript 处理（不是后端）
// - 所有非 API 路径都应该返回 index.html
//
// 【Go 语言知识点：embed】
// //go:embed web/* 指令会将 web/ 目录下的所有文件嵌入到编译后的二进制文件中。
// 这样部署时只需要一个可执行文件，不需要额外的静态文件目录。
//
// 【Go 语言知识点：fs.Sub】
// fs.Sub 用于创建子文件系统，剥离前缀路径。
// 例如：web/index.html → index.html
// ============================================================

import (
	"embed"     // 嵌入静态文件
	"io/fs"     // 文件系统接口
	"net/http"  // HTTP 服务器
	"strings"   // 字符串处理
)

//go:embed web/*
// webFS 嵌入 web/ 目录下的所有文件
// 编译后，这些文件会被打包到二进制文件中
var webFS embed.FS

// spaFileServer SPA 文件服务器
// 实现了 http.Handler 接口
type spaFileServer struct {
	root fs.FS // 文件系统根目录
}

// ServeHTTP 处理 HTTP 请求
// 【SPA 路由逻辑】
// 1. 先尝试打开请求的文件
// 2. 如果文件存在，直接返回
// 3. 如果文件不存在，返回 index.html（让前端路由处理）
func (s *spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 根路径 → index.html
	if path == "/" {
		path = "index.html"
	} else {
		path = strings.TrimPrefix(path, "/") // 去掉前导斜杠
	}

	// 尝试打开文件
	if f, err := s.root.Open(path); err == nil {
		f.Close()                                          // 文件存在，关闭句柄
		http.FileServer(http.FS(s.root)).ServeHTTP(w, r)   // 返回文件
		return
	}

	// 文件不存在 → 回退到 index.html (SPA 路由)
	// 例如：用户访问 /backtest/123，但这个路径没有对应的文件
	// 返回 index.html，让前端的 React Router 处理路由
	r.URL.Path = "/"
	http.FileServer(http.FS(s.root)).ServeHTTP(w, r)
}

// registerStaticRoutes 注册静态文件路由
// 在 main.go 中调用，将前端文件挂载到根路径 "/"
func registerStaticRoutes() {
	// 前端文件在 web/ 子目录，需要剥离前缀
	// web/index.html → index.html
	webSub, _ := fs.Sub(webFS, "web")

	// 注册到根路径
	// 这样 /api/* 会被 API 路由处理
	// 其他路径会被 spaFileServer 处理（返回前端文件）
	http.Handle("/", &spaFileServer{root: webSub})
}
