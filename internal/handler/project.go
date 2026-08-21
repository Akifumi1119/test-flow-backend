package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

type ProjectHandler struct {
	db *gorm.DB
}

func NewProjectHandler(db *gorm.DB) *ProjectHandler {
	return &ProjectHandler{db: db}
}

type projectListItem struct {
	ProjectID         uint    `json:"project_id"`
	ProjectName       string  `json:"project_name"`
	TaskPerComplete   float64 `json:"task_per_complete"`
	TaskPerIncomplete float64 `json:"task_per_incomplete"`
}

type taskStats struct {
	ProjectID      uint
	TotalCount     int64
	CompletedCount int64
}

func (h *ProjectHandler) GetProjects(c *gin.Context) {
	userIDParam := c.Param("id")
	userID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
		return
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
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

	if len(members) == 0 {
		c.JSON(http.StatusOK, []projectListItem{})
		return
	}

	projectIDs := make([]uint, len(members))
	for i, m := range members {
		projectIDs[i] = m.ProjectID
	}

	var projects []model.Project
	if err := h.db.Where("project_id IN ?", projectIDs).Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var stats []taskStats
	if err := h.db.Model(&model.Task{}).
		Select("project_id, COUNT(*) as total_count, SUM(CASE WHEN status = 3 THEN 1 ELSE 0 END) as completed_count").
		Where("project_id IN ?", projectIDs).
		Group("project_id").
		Scan(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	statsMap := make(map[uint]taskStats, len(stats))
	for _, s := range stats {
		statsMap[s.ProjectID] = s
	}

	result := make([]projectListItem, len(projects))
	for i, p := range projects {
		var complete, incomplete float64
		if s, ok := statsMap[p.ProjectID]; ok && s.TotalCount > 0 {
			complete = math.Round(float64(s.CompletedCount)/float64(s.TotalCount)*1000) / 10
			incomplete = math.Round((1-float64(s.CompletedCount)/float64(s.TotalCount))*1000) / 10
		}
		result[i] = projectListItem{
			ProjectID:         p.ProjectID,
			ProjectName:       p.ProjectName,
			TaskPerComplete:   complete,
			TaskPerIncomplete: incomplete,
		}
	}

	c.JSON(http.StatusOK, result)
}

type createProjectRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	ProjectName string `json:"project_name" binding:"required"`
}

type projectItem struct {
	ProjectName string `json:"project_name"`
	ProjectID   uint   `json:"project_id"`
}

func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req createProjectRequest
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

	project := model.Project{
		ProjectName: req.ProjectName,
		CreatedBy:   req.UserID,
		CreatedAt:   time.Now(),
	}
	if err := h.db.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	member := model.ProjectMember{
		ProjectID: project.ProjectID,
		UserID:    req.UserID,
		Authority: 3,
		EntryDate: time.Now(),
	}
	if err := h.db.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var members []model.ProjectMember
	if err := h.db.Where("user_id = ?", req.UserID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	projectIDs := make([]uint, len(members))
	for i, m := range members {
		projectIDs[i] = m.ProjectID
	}

	var projects []model.Project
	if err := h.db.Where("project_id IN ?", projectIDs).Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	items := make([]projectItem, len(projects))
	for i, p := range projects {
		items[i] = projectItem{
			ProjectName: p.ProjectName,
			ProjectID:   p.ProjectID,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"projects": items,
	})
}

type memberItem struct {
	UserID uint   `json:"user_id"`
	Name   string `json:"name"`
}

func (h *ProjectHandler) GetProjectMembers(c *gin.Context) {
	projectIDParam := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var members []model.ProjectMember
	if err := h.db.Where("project_id = ?", projectID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	if len(members) == 0 {
		c.JSON(http.StatusOK, []memberItem{})
		return
	}

	userIDs := make([]uint, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}

	var users []model.User
	if err := h.db.Where("user_id IN ?", userIDs).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	result := make([]memberItem, len(users))
	for i, u := range users {
		result[i] = memberItem{
			UserID: u.UserID,
			Name:   u.Name,
		}
	}

	c.JSON(http.StatusOK, result)
}

type getAuthorityRequest struct {
	UserID uint `form:"user_id" binding:"required"`
}

func (h *ProjectHandler) GetAuthority(c *gin.Context) {
	projectIDParam := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var req getAuthorityRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var member model.ProjectMember
	err = h.db.Where("project_id = ? AND user_id = ?", projectID, req.UserID).Order("authority DESC").First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"authority": 1})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	authority := member.Authority
	if authority == 0 {
		authority = 1
	}

	c.JSON(http.StatusOK, gin.H{"authority": authority})
}

type updateProjectRequest struct {
	ProjectID     uint     `json:"project_id" binding:"required"`
	Manager       uint     `json:"manager"`
	RenameProject string   `json:"rename_project"`
	Members       []string `json:"members"`
}

func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		// マネージャー更新（入力がある場合のみ）
		if req.Manager != 0 {
			// 既存管理者（authority=3）を降格
			if err := tx.Model(&model.ProjectMember{}).
				Where("project_id = ? AND authority = 3", req.ProjectID).
				Update("authority", 1).Error; err != nil {
				return err
			}

			// 新マネージャーの既存レコードを全件削除してから INSERT
			if err := tx.Where("project_id = ? AND user_id = ?", req.ProjectID, req.Manager).
				Delete(&model.ProjectMember{}).Error; err != nil {
				return err
			}
			newMember := model.ProjectMember{
				ProjectID: req.ProjectID,
				UserID:    req.Manager,
				Authority: 3,
				EntryDate: time.Now(),
			}
			if err := tx.Create(&newMember).Error; err != nil {
				return err
			}
		}

		// プロジェクト名を更新（入力がある場合のみ）
		if req.RenameProject != "" {
			if err := tx.Model(&model.Project{}).
				Where("project_id = ?", req.ProjectID).
				Update("project_name", req.RenameProject).Error; err != nil {
				return err
			}
		}

		// メンバー追加（入力がある場合のみ）
		for _, email := range req.Members {
			var user model.User
			if err := tx.Where("email = ?", email).First(&user).Error; err != nil {
				return err
			}
			member := model.ProjectMember{
				ProjectID: req.ProjectID,
				UserID:    user.UserID,
				Authority: 1,
				EntryDate: time.Now(),
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	projectIDParam := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var project model.Project
	if err := h.db.First(&project, projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		var taskIDs []uint
		if err := tx.Model(&model.Task{}).
			Where("project_id = ?", projectID).
			Pluck("task_id", &taskIDs).Error; err != nil {
			return err
		}

		if len(taskIDs) > 0 {
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&model.Comment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", projectID).Delete(&model.Task{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("project_id = ?", projectID).Delete(&model.ProjectMember{}).Error; err != nil {
			return err
		}

		return tx.Delete(&project).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
