package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const outputFile = "project_context.md"

// 忽略的目录：防止大模型被无关信息淹没（特别注意规避 git 和依赖库）
var ignoreDirs = map[string]bool{
	".git":         true,
	".idea":        true,
	".vscode":      true,
	"vendor":       true,
	"node_modules": true,
	"bin":          true,
	"data":         true, // 假设你的股票项目里有本地数据文件夹，不要导给大模型
}

// 允许导出的文件后缀：按需增删
var allowedExts = map[string]bool{
	".go":   true,
	".mod":  true,
	".sum":  true,
	".md":   true,
	".json": true,
	".yaml": true,
}

func main() {
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("创建输出文件失败: %v\n", err)
		return
	}
	defer file.Close()

	rootDir := "." // 默认当前目录

	// 1. 写入项目目录结构
	file.WriteString("# 项目目录结构\n\n```text\n")
	generateTree(rootDir, "", file)
	file.WriteString("```\n\n")

	// 2. 写入源代码内容
	file.WriteString("# 源代码\n\n")
	appendFileContents(rootDir, file)

	fmt.Printf("✅ 上下文已成功导出至: %s\n", outputFile)
}

// generateTree 递归生成目录树视图
func generateTree(path string, indent string, file *os.File) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for i, entry := range entries {
		if entry.IsDir() && ignoreDirs[entry.Name()] {
			continue
		}
		// 忽略导出工具自身和生成的结果文件
		if entry.Name() == "export_context.go" || entry.Name() == outputFile {
			continue
		}

		isLast := i == len(entries)-1
		prefix := "├── "
		if isLast {
			prefix = "└── "
		}

		file.WriteString(fmt.Sprintf("%s%s%s\n", indent, prefix, entry.Name()))

		if entry.IsDir() {
			newIndent := indent + "│   "
			if isLast {
				newIndent = indent + "    "
			}
			generateTree(filepath.Join(path, entry.Name()), newIndent, file)
		}
	}
}

// appendFileContents 遍历并追加允许的文件内容
func appendFileContents(root string, outFile *os.File) {
	// 使用 WalkDir 比 Walk 更高效，因为它避免了每次访问都调用 os.Stat
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 处理目录跳过逻辑
		if d.IsDir() {
			if path != root && ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// 忽略自身和输出文件
		if d.Name() == "export_context.go" || d.Name() == outputFile {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if allowedExts[ext] || d.Name() == "Makefile" || d.Name() == "Dockerfile" {
			content, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("读取文件失败 %s: %v\n", path, err)
				return nil
			}

			// 使用 Markdown 代码块包裹，大模型能精准识别文件名和语言类型
			lang := strings.TrimPrefix(ext, ".")
			outFile.WriteString(fmt.Sprintf("## File: %s\n\n```%s\n%s\n```\n\n", path, lang, string(content)))
		}

		return nil
	})
}
