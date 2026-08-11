package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// writeCompanyInventoryCSV 把公司磁盘上的真实交付物整理成 Excel 可直接使用的数据表。
// 它不让模型编数字；文件大小、时间、路径和类型都来自 os.Stat。
func writeCompanyInventoryCSV(home string) (string, error) {
	companyRoot := filepath.Dir(home)
	if filepath.Base(companyRoot) != "company" {
		return "", fmt.Errorf("当前 Agent 不在公司目录")
	}
	outDir := filepath.Join(home, "outputs")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("Excel-生产清单-%s-%02d.csv", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
	file, err := os.Create(filepath.Join(outDir, name))
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, _ = file.Write([]byte{0xEF, 0xBB, 0xBF}) // Excel 直接打开中文不乱码
	w := csv.NewWriter(file)
	defer w.Flush()
	_ = w.Write([]string{"Agent", "部门", "区域", "文件名", "扩展名", "字节", "修改时间", "相对路径"})
	err = filepath.WalkDir(companyRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil || entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(companyRoot, path)
		if relErr != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 3 || (parts[1] != "outputs" && parts[1] != "projects") {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil
		}
		role := strings.SplitN(parts[0], "-", 2)[0]
		return w.Write([]string{parts[0], role, parts[1], entry.Name(), strings.ToLower(filepath.Ext(entry.Name())), strconv.FormatInt(info.Size(), 10), info.ModTime().Format("2006-01-02 15:04:05"), filepath.ToSlash(rel)})
	})
	w.Flush()
	if err != nil {
		return "", err
	}
	if err := w.Error(); err != nil {
		return "", err
	}
	return name, nil
}
