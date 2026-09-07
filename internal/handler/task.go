package handler

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
	"task-management/backend/internal/storage"
)

type TaskHandler struct {
	db      *gorm.DB
	storage *storage.Client
}

func NewTaskHandler(db *gorm.DB, storage *storage.Client) *TaskHandler {
	return &TaskHandler{db: db, storage: storage}
}

type getTasksRequest struct {
	ProjectID uint  `form:"project_id" binding:"required"`
	Status    *int  `form:"status"`
	UserID    *uint `form:"user_id"`
	CreatedBy *uint `form:"created_by"`
}

type taskListItem struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Status    int       `json:"status"`
	Priority  *int      `json:"priority"`
	CreatedBy string    `json:"created_by"`
	UserName  string    `json:"user_name"`
	CreatedAt time.Time `json:"created_at"`
	Images    []string  `json:"images"`
}

func (h *TaskHandler) GetTasks(c *gin.Context) {
	var req getTasksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	query := h.db.Where("project_id = ?", req.ProjectID)
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	}
	if req.CreatedBy != nil {
		query = query.Where("created_by = ?", *req.CreatedBy)
	}

	var tasks []model.Task
	if err := query.Order("task_id ASC").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	if len(tasks) == 0 {
		c.JSON(http.StatusOK, []taskListItem{})
		return
	}

	taskIDs := make([]uint, len(tasks))
	userIDs := make(map[uint]struct{})
	for i, t := range tasks {
		taskIDs[i] = t.TaskID
		userIDs[t.CreatedBy] = struct{}{}
		if t.UserID != nil {
			userIDs[*t.UserID] = struct{}{}
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

	var taskImages []model.TaskImage
	if err := h.db.Where("task_id IN ?", taskIDs).Find(&taskImages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}
	imageMap := make(map[uint][]string)
	for _, img := range taskImages {
		imageMap[img.TaskID] = append(imageMap[img.TaskID], img.ImageURL)
	}

	result := make([]taskListItem, len(tasks))
	for i, t := range tasks {
		assigneeName := ""
		if t.UserID != nil {
			assigneeName = userMap[*t.UserID]
		}
		imgs := imageMap[t.TaskID]
		if imgs == nil {
			imgs = []string{}
		}
		result[i] = taskListItem{
			TaskID:    t.TaskID,
			Title:     t.Title,
			Status:    t.Status,
			Priority:  t.Priority,
			CreatedBy: userMap[t.CreatedBy],
			UserName:  assigneeName,
			CreatedAt: t.CreatedAt,
			Images:    imgs,
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
	Images      []string  `json:"images"`
}

type getTaskResponse struct {
	TaskID      uint          `json:"task_id"`
	Title       string        `json:"title"`
	Status      int           `json:"status"`
	Priority    *int          `json:"priority"`
	Content     string        `json:"content"`
	Comments    []commentItem `json:"comments"`
	CreatedBy   string        `json:"created_by"`
	CreatedByID uint          `json:"created_by_id"`
	UserName    string        `json:"user_name"`
	CreatedAt   time.Time     `json:"created_at"`
	Images      []string      `json:"images"`
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

	// タスク画像を取得する
	var taskImages []model.TaskImage
	if err := h.db.Where("task_id = ?", task.TaskID).Find(&taskImages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}
	taskImageURLs := make([]string, len(taskImages))
	for i, img := range taskImages {
		taskImageURLs[i] = img.ImageURL
	}

	// コメント画像をコメントIDをキーに一括取得する
	commentIDs := make([]uint, len(comments))
	for i, cm := range comments {
		commentIDs[i] = cm.CommentID
	}
	var commentImages []model.CommentImage
	if len(commentIDs) > 0 {
		if err := h.db.Where("comment_id IN ?", commentIDs).Find(&commentImages).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
			return
		}
	}
	commentImageMap := make(map[uint][]string)
	for _, img := range commentImages {
		commentImageMap[img.CommentID] = append(commentImageMap[img.CommentID], img.ImageURL)
	}

	userIDs := map[uint]struct{}{
		task.CreatedBy: {},
	}
	if task.UserID != nil {
		userIDs[*task.UserID] = struct{}{}
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
		imgs := commentImageMap[cm.CommentID]
		if imgs == nil {
			imgs = []string{}
		}
		commentItems[i] = commentItem{
			CommentID:   cm.CommentID,
			Content:     cm.Content,
			CreatedBy:   userMap[cm.CreatedBy],
			CreatedByID: cm.CreatedBy,
			CreatedAt:   cm.CreatedAt,
			Images:      imgs,
		}
	}

	assigneeName := ""
	if task.UserID != nil {
		assigneeName = userMap[*task.UserID]
	}

	c.JSON(http.StatusOK, getTaskResponse{
		TaskID:      task.TaskID,
		Title:       task.Title,
		Status:      task.Status,
		Priority:    task.Priority,
		Content:     task.Content,
		Comments:    commentItems,
		CreatedBy:   userMap[task.CreatedBy],
		CreatedByID: task.CreatedBy,
		UserName:    assigneeName,
		CreatedAt:   task.CreatedAt,
		Images:      taskImageURLs,
	})
}

type updateTaskRequest struct {
	UserID         uint   `json:"user_id" binding:"required"`
	TaskID         uint   `json:"task_id" binding:"required"`
	Title          string `json:"title" binding:"required"`
	Content        string `json:"content"`
	Status         int    `json:"status" binding:"required"`
	Priority       string `json:"priority"`
	AssigneeUserID uint   `json:"assignee_user_id"`
}

type updateTaskResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    int       `json:"status"`
	Priority  *int      `json:"priority"`
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

	priorityMap := map[string]*int{
		"urgent": intPtr(1),
		"high":   intPtr(2),
		"medium": intPtr(3),
		"low":    intPtr(4),
		"":       nil,
	}
	priority, ok := priorityMap[req.Priority]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "priorityの値が不正です。"})
		return
	}

	task.Title = req.Title
	task.Content = req.Content
	task.Status = req.Status
	task.Priority = priority
	task.UpdatedAt = time.Now()
	if req.AssigneeUserID != 0 {
		task.UserID = &req.AssigneeUserID
	} else {
		task.UserID = nil
	}

	if err := h.db.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	userIDs := map[uint]struct{}{
		task.CreatedBy: {},
	}
	if task.UserID != nil {
		userIDs[*task.UserID] = struct{}{}
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

	assigneeName := ""
	if task.UserID != nil {
		assigneeName = userMap[*task.UserID]
	}

	c.JSON(http.StatusOK, updateTaskResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		Content:   task.Content,
		Status:    task.Status,
		Priority:  task.Priority,
		CreatedBy: userMap[task.CreatedBy],
		UserName:  assigneeName,
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

	// タスク・コメントに紐づく画像をCloudinaryから削除する
	h.deleteTaskImagesFromStorage(req.TaskID)

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		commentIDs := []uint{}
		var comments []model.Comment
		if err := tx.Where("task_id = ?", req.TaskID).Find(&comments).Error; err != nil {
			return err
		}
		for _, cm := range comments {
			commentIDs = append(commentIDs, cm.CommentID)
		}
		if len(commentIDs) > 0 {
			if err := tx.Where("comment_id IN ?", commentIDs).Delete(&model.CommentImage{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("task_id = ?", req.TaskID).Delete(&model.Comment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", req.TaskID).Delete(&model.TaskImage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&task).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task_id": req.TaskID})
}

// createTaskRequest はmultipart/form-dataで受け取る（画像ファイルを含むため）。
type createTaskRequest struct {
	CreatedBy uint   `form:"created_by" binding:"required"`
	ProjectID uint   `form:"project_id" binding:"required"`
	Title     string `form:"title" binding:"required"`
	Content   string `form:"content"`
	Priority  string `form:"priority"`
	UserName  uint   `form:"user_name"`
}

type createTaskResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Priority  *int      `json:"priority"`
	CreatedBy string    `json:"created_by"`
	UserName  string    `json:"user_name"`
	CreatedAt time.Time `json:"created_at"`
	Images    []string  `json:"images"`
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBind(&req); err != nil {
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

	priorityMap := map[string]*int{
		"urgent": intPtr(1),
		"high":   intPtr(2),
		"medium": intPtr(3),
		"low":    intPtr(4),
		"":       nil,
	}
	priority, ok := priorityMap[req.Priority]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "priorityの値が不正です。"})
		return
	}

	var assigneeID *uint
	assigneeName := ""
	if req.UserName != 0 {
		var assignee model.User
		if err := h.db.First(&assignee, req.UserName).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
			return
		}
		assigneeID = &req.UserName
		assigneeName = assignee.Name
	}

	task := model.Task{
		ProjectID: req.ProjectID,
		Title:     req.Title,
		Content:   req.Content,
		Priority:  priority,
		UserID:    assigneeID,
		CreatedBy: req.CreatedBy,
		Status:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// 画像をアップロードしてDBに保存する
	imageURLs := h.uploadImages(c, "images", func(url, publicID string) error {
		return h.db.Create(&model.TaskImage{
			TaskID:    task.TaskID,
			ImageURL:  url,
			PublicID:  publicID,
			CreatedAt: time.Now(),
		}).Error
	})

	c.JSON(http.StatusOK, createTaskResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		Content:   task.Content,
		Priority:  task.Priority,
		CreatedBy: creator.Name,
		UserName:  assigneeName,
		CreatedAt: task.CreatedAt,
		Images:    imageURLs,
	})
}

// uploadImages はリクエストの画像ファイルをCloudinaryにアップロードし、URLの一覧を返す。
// ストレージが未設定の場合やファイルが添付されていない場合は空スライスを返す。
func (h *TaskHandler) uploadImages(c *gin.Context, field string, save func(url, publicID string) error) []string {
	urls := []string{}
	if h.storage == nil {
		return urls
	}
	form, err := c.MultipartForm()
	if err != nil || form == nil {
		return urls
	}
	files := form.File[field]
	log.Printf("[uploadImages] field=%s, files=%d", field, len(files))
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			log.Printf("[uploadImages] open error: %v", err)
			continue
		}
		result, err := h.storage.Upload(f, "task-management")
		f.Close()
		if err != nil {
			log.Printf("[uploadImages] cloudinary upload error: %v", err)
			continue
		}
		log.Printf("[uploadImages] upload result: URL=%q, PublicID=%q", result.URL, result.PublicID)
		if result.URL == "" {
			continue
		}
		if err := save(result.URL, result.PublicID); err != nil {
			log.Printf("[uploadImages] db save error: %v", err)
			continue
		}
		urls = append(urls, result.URL)
	}
	return urls
}

// deleteTaskImagesFromStorage はタスクとそのコメントに紐づく画像をCloudinaryから削除する。
// DBからの削除はトランザクション内で行うため、ここではストレージのみ削除する。
func (h *TaskHandler) deleteTaskImagesFromStorage(taskID uint) {
	if h.storage == nil {
		return
	}
	var taskImages []model.TaskImage
	h.db.Where("task_id = ?", taskID).Find(&taskImages)
	for _, img := range taskImages {
		h.storage.Delete(img.PublicID)
	}

	var comments []model.Comment
	h.db.Where("task_id = ?", taskID).Find(&comments)
	for _, cm := range comments {
		var commentImages []model.CommentImage
		h.db.Where("comment_id = ?", cm.CommentID).Find(&commentImages)
		for _, img := range commentImages {
			h.storage.Delete(img.PublicID)
		}
	}
}

func intPtr(v int) *int { return &v }
