package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

type TaskHandler struct {
	db *gorm.DB
}

func NewTaskHandler(db *gorm.DB) *TaskHandler {
	return &TaskHandler{db: db}
}

type getTasksRequest struct {
	ProjectID uint `form:"project_id" binding:"required"`
}

type taskListItem struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Status    int       `json:"status"`
	CreatedBy string    `json:"created_by"`
	UserName  string    `json:"user_name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *TaskHandler) GetTasks(c *gin.Context) {
	var req getTasksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var tasks []model.Task
	if err := h.db.Where("project_id = ?", req.ProjectID).Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	if len(tasks) == 0 {
		c.JSON(http.StatusOK, []taskListItem{})
		return
	}

	userIDs := make(map[uint]struct{})
	for _, t := range tasks {
		userIDs[t.CreatedBy] = struct{}{}
		if t.UserID != 0 {
			userIDs[t.UserID] = struct{}{}
		}
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	var users []model.User
	if err := h.db.Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	userMap := make(map[uint]string, len(users))
	for _, u := range users {
		userMap[u.UserID] = u.Name
	}

	result := make([]taskListItem, len(tasks))
	for i, t := range tasks {
		result[i] = taskListItem{
			TaskID:    t.TaskID,
			Title:     t.Title,
			Status:    t.Status,
			CreatedBy: userMap[t.CreatedBy],
			UserName:  userMap[t.UserID],
			CreatedAt: t.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, result)
}

type commentItem struct {
	CommentID   uint      `json:"comment_id"`
	Content     string    `json:"content"`
	CreatedBy   string    `json:"created_by"`
	CreatedByID uint      `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type getTaskResponse struct {
	TaskID      uint          `json:"task_id"`
	Title       string        `json:"title"`
	Status      int           `json:"status"`
	Content     string        `json:"content"`
	Comments    []commentItem `json:"comments"`
	CreatedBy   string        `json:"created_by"`
	CreatedByID uint          `json:"created_by_id"`
	UserName    string        `json:"user_name"`
	CreatedAt   time.Time     `json:"created_at"`
}

func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	var task model.Task
	if err := h.db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var comments []model.Comment
	if err := h.db.Where("task_id = ?", task.TaskID).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	userIDs := map[uint]struct{}{
		task.CreatedBy: {},
	}
	if task.UserID != 0 {
		userIDs[task.UserID] = struct{}{}
	}
	for _, cm := range comments {
		userIDs[cm.CreatedBy] = struct{}{}
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	var users []model.User
	if err := h.db.Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}
	userMap := make(map[uint]string, len(users))
	for _, u := range users {
		userMap[u.UserID] = u.Name
	}

	commentItems := make([]commentItem, len(comments))
	for i, cm := range comments {
		commentItems[i] = commentItem{
			CommentID:   cm.CommentID,
			Content:     cm.Content,
			CreatedBy:   userMap[cm.CreatedBy],
			CreatedByID: cm.CreatedBy,
			CreatedAt:   cm.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, getTaskResponse{
		TaskID:      task.TaskID,
		Title:       task.Title,
		Status:      task.Status,
		Content:     task.Content,
		Comments:    commentItems,
		CreatedBy:   userMap[task.CreatedBy],
		CreatedByID: task.CreatedBy,
		UserName:    userMap[task.UserID],
		CreatedAt:   task.CreatedAt,
	})
}

type updateTaskRequest struct {
	UserID         uint   `json:"user_id" binding:"required"`
	TaskID         uint   `json:"task_id" binding:"required"`
	Title          string `json:"title" binding:"required"`
	Content        string `json:"content"`
	Status         int    `json:"status" binding:"required"`
	AssigneeUserID uint   `json:"assignee_user_id"`
}

type updateTaskResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    int       `json:"status"`
	CreatedBy string    `json:"created_by"`
	UserName  string    `json:"user_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *TaskHandler) UpdateTask(c *gin.Context) {
	var req updateTaskRequest
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

	task.Title = req.Title
	task.Content = req.Content
	task.Status = req.Status
	task.UserID = req.AssigneeUserID
	task.UpdatedAt = time.Now()

	if err := h.db.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	userIDs := map[uint]struct{}{
		task.CreatedBy: {},
	}
	if task.UserID != 0 {
		userIDs[task.UserID] = struct{}{}
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	var users []model.User
	if err := h.db.Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}
	userMap := make(map[uint]string, len(users))
	for _, u := range users {
		userMap[u.UserID] = u.Name
	}

	c.JSON(http.StatusOK, updateTaskResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		Content:   task.Content,
		Status:    task.Status,
		CreatedBy: userMap[task.CreatedBy],
		UserName:  userMap[task.UserID],
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
	})
}

type deleteTaskRequest struct {
	UserID uint `json:"user_id" binding:"required"`
	TaskID uint `json:"task_id" binding:"required"`
}

func (h *TaskHandler) DeleteTask(c *gin.Context) {
	var req deleteTaskRequest
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

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", req.TaskID).Delete(&model.Comment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&task).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task_id": req.TaskID})
}

type createTaskRequest struct {
	CreatedBy uint   `json:"created_by" binding:"required"`
	ProjectID uint   `json:"project_id" binding:"required"`
	Title     string `json:"title" binding:"required"`
	Content   string `json:"content"`
	UserName  uint   `json:"user_name"`
}

type createTaskResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"`
	UserName  string    `json:"user_name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var creator model.User
	if err := h.db.First(&creator, req.CreatedBy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	var assignee model.User
	assigneeName := ""
	if req.UserName != 0 {
		if err := h.db.First(&assignee, req.UserName).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
			return
		}
		assigneeName = assignee.Name
	}

	task := model.Task{
		ProjectID: req.ProjectID,
		Title:     req.Title,
		Content:   req.Content,
		UserID:    req.UserName,
		CreatedBy: req.CreatedBy,
		Status:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, createTaskResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		Content:   task.Content,
		CreatedBy: creator.Name,
		UserName:  assigneeName,
		CreatedAt: task.CreatedAt,
	})
}
