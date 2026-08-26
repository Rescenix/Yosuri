package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backend/internal/ai/core"
)

// skipTreeDirs 是构建文件树时整个跳过的目录名（不进 children，也不递归）。
var skipTreeDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".gocache": true, ".mimocode": true, // 各几千个文件的构建缓存/工具目录，浏览器点不动
	"__pycache__": true, ".pytest_cache": true,
}

type FileNode struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "file" or "folder"
	Path     string      `json:"path,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// watchedFileVersion 是文件工具用来识别磁盘变化的轻量指纹。我们只监视客户端
// 当前打开的少数文件，而不是递归监听整个仓库（node_modules 等目录会产生海量事件）。
type watchedFileVersion struct {
	modified time.Time
	size     int64
	exists   bool
}

func watchedVersion(fullPath string) watchedFileVersion {
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return watchedFileVersion{}
	}
	return watchedFileVersion{modified: info.ModTime(), size: info.Size(), exists: true}
}

func (v watchedFileVersion) changedFrom(other watchedFileVersion) bool {
	return v.exists != other.exists || v.size != other.size || !v.modified.Equal(other.modified)
}

// FileChangesHandler 为已打开的文件提供 SSE 变更通知。其他 agent、终端和这个
// 编辑器自己的写入都会落到同一个磁盘，因此轮询 stat 是跨写入来源最可靠的共同点。
// 前端收到 path 后再 GET 内容，避免把文件正文长期缓存或塞进 SSE 流。
func FileChangesHandler(w http.ResponseWriter, r *http.Request) {
	rawPaths := strings.Split(r.URL.Query().Get("paths"), ",")
	versions := make(map[string]watchedFileVersion, len(rawPaths))
	for _, path := range rawPaths {
		path = strings.TrimSpace(path)
		fullPath, ok := resolveRepoPath(path)
		if path == "" || !ok {
			continue
		}
		versions[path] = watchedVersion(fullPath)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(350 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			for path, previous := range versions {
				fullPath, _ := resolveRepoPath(path)
				current := watchedVersion(fullPath)
				if !current.changedFrom(previous) {
					continue
				}
				versions[path] = current
				data, _ := json.Marshal(map[string]string{"path": path})
				fmt.Fprintf(w, "event: changed\ndata: %s\n\n", data)
			}
			flusher.Flush()
		}
	}
}

// FileTreeHandler 返回项目目录树
func FileTreeHandler(w http.ResponseWriter, r *http.Request) {
	root := currentFileRoot()
	tree, err := buildFileTree(root, root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

// FileReadHandler 读取单个文件内容
// maxReadableFileBytes 是「文件」工具单次可读取的上限。仓库根目录和 main-backend/models/
// 下都躺着几百 MB～几 GB 的二进制（.exe/.jar/.gguf）——不拦的话点开一个就是把整个文件读进
// 内存塞给浏览器，Monaco 拿几 GB 字符串糊自己，标签页直接卡死。真实源码文件基本不可能碰到
// 这个上限，碰到了大概率本来就不该被当文本打开。
const maxReadableFileBytes = 3 * 1024 * 1024 // 3MB

func FileReadHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	fullPath, ok := resolveRepoPath(filePath)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot stat %s: %v", fullPath, err), http.StatusNotFound)
		return
	}
	if info.IsDir() {
		http.Error(w, "path is a directory", http.StatusBadRequest)
		return
	}
	if info.Size() > maxReadableFileBytes {
		http.Error(w, fmt.Sprintf("文件过大（%.1fMB），超过 %dMB 上限，可能不是文本文件",
			float64(info.Size())/1024/1024, maxReadableFileBytes/1024/1024), http.StatusRequestEntityTooLarge)
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot read %s: %v", fullPath, err), http.StatusNotFound)
		return
	}
	w.Write(content)
}

// fileWriteRequest 是「文件」工具编辑器保存时的请求体
type fileWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FileWriteHandler 保存单个文件内容（POST /api/file）。跟 FileReadHandler 共用同一套
// 仓库根越界拦截——这是给人在「文件」工具里直接点保存用的，不是 agent 工具调用，
// 不经过 Ask/Yolo 审批链路（approval.go 那套是给 LLM 发起的工具调用用的）。
func FileWriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req fileWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	fullPath, ok := resolveRepoPath(req.Path)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// 新文件所在目录可能还不存在（编辑器里新建的文件、或树上还没有的路径）；
	// os.WriteFile 不会自动建父目录，不 MkdirAll 会直接 500。
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		http.Error(w, fmt.Sprintf("cannot create directory for %s: %v", fullPath, err), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		http.Error(w, fmt.Sprintf("cannot write %s: %v", fullPath, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// FileCreateFolderHandler 创建空目录。写文件时会自动创建父级目录，但教学演示或
// 新项目起步时，用户也需要能先把目录结构搭出来。
func FileCreateFolderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	fullPath, ok := resolveRepoPath(req.Path)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		http.Error(w, fmt.Sprintf("cannot create folder %s: %v", req.Path, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

type fileRenameRequest struct {
	Path    string `json:"path"`
	NewPath string `json:"newPath"`
}

// FileRenameHandler 重命名文件或目录。目标已存在时拒绝覆盖，避免一次误操作损失内容。
func FileRenameHandler(w http.ResponseWriter, r *http.Request) {
	var req fileRenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	from, fromOK := resolveRepoPath(req.Path)
	to, toOK := resolveRepoPath(req.NewPath)
	if !fromOK || !toOK || from == currentFileRoot() || strings.TrimSpace(req.NewPath) == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if _, err := os.Stat(to); err == nil {
		http.Error(w, "destination already exists", http.StatusConflict)
		return
	} else if !os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(from, to); err != nil {
		http.Error(w, fmt.Sprintf("cannot rename %s: %v", req.Path, err), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// FileDeleteHandler 删除项目根内的单个文件或目录；根目录本身永远不可删除。
func FileDeleteHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	fullPath, ok := resolveRepoPath(path)
	if !ok || path == "" || fullPath == currentFileRoot() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if info.IsDir() {
		err = os.RemoveAll(fullPath)
	} else {
		err = os.Remove(fullPath)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot delete %s: %v", path, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// currentFileRoot 让编辑器跟随当前对话工作目录；GitRepoRoot 仍只服务 git 操作和
// 工作目录的相对路径解析，不能再拿它当文件面板的根，否则会出现“对话已切项目、
// 编辑器仍显示 re0”的脱节。
func currentFileRoot() string {
	return filepath.Clean(core.GetProjectRoot())
}

// resolveRepoPath 把请求里的相对路径解析成绝对路径，并确认没跑出当前工作目录。
// FileReadHandler / FileWriteHandler 共用，越界拦截逻辑只写一份。
func resolveRepoPath(relPath string) (string, bool) {
	root := currentFileRoot()
	fullPath := filepath.Clean(filepath.Join(root, relPath))
	if fullPath != root && !strings.HasPrefix(fullPath, root+string(filepath.Separator)) {
		return "", false
	}
	return fullPath, true
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
		// 跳过不关心的目录：.gocache(Go 构建缓存)/.mimocode 各有几千个文件，
		// 不跳的话「文件」工具的树会被这些垃圾目录塞爆——2026-07-24 实测过。
		if skipTreeDirs[name] {
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
