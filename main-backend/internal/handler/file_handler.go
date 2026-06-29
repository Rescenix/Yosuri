package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileNode struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "file" or "folder"
	Path     string      `json:"path,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// FileTreeHandler 返回项目目录树
func FileTreeHandler(w http.ResponseWriter, r *http.Request) {
	root := GitRepoRoot
	tree, err := buildFileTree(root, root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

// FileReadHandler 读取单个文件内容
func FileReadHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(GitRepoRoot, filePath)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(GitRepoRoot)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot read %s: %v", fullPath, err), http.StatusNotFound)
		return
	}
	w.Write(content)
}

// buildFileTree 递归构建文件树，并按 VS Code 规则排序
func buildFileTree(root string, current string) ([]*FileNode, error) {
	entries, err := os.ReadDir(current)
	if err != nil {
		return nil, err
	}

	var nodes []*FileNode
	for _, entry := range entries {
		name := entry.Name()
		// 跳过不关心的目录
		if name == ".git" || name == "node_modules" || name == "vendor" {
			continue
		}

		fullPath := filepath.Join(current, name)
		relPath, _ := filepath.Rel(root, fullPath)
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		if entry.IsDir() {
			children, err := buildFileTree(root, fullPath)
			if err != nil {
				continue
			}
			nodes = append(nodes, &FileNode{
				Name:     name,
				Type:     "folder",
				Path:     relPath,
				Children: children,
			})
		} else {
			nodes = append(nodes, &FileNode{
				Name: name,
				Type: "file",
				Path: relPath,
			})
		}
	}

	// 排序：文件夹 > 点文件 > 普通文件，同类按字母序
	sort.Slice(nodes, func(i, j int) bool {
		ni, nj := nodes[i], nodes[j]
		// 1. 文件夹优先
		if ni.Type != nj.Type {
			return ni.Type == "folder"
		}
		// 2. 同是文件时，点文件优先
		if ni.Type == "file" {
			dotI := strings.HasPrefix(ni.Name, ".")
			dotJ := strings.HasPrefix(nj.Name, ".")
			if dotI != dotJ {
				return dotI
			}
		}
		// 3. 字母序
		return strings.ToLower(ni.Name) < strings.ToLower(nj.Name)
	})

	return nodes, nil
}
