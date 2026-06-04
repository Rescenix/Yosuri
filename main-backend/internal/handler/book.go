package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	bookList   []BookEntry
	bookListMu sync.Mutex
)

func InitBookList() {
	booksDir := GetBooksDir()
	indexPath := filepath.Join(booksDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		bookList = []BookEntry{}
		return
	}
	var index BookIndex
	if json.Unmarshal(data, &index) != nil {
		bookList = []BookEntry{}
		return
	}
	bookList = index.Books
	if bookList == nil {
		bookList = []BookEntry{}
	}
}

func ListBooks(c *gin.Context) {
	booksDir := GetBooksDir()
	indexPath := filepath.Join(booksDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		c.JSON(http.StatusOK, BookIndex{Books: []BookEntry{}})
		return
	}
	var index BookIndex
	if json.Unmarshal(data, &index) != nil {
		index.Books = []BookEntry{}
	}
	if index.Books == nil {
		index.Books = []BookEntry{}
	}
	c.JSON(http.StatusOK, index)
}

func GetBookContent(c *gin.Context) {
	bookID := c.Query("bookId")
	if bookID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 bookId"})
		return
	}

	// 尝试从 Redis 读取
	if redisEnabled {
		cacheKey := "book_content:" + bookID
		ctx := context.Background()
		if val, err := redisClient.Get(ctx, cacheKey).Result(); err == nil {
			c.String(http.StatusOK, val)
			return
		}
	}

	booksDir := GetBooksDir()
	filePath := filepath.Join(booksDir, bookID+".txt")
	data, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "书籍文件未找到"})
		return
	}
	text := string(data) // 文件已经是 UTF-8，直接使用

	// 异步写入 Redis（24 小时过期）
	if redisEnabled {
		go func() {
			ctx := context.Background()
			cacheKey := "book_content:" + bookID
			if err := redisClient.Set(ctx, cacheKey, text, 24*time.Hour).Err(); err != nil {
				fmt.Printf("[WARN] 缓存书籍内容失败: %v\n", err)
			}
		}()
	}

	c.String(http.StatusOK, text)
}

// 在 handler/book.go 中添加
func updateBookCoverInIndex(bookID, coverURL string) {
	indexMutex.Lock()
	defer indexMutex.Unlock()

	indexPath := filepath.Join(GetBooksDir(), "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}
	var index BookIndex
	if json.Unmarshal(data, &index) != nil {
		return
	}
	for i, b := range index.Books {
		if b.ID == bookID {
			index.Books[i].Cover = coverURL
			break
		}
	}
	idxData, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(indexPath, idxData, 0644)
}
func DeleteBook(c *gin.Context) {
	bookID := c.Query("bookId")
	if bookID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 bookId"})
		return
	}

	booksDir := GetBooksDir()

	// 1. 删除 TXT 文件
	txtPath := filepath.Join(booksDir, bookID+".txt")
	if err := os.Remove(txtPath); err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文件失败"})
		return
	}

	// 2. 删除封面图片（如果存在）
	coversDir := filepath.Join(booksDir, "covers")
	coverPattern := filepath.Join(coversDir, bookID+".*")
	if matches, err := filepath.Glob(coverPattern); err == nil {
		for _, path := range matches {
			if err := os.Remove(path); err != nil {
				fmt.Printf("[WARN] 删除封面文件失败: %s, %v\n", path, err)
			}
		}
	}

	// 3. 更新索引文件
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

	newBooks := make([]BookEntry, 0, len(index.Books))
	for _, b := range index.Books {
		if b.ID != bookID {
			newBooks = append(newBooks, b)
		}
	}
	index.Books = newBooks

	idxData, _ := json.MarshalIndent(index, "", "  ")
	if err := os.WriteFile(indexPath, idxData, 0644); err != nil {
		fmt.Printf("[ERROR] 写入索引文件失败: %v\n", err)
	}

	// 4. 清除 Redis 缓存（与本书相关的所有键）
	if redisEnabled {
		ctx := context.Background()
		// 安全删除：先搜索再批量删除
		keys, err := redisClient.Keys(ctx, fmt.Sprintf("*%s*", bookID)).Result()
		if err == nil && len(keys) > 0 {
			if err := redisClient.Del(ctx, keys...).Err(); err != nil {
				fmt.Printf("[WARN] 清除 Redis 缓存失败: %v\n", err)
			} else {
				fmt.Printf("[INFO] 已清除 Redis 缓存 %d 条\n", len(keys))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// 在 handler/book.go 中添加
func imageToBase64(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(filePath)
	mimeType := "image/" + strings.TrimPrefix(ext, ".")
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
func UploadCover(c *gin.Context) {
	bookID := c.PostForm("bookId")
	if bookID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 bookId"})
		return
	}
	coverFile, err := c.FormFile("cover")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未上传封面"})
		return
	}

	src, err := coverFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取封面失败"})
		return
	}
	defer src.Close()
	imgData, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取封面失败"})
		return
	}

	mimeType := "image/" + strings.TrimPrefix(filepath.Ext(coverFile.Filename), ".")
	coverBase64 := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	updateBookCoverInIndex(bookID, coverBase64)

	c.JSON(http.StatusOK, gin.H{"cover": coverBase64})
}
