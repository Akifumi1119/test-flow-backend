package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

type CommentHandler struct {
	db *gorm.DB
}

func NewCommentHandler(db *gorm.DB) *CommentHandler {
	return &CommentHandler{db: db}
}

type createCommentRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	TaskID  uint   `json:"task_id" binding:"required"`
	Comment string `json:"comment" binding:"required"`
}

type createCommentResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	CommentID uint      `json:"comment_id"`
	Comment   string    `json:"comment"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *CommentHandler) CreateComment(c *gin.Context) {
	var req createCommentRequest
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

	c.JSON(http.StatusOK, createCommentResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		CommentID: comment.CommentID,
		Comment:   comment.Content,
		CreatedBy: user.Name,
		CreatedAt: comment.CreatedAt,
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

	if err := h.db.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"comment_id": req.CommentID})
}
