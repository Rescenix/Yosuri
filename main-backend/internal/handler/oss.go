package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gin-gonic/gin"
)

var (
	cacheFilePath = filepath.Join("data", "image_cache.json")
	imageLocalDir = filepath.Join("data", "images")
	mu            sync.RWMutex
)

type ImageRecord struct {
	RelPath   string   `json:"rel_path"`   // 例: "img/1700000000.jpg"
	LocalPath string   `json:"local_path"` // 本地绝对路径
	Tags      []string `json:"tags"`
	Size      int64    `json:"size"`
	CreatedAt string   `json:"created_at"`
}

func init() {
	os.MkdirAll(imageLocalDir, 0755)
	os.MkdirAll(filepath.Dir(cacheFilePath), 0755)
}

// ---------- 缓存读写（内部线程安全）----------
func loadCache() []ImageRecord {
	mu.RLock()
	defer mu.RUnlock()
	return loadCacheNoLock()
}

func loadCacheNoLock() []ImageRecord {
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return []ImageRecord{}
	}
	var records []ImageRecord
	_ = json.Unmarshal(data, &records)
	// 过滤掉 rel_path 为空的脏数据
	valid := make([]ImageRecord, 0, len(records))
	for _, r := range records {
		if r.RelPath != "" {
			valid = append(valid, r)
		}
	}
	return valid
}

func saveCache(records []ImageRecord) error {
	mu.Lock()
	defer mu.Unlock()
	return saveCacheNoLock(records)
}

func saveCacheNoLock(records []ImageRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFilePath, data, 0644)
}

// ---------- 辅助函数 ----------
func getContentType(key string) string {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// ---------- API 实现 ----------
func UploadToOSS(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法打开文件"})
		return
	}
	defer src.Close()

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	client, err := oss.New(
		os.Getenv("OSS_ENDPOINT"),
		os.Getenv("OSS_ACCESS_KEY_ID"),
		os.Getenv("OSS_ACCESS_KEY_SECRET"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OSS初始化失败"})
		return
	}
	bucket, err := client.Bucket(os.Getenv("OSS_BUCKET_NAME"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bucket访问失败"})
		return
	}

	ext := filepath.Ext(file.Filename)
	relPath := fmt.Sprintf("img/%d%s", time.Now().UnixNano(), ext)

	// 上传到 OSS
	if err := bucket.PutObject(relPath, strings.NewReader(string(fileBytes))); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OSS上传失败"})
		return
	}

	// 保存本地缓存
	localPath := filepath.Join(imageLocalDir, relPath)
	os.MkdirAll(filepath.Dir(localPath), 0755)
	os.WriteFile(localPath, fileBytes, 0644)

	// 写入映射表
	record := ImageRecord{
		RelPath:   relPath,
		LocalPath: localPath,
		Tags:      []string{}, // 默认无标签
		Size:      file.Size,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	records := loadCache()
	records = append(records, record)
	saveCache(records)

	// 返回代理 URL
	proxyURL := fmt.Sprintf("/api/images/view?rel_path=%s", relPath)
	c.JSON(http.StatusOK, gin.H{"url": proxyURL})
}

func ListImages(c *gin.Context) {
	records := loadCache()
	resp := make([]gin.H, 0, len(records))
	for _, r := range records {
		resp = append(resp, gin.H{
			"url":        fmt.Sprintf("/api/images/view?rel_path=%s", r.RelPath),
			"rel_path":   r.RelPath,
			"tags":       r.Tags,
			"size":       r.Size,
			"created_at": r.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func ViewImage(c *gin.Context) {
	relPath := c.Query("rel_path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 rel_path 参数"})
		return
	}

	// 1. 从本地缓存读取（使用 LocalPath 字段）
	records := loadCache()
	for _, r := range records {
		if r.RelPath == relPath {
			if data, err := os.ReadFile(r.LocalPath); err == nil {
				c.Data(http.StatusOK, getContentType(relPath), data)
				return
			}
			// 本地文件丢失，继续回源
			break
		}
	}

	// 2. 回源 OSS
	client, err := oss.New(
		os.Getenv("OSS_ENDPOINT"),
		os.Getenv("OSS_ACCESS_KEY_ID"),
		os.Getenv("OSS_ACCESS_KEY_SECRET"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OSS初始化失败"})
		return
	}
	bucket, err := client.Bucket(os.Getenv("OSS_BUCKET_NAME"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bucket访问失败"})
		return
	}
	body, err := bucket.GetObject(relPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return
	}
	defer body.Close()
	data, _ := io.ReadAll(body)

	// 异步写入本地缓存（并更新映射表）
	go func() {
		localPath := filepath.Join(imageLocalDir, relPath)
		os.MkdirAll(filepath.Dir(localPath), 0755)
		os.WriteFile(localPath, data, 0644)

		mu.Lock()
		defer mu.Unlock()
		records := loadCacheNoLock()
		found := false
		for i, r := range records {
			if r.RelPath == relPath {
				records[i].LocalPath = localPath
				found = true
				break
			}
		}
		if !found {
			records = append(records, ImageRecord{
				RelPath:   relPath,
				LocalPath: localPath,
				Tags:      []string{},
				Size:      int64(len(data)),
				CreatedAt: time.Now().Format(time.RFC3339),
			})
		}
		saveCacheNoLock(records)
	}()

	c.Data(http.StatusOK, getContentType(relPath), data)
}

func UpdateImageTag(c *gin.Context) {
	var req struct {
		RelPath string   `json:"rel_path"`
		Tags    []string `json:"tags"`
	}
	// 增加详细的错误输出
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数解析失败: " + err.Error()})
		return
	}
	if req.RelPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rel_path 不能为空"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	records := loadCacheNoLock()
	for i, r := range records {
		if r.RelPath == req.RelPath {
			records[i].Tags = req.Tags
			if err := saveCacheNoLock(records); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
}

func DeleteImage(c *gin.Context) {
	var req struct {
		RelPaths []string `json:"rel_paths"` // 支持批量删除
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if len(req.RelPaths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rel_paths 不能为空"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	records := loadCacheNoLock()
	newRecords := make([]ImageRecord, 0, len(records))

	// 初始化 OSS 客户端（只初始化一次）
	client, err := oss.New(
		os.Getenv("OSS_ENDPOINT"),
		os.Getenv("OSS_ACCESS_KEY_ID"),
		os.Getenv("OSS_ACCESS_KEY_SECRET"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OSS初始化失败"})
		return
	}
	bucket, err := client.Bucket(os.Getenv("OSS_BUCKET_NAME"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bucket访问失败"})
		return
	}

	for _, rec := range records {
		if contains(req.RelPaths, rec.RelPath) {
			// 删除 OSS
			_ = bucket.DeleteObject(rec.RelPath)
			// 删除本地文件
			_ = os.Remove(rec.LocalPath)
			// 不加入新列表
			continue
		}
		newRecords = append(newRecords, rec)
	}

	if err := saveCacheNoLock(newRecords); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新缓存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// DeleteTag 删除指定标签（从所有图片的 tags 数组中移除）
func DeleteTag(c *gin.Context) {
	var req struct {
		Tag string `json:"tag"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效标签"})
		return
	}

	mu.Lock()
	defer mu.Unlock()
	records := loadCacheNoLock()
	changed := false

	for i := range records {
		newTags := make([]string, 0, len(records[i].Tags))
		for _, t := range records[i].Tags {
			if t != req.Tag {
				newTags = append(newTags, t)
			}
		}
		if len(newTags) != len(records[i].Tags) {
			records[i].Tags = newTags
			changed = true
		}
	}

	if !changed {
		c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		return
	}

	if err := saveCacheNoLock(records); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// RandomImageWithAI 随机返回一张图片及其 AI 评价（按日期缓存）
func RandomImageWithAI(c *gin.Context) {
	force := c.Query("force") == "true"
	date := time.Now().Format("2006-01-02")
	cacheKey := fmt.Sprintf("daily_ai:%s", date)

	// 尝试从 Redis 读取缓存（如果启用了 Redis）
	if !force && redisEnabled {
		val, err := redisClient.Get(context.Background(), cacheKey).Result()
		if err == nil {
			var cached struct {
				ImageURL string `json:"imageUrl"`
				Comment  string `json:"comment"`
			}
			if json.Unmarshal([]byte(val), &cached) == nil {
				c.JSON(http.StatusOK, gin.H{
					"imageUrl": cached.ImageURL,
					"comment":  cached.Comment,
				})
				return
			}
		}
	}

	// 获取所有图片记录
	records := loadCache()
	if len(records) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "暂无图片"})
		return
	}

	// 随机选一张
	rand.Seed(time.Now().UnixNano())
	selected := records[rand.Intn(len(records))]
	imageURL := fmt.Sprintf("/api/images/view?rel_path=%s", selected.RelPath)

	// 调用 AI 分析图片（复用你的 AnalyzeImage 函数）
	question := "请模仿柳永的风格写一首青词,50字内"
	aiComment := askDeepSeekSimple(question)

	// 缓存到 Redis（有效期至次日零点）
	if redisEnabled {
		cacheData := struct {
			ImageURL string `json:"imageUrl"`
			Comment  string `json:"comment"`
		}{ImageURL: imageURL, Comment: aiComment}
		data, _ := json.Marshal(cacheData)
		// 设置过期时间为第二天凌晨 0 点
		tomorrow := time.Now().Add(24 * time.Hour)
		nextMidnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
		ttl := nextMidnight.Sub(time.Now())
		redisClient.Set(context.Background(), cacheKey, data, ttl)
	}

	c.JSON(http.StatusOK, gin.H{
		"imageUrl": imageURL,
		"comment":  aiComment,
	})
}
