package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

type updateUserProjectItem struct {
	ProjectID   uint   `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type updateUserRequest struct {
	Name     string                  `json:"name"`
	Email    string                  `json:"email" binding:"omitempty,email"`
	Projects []updateUserProjectItem `json:"projects"`
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	userIDParam := c.Param("id")
	userID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不正なユーザーIDです。"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "入力内容が正しくありません。"})
		return
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "ユーザーが見つかりません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// メールアドレスのバリデーション（変更がある場合のみ）
	if req.Email != "" && req.Email != user.Email {
		var existing model.User
		err := h.db.Where("email = ? AND user_id != ?", req.Email, userID).First(&existing).Error
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"message": "このメールアドレスはすでに使用されています。"})
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
			return
		}
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{}
		if req.Name != "" {
			updates["name"] = req.Name
		}
		if req.Email != "" {
			updates["email"] = req.Email
		}
		if len(updates) > 0 {
			if err := tx.Model(&user).Updates(updates).Error; err != nil {
				return err
			}
		}

		// リクエストに含まれるプロジェクトのみ脱退（含まれないプロジェクトは操作しない）
		if len(req.Projects) > 0 {
			deleteIDs := make([]uint, len(req.Projects))
			for i, p := range req.Projects {
				deleteIDs[i] = p.ProjectID
			}
			return tx.Where("user_id = ? AND project_id IN ?", userID, deleteIDs).Delete(&model.ProjectMember{}).Error
		}

		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ユーザー情報を更新しました。"})
}

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

type userProjectItem struct {
	ProjectID   uint   `json:"project_id"`
	ProjectName string `json:"project_name"`
	Authority   int    `json:"authority"`
}

type getUserResponse struct {
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Projects []userProjectItem `json:"projects"`
}

func (h *UserHandler) GetUser(c *gin.Context) {
	userIDParam := c.Param("id")
	userID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不正なユーザーIDです。"})
		return
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "ユーザーが見つかりません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var members []model.ProjectMember
	if err := h.db.Where("user_id = ?", userID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	projects := []userProjectItem{}
	if len(members) > 0 {
		projectIDs := make([]uint, len(members))
		authorityMap := make(map[uint]int, len(members))
		for i, m := range members {
			projectIDs[i] = m.ProjectID
			authorityMap[m.ProjectID] = m.Authority
		}

		var projectList []model.Project
		if err := h.db.Where("project_id IN ?", projectIDs).Find(&projectList).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
			return
		}

		projects = make([]userProjectItem, len(projectList))
		for i, p := range projectList {
			projects[i] = userProjectItem{
				ProjectID:   p.ProjectID,
				ProjectName: p.ProjectName,
				Authority:   authorityMap[p.ProjectID],
			}
		}
	}

	c.JSON(http.StatusOK, getUserResponse{
		Name:     user.Name,
		Email:    user.Email,
		Projects: projects,
	})
}
