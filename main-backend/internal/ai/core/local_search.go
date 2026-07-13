package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
)

var (
	codeIndex   bleve.Index
	codeIndexMu sync.RWMutex
)

// 项目根目录改成走 GetProjectRoot()，不再是编译期写死的 const——workdir 切换后代码搜索索引也要跟着变

// InitCodebaseIndex 扫描项目目录，存储相对路径，避免路径重复
func InitCodebaseIndex() error {
	codeIndexMu.Lock()
	defer codeIndexMu.Unlock()
	if codeIndex != nil {
		return nil
	}

	mapping := bleve.NewIndexMapping()
	idx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return fmt.Errorf("创建代码索引失败: %w", err)
	}

	root := GetProjectRoot()
	err = filepath.Walk(root, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("⚠️ 跳过无法访问的文件: %s (%v)\n", absPath, err)
			return nil
		}
		if info.IsDir() {
			dirName := strings.ToLower(info.Name())
			if dirName == ".git" || dirName == "node_modules" || dirName == "dist" ||
				dirName == "vendor" || dirName == "third_party" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(absPath))
		if ext != ".go" && ext != ".vue" && ext != ".js" && ext != ".ts" &&
			ext != ".cpp" && ext != ".h" && ext != ".md" {
			return nil
		}
		if info.Size() > 500*1024 {
			fmt.Printf("⚠️ 跳过大文件: %s (%d KB)\n", absPath, info.Size()/1024)
			return nil
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			fmt.Printf("⚠️ 跳过读取失败的文件: %s (%v)\n", absPath, err)
			return nil
		}

		// 关键修复：存储相对于项目根目录的相对路径
		relPath, err := filepath.Rel(root, absPath)
		if err != nil {
			fmt.Printf("⚠️ 无法计算相对路径: %s (%v)\n", absPath, err)
			return nil
		}
		// 统一使用正斜杠，避免 Windows 反斜杠问题
		relPath = filepath.ToSlash(relPath)

		doc := map[string]interface{}{
			"path":    relPath,
			"content": string(content),
			"type":    "code",
		}
		return idx.Index(relPath, doc)
	})
	if err != nil {
		return fmt.Errorf("扫描项目目录失败: %w", err)
	}
	codeIndex = idx
	fmt.Printf("✅ 项目代码索引构建完成（相对路径）。\n")
	return nil
}

// ResetCodebaseIndex 丢弃旧工作目录的内存索引。下一次搜索会按新的工作目录懒加载，
// 因而切换目录后不会把旧项目的文件错误地返回给 Agent。
func ResetCodebaseIndex() {
	codeIndexMu.Lock()
	defer codeIndexMu.Unlock()
	if codeIndex != nil {
		_ = codeIndex.Close()
		codeIndex = nil
	}
}

// SearchLocalCodebase 在本地代码索引中执行搜索，返回前 topK 个结果
func SearchLocalCodebase(query string, topK int) (string, error) {
	if err := InitCodebaseIndex(); err != nil {
		return "", err
	}
	codeIndexMu.RLock()
	defer codeIndexMu.RUnlock()
	if codeIndex == nil {
		return "", fmt.Errorf("代码索引尚未初始化")
	}

	queryStr := bleve.NewQueryStringQuery(query)
	searchReq := bleve.NewSearchRequest(queryStr)
	searchReq.Size = topK
	searchReq.Fields = []string{"path", "content"}

	result, err := codeIndex.Search(searchReq)
	if err != nil {
		return "", fmt.Errorf("搜索失败: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("为“%s”找到 %d 个结果：\n", query, len(result.Hits)))
	for i, hit := range result.Hits {
		path := hit.Fields["path"].(string)
		content := hit.Fields["content"].(string)
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, path, content))
	}
	return sb.String(), nil
}

// UpdateCodeIndex 更新单个文件在代码索引中的内容
func UpdateCodeIndex(filePath string) error {
	if err := InitCodebaseIndex(); err != nil {
		return err
	}
	codeIndexMu.RLock()
	defer codeIndexMu.RUnlock()
	if codeIndex == nil {
		return fmt.Errorf("代码索引尚未初始化")
	}

	// 统一转换为绝对路径，然后转为相对路径进行索引
	absPath := filePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(GetProjectRoot(), absPath)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		// 文件不存在，从索引中移除
		relPath, _ := filepath.Rel(GetProjectRoot(), absPath)
		if relPath != "" {
			return codeIndex.Delete(filepath.ToSlash(relPath))
		}
		return codeIndex.Delete(filePath)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	relPath, err := filepath.Rel(GetProjectRoot(), absPath)
	if err != nil {
		return fmt.Errorf("计算相对路径失败: %w", err)
	}
	relPath = filepath.ToSlash(relPath)

	doc := map[string]interface{}{
		"path":    relPath,
		"content": string(content),
		"type":    "code",
	}
	return codeIndex.Index(relPath, doc)
}
