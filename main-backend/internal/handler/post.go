package handler

import (
	"backend/internal/database"
	"backend/internal/model"
	"backend/internal/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetPosts 获取所有博客文章
func GetPosts(c *gin.Context) {
	posts, err := service.GetAllPosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if posts == nil {
		posts = []model.Post{}
	}
	c.JSON(http.StatusOK, posts)
}

// CreatePost 创建新文章
func CreatePost(c *gin.Context) {
	var req struct {
		Title       string   `json:"title"`
		Content     string   `json:"content"`
		Tags        []string `json:"tags"`
		Attachments []string `json:"attachments"`
		CoverImage  string   `json:"cover_image"`
		CustomURL   string   `json:"custom_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Title == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题和内容不能为空"})
		return
	}
	post := &model.Post{
		Title:       req.Title,
		Content:     req.Content,
		Tags:        req.Tags,
		Attachments: req.Attachments,
		CoverImage:  req.CoverImage,
		CustomURL:   req.CustomURL,
	}
	if err := service.CreateNewPost(post); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "发布成功", "post": post})
}

// DeletePost 删除文章
func DeletePost(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效ID"})
		return
	}
	if err := service.DeletePost(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 在 handler/post.go 中添加
func UpdatePostTags(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	var req struct {
		Tags []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	_, err = database.DB.Exec("UPDATE posts SET tags = ? WHERE id = ?", tagsJSON, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新标签失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "标签更新成功"})
}
