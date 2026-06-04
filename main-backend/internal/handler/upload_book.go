package handler

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var indexMutex sync.Mutex

type BookEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Cover string `json:"cover,omitempty"` // 封面图片 Base64 编码
}

type BookIndex struct {
	Books []BookEntry `json:"books"`
}

func UploadBook(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未上传文件"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".txt") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 TXT 文件"})
		return
	}

	// 书名
	title := c.PostForm("title")
	if title == "" {
		title = strings.TrimSuffix(file.Filename, ".txt")
		title = strings.TrimSuffix(title, ".TXT")
	}
	title = sanitizeBookID(title)

	booksDir := GetBooksDir()
	if err := os.MkdirAll(booksDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建目录"})
		return
	}

	// 保存 TXT 文件
	savePath := filepath.Join(booksDir, title+".txt")
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}
	defer src.Close()

	rawData, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取内容失败"})
		return
	}

	utf8Data, err := convertToUTF8(rawData)
	if err != nil {
		utf8Data = rawData
	}

	if err := os.WriteFile(savePath, utf8Data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}

	// 处理封面图片（转为 Base64 存储）
	coverBase64 := ""
	if coverFile, err := c.FormFile("cover"); err == nil {
		src, err := coverFile.Open()
		if err == nil {
			defer src.Close()
			imgData, err := io.ReadAll(src)
			if err == nil {
				mimeType := "image/" + strings.TrimPrefix(filepath.Ext(coverFile.Filename), ".")
				coverBase64 = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(imgData)
			}
		}
	}

	// 更新索引
	indexMutex.Lock()
	defer indexMutex.Unlock()

	indexPath := filepath.Join(booksDir, "index.json")
	var index BookIndex
	if data, err := os.ReadFile(indexPath); err == nil {
		json.Unmarshal(data, &index)
	}
	if index.Books == nil {
		index.Books = []BookEntry{}
	}

	found := false
	// 在索引更新处
	for i, b := range index.Books {
		if b.ID == title {
			index.Books[i].Title = title
			index.Books[i].Cover = coverBase64 // 改为 Base64
			found = true
			break
		}
	}
	if !found {
		index.Books = append(index.Books, BookEntry{ID: title, Title: title, Cover: coverBase64})
	}
	idxData, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(indexPath, idxData, 0644)

	c.JSON(http.StatusOK, gin.H{
		"bookId":   title,
		"fileName": file.Filename,
		"books":    index.Books,
		"cover":    coverBase64,
		"message":  "上传成功",
	})
}

// sanitizeBookID 清理书名中的危险字符
func sanitizeBookID(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, name)
}
