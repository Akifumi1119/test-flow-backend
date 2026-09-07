package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
	"task-management/backend/internal/storage"
)

type CommentHandler struct {
	db      *gorm.DB
	storage *storage.Client
}

func NewCommentHandler(db *gorm.DB, storage *storage.Client) *CommentHandler {
	return &CommentHandler{db: db, storage: storage}
}

// createCommentRequest はmultipart/form-dataで受け取る（画像ファイルを含むため）。
type createCommentRequest struct {
	UserID  uint   `form:"user_id" binding:"required"`
	TaskID  uint   `form:"task_id" binding:"required"`
	Comment string `form:"comment" binding:"required"`
}

type createCommentResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	CommentID uint      `json:"comment_id"`
	Comment   string    `json:"comment"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Images    []string  `json:"images"`
}

func (h *CommentHandler) CreateComment(c *gin.Context) {
	var req createCommentRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var task model.Task
	if err := h.db.First(&task, req.TaskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	comment := model.Comment{
		TaskID:    req.TaskID,
		Content:   req.Comment,
		CreatedBy: req.UserID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// 画像をアップロードしてDBに保存する
	imageURLs := h.uploadImages(c, "images", func(url, publicID string) error {
		return h.db.Create(&model.CommentImage{
			CommentID: comment.CommentID,
			ImageURL:  url,
			PublicID:  publicID,
			CreatedAt: time.Now(),
		}).Error
	})

	c.JSON(http.StatusOK, createCommentResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		CommentID: comment.CommentID,
		Comment:   comment.Content,
		CreatedBy: user.Name,
		CreatedAt: comment.CreatedAt,
		Images:    imageURLs,
	})
}

type updateCommentRequest struct {
	UserID    uint   `json:"user_id" binding:"required"`
	CommentID uint   `json:"comment_id" binding:"required"`
	Comment   string `json:"comment" binding:"required"`
}

type updateCommentResponse struct {
	CommentID uint      `json:"comment_id"`
	Comment   string    `json:"comment"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *CommentHandler) UpdateComment(c *gin.Context) {
	var req updateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var comment model.Comment
	if err := h.db.First(&comment, req.CommentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	comment.Content = req.Comment
	comment.UpdatedAt = time.Now()

	if err := h.db.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, updateCommentResponse{
		CommentID: comment.CommentID,
		Comment:   comment.Content,
		UpdatedAt: comment.UpdatedAt,
	})
}

type deleteCommentRequest struct {
	UserID    uint `json:"user_id" binding:"required"`
	CommentID uint `json:"comment_id" binding:"required"`
}

func (h *CommentHandler) DeleteComment(c *gin.Context) {
	var req deleteCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var comment model.Comment
	if err := h.db.First(&comment, req.CommentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "コメントが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// コメントに紐づく画像をCloudinaryから削除する
	h.deleteCommentImagesFromStorage(comment.CommentID)

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("comment_id = ?", comment.CommentID).Delete(&model.CommentImage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&comment).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"comment_id": req.CommentID})
}

func (h *CommentHandler) uploadImages(c *gin.Context, field string, save func(url, publicID string) error) []string {
	urls := []string{}
	if h.storage == nil {
		return urls
	}
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		return urls
	}
	files := form.File[field]
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		result, err := h.storage.Upload(f, "task-management")
		f.Close()
		if err != nil {
			continue
		}
		if result.URL == "" {
			continue
		}
		if err := save(result.URL, result.PublicID); err != nil {
			continue
		}
		urls = append(urls, result.URL)
	}
	return urls
}

func (h *CommentHandler) deleteCommentImagesFromStorage(commentID uint) {
	if h.storage == nil {
		return
	}
	var images []model.CommentImage
	h.db.Where("comment_id = ?", commentID).Find(&images)
	for _, img := range images {
		h.storage.Delete(img.PublicID)
	}
}
