package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type novelChapter struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type novelBook struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Genre        string         `json:"genre"`
	Summary      string         `json:"summary"`
	Style        string         `json:"style"`
	Cover        string         `json:"cover"`
	Platforms    []string       `json:"platforms"`
	Chapters     []novelChapter `json:"chapters"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	LastOpenedAt time.Time      `json:"lastOpenedAt"`
}

var novelLibraryMu sync.Mutex

func novelLibraryPath() string {
	if root := strings.TrimSpace(os.Getenv("RESCENE_NOVELS_DIR")); root != "" {
		return filepath.Join(root, "books.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "novels", "books.json")
}

func loadNovelBooks() ([]novelBook, error) {
	data, err := os.ReadFile(novelLibraryPath())
	if os.IsNotExist(err) {
		return []novelBook{}, nil
	}
	if err != nil {
		return nil, err
	}
	var books []novelBook
	if err := json.Unmarshal(data, &books); err != nil {
		return nil, err
	}
	return books, nil
}

func saveNovelBooks(books []novelBook) error {
	path := novelLibraryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(books, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func sortNovelBooks(books []novelBook) {
	sort.SliceStable(books, func(i, j int) bool { return books[i].LastOpenedAt.After(books[j].LastOpenedAt) })
}

// HandleNovelBooks GET /api/publish/books —— 最近打开的书排在第一位。
func HandleNovelBooks(c *gin.Context) {
	novelLibraryMu.Lock()
	defer novelLibraryMu.Unlock()
	books, err := loadNovelBooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取书架失败: " + err.Error()})
		return
	}
	sortNovelBooks(books)
	c.JSON(http.StatusOK, gin.H{"books": books})
}

type novelBookRequest struct {
	Title     string   `json:"title" binding:"required"`
	Genre     string   `json:"genre"`
	Summary   string   `json:"summary"`
	Style     string   `json:"style"`
	Cover     string   `json:"cover"`
	Platforms []string `json:"platforms"`
}

func validateNovelCover(cover string) error {
	if len(cover) > 4*1024*1024 {
		return fmt.Errorf("封面不能超过 3MB")
	}
	if cover != "" && !strings.HasPrefix(cover, "data:image/") {
		return fmt.Errorf("封面格式无效")
	}
	return nil
}

// HandleCreateNovelBook POST /api/publish/books
func HandleCreateNovelBook(c *gin.Context) {
	var req novelBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写书名"})
		return
	}
	if err := validateNovelCover(req.Cover); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	now := time.Now()
	book := novelBook{ID: fmt.Sprintf("book-%d", now.UnixNano()), Title: strings.TrimSpace(req.Title), Genre: strings.TrimSpace(req.Genre), Summary: strings.TrimSpace(req.Summary), Style: strings.TrimSpace(req.Style), Cover: req.Cover, Platforms: req.Platforms, Chapters: []novelChapter{}, CreatedAt: now, UpdatedAt: now, LastOpenedAt: now}
	novelLibraryMu.Lock()
	defer novelLibraryMu.Unlock()
	books, err := loadNovelBooks()
	if err == nil {
		books = append(books, book)
		err = saveNovelBooks(books)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存书籍失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"book": book})
}

// HandleUpdateNovelBook PUT /api/publish/books/:id —— 修改书名、封面和长期设定。
func HandleUpdateNovelBook(c *gin.Context) {
	var req novelBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写书名"})
		return
	}
	if err := validateNovelCover(req.Cover); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	novelLibraryMu.Lock()
	defer novelLibraryMu.Unlock()
	books, err := loadNovelBooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range books {
		if books[i].ID != c.Param("id") {
			continue
		}
		books[i].Title = strings.TrimSpace(req.Title)
		books[i].Genre = strings.TrimSpace(req.Genre)
		books[i].Summary = strings.TrimSpace(req.Summary)
		books[i].Style = strings.TrimSpace(req.Style)
		books[i].Cover = req.Cover
		books[i].Platforms = req.Platforms
		books[i].UpdatedAt = time.Now()
		if err := saveNovelBooks(books); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"book": books[i]})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "书籍不存在"})
}

// HandleDeleteNovelBook DELETE /api/publish/books/:id
func HandleDeleteNovelBook(c *gin.Context) {
	novelLibraryMu.Lock()
	defer novelLibraryMu.Unlock()
	books, err := loadNovelBooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range books {
		if books[i].ID != c.Param("id") {
			continue
		}
		deleted := books[i]
		books = append(books[:i], books[i+1:]...)
		if err := saveNovelBooks(books); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": deleted.ID})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "书籍不存在"})
}

// HandleOpenNovelBook POST /api/publish/books/:id/open
func HandleOpenNovelBook(c *gin.Context) {
	novelLibraryMu.Lock()
	defer novelLibraryMu.Unlock()
	books, err := loadNovelBooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range books {
		if books[i].ID == c.Param("id") {
			books[i].LastOpenedAt = time.Now()
			books[i].UpdatedAt = books[i].LastOpenedAt
			if err := saveNovelBooks(books); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"book": books[i]})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "书籍不存在"})
}

type novelChapterRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// HandleSaveNovelChapter POST /api/publish/books/:id/chapters —— 默认保存为草稿，不触发外部发布。
func HandleSaveNovelChapter(c *gin.Context) {
	var req novelChapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "章节标题和正文不能为空"})
		return
	}
	if len([]rune(req.Content)) > 120000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单章不能超过 120000 字"})
		return
	}
	novelLibraryMu.Lock()
	defer novelLibraryMu.Unlock()
	books, err := loadNovelBooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range books {
		if books[i].ID != c.Param("id") {
			continue
		}
		now := time.Now()
		chapter := novelChapter{ID: fmt.Sprintf("chapter-%d", now.UnixNano()), Title: strings.TrimSpace(req.Title), Content: req.Content, Status: "draft", UpdatedAt: now}
		books[i].Chapters = append(books[i].Chapters, chapter)
		books[i].UpdatedAt, books[i].LastOpenedAt = now, now
		if err := saveNovelBooks(books); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"book": books[i], "chapter": chapter})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "书籍不存在"})
}
