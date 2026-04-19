package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const outputFile = "project_context.md"

var ignoreDirs = map[string]bool{
	".git":         true,
	".idea":        true,
	".vscode":      true,
	"vendor":       true,
	"node_modules": true,
	"bin":          true,
	"data":         true,
}

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

	rootDir := "."

	file.WriteString("# 项目目录结构\n\n```text\n")
	generateTree(rootDir, "", file)
	file.WriteString("```\n\n")

	file.WriteString("# 源代码\n\n")
	appendFileContents(rootDir, file)

	fmt.Printf("✅ 上下文已成功导出至: %s\n", outputFile)
}

func generateTree(path string, indent string, file *os.File) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for i, entry := range entries {
		if entry.IsDir() && ignoreDirs[entry.Name()] {
			continue
		}
		if path == "cmd/export-context" && entry.Name() == "main.go" {
			continue
		}
		if entry.Name() == outputFile {
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

func appendFileContents(root string, outFile *os.File) {
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if path != root && ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if path == "cmd/export-context/main.go" || d.Name() == outputFile {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if allowedExts[ext] || d.Name() == "Makefile" || d.Name() == "Dockerfile" {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				fmt.Printf("读取文件失败 %s: %v\n", path, readErr)
				return nil
			}

			lang := strings.TrimPrefix(ext, ".")
			outFile.WriteString(fmt.Sprintf("## File: %s\n\n```%s\n%s\n```\n\n", path, lang, string(content)))
		}

		return nil
	})
}
