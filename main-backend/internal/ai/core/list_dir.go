package core

// list_dir 工具的实现：只读列目录，紧凑输出，自动跳过噪声目录。
// 子代理（雨燕）只读白名单的核心成员——调研任务几乎都从"看目录结构"开始，
// 之前白名单里只有 read_file 读不了目录，统计类任务必然超轮数失败。

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const listDirMaxEntries = 500

// 递归时跳过的噪声目录（一层列举时仍显示目录名本身，只是不进入）
var listDirSkip = map[string]bool{
	"node_modules": true, ".git": true, "__pycache__": true,
	".gocache": true, "dist": true, "build": true, ".vite": true,
}

func listDirTool(root string, recursive bool) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s 不是目录", root)
	}

	var lines []string
	truncated := false

	if !recursive {
		entries, err := os.ReadDir(root)
		if err != nil {
			return "", err
		}
		sort.Slice(entries, func(i, j int) bool {
			// 目录在前，同类按名称
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})
		for _, e := range entries {
			if len(lines) >= listDirMaxEntries {
				truncated = true
				break
			}
			if e.IsDir() {
				lines = append(lines, e.Name()+"/")
			} else {
				lines = append(lines, e.Name())
			}
		}
	} else {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // 单个条目失败不中断整体
			}
			if path == root {
				return nil
			}
			if d.IsDir() && listDirSkip[d.Name()] {
				return filepath.SkipDir
			}
			if len(lines) >= listDirMaxEntries {
				truncated = true
				return filepath.SkipAll
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				rel += "/"
			}
			lines = append(lines, rel)
			return nil
		})
		if err != nil {
			return "", err
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s（%d 项）:\n", root, len(lines)))
	sb.WriteString(strings.Join(lines, "\n"))
	if truncated {
		sb.WriteString(fmt.Sprintf("\n...[超过 %d 项已截断]", listDirMaxEntries))
	}
	return sb.String(), nil
}
