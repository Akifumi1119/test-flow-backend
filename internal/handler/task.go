package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

// TaskHandler はタスク関連のHTTPハンドラーをまとめた構造体。
// タスクの一覧取得・詳細取得・作成・更新・削除の5つの処理を提供する。
type TaskHandler struct {
	db *gorm.DB
}

func NewTaskHandler(db *gorm.DB) *TaskHandler {
	return &TaskHandler{db: db}
}

// getTasksRequest はタスク一覧取得APIのクエリパラメータ。
// project_id は必須。status・user_id・created_by は省略可能で、指定した項目のみ AND 条件で絞り込む。
type getTasksRequest struct {
	ProjectID uint  `form:"project_id" binding:"required"`
	Status    *int  `form:"status"`   // 省略可能: 指定時はそのステータスのタスクのみ返す
	UserID    *uint `form:"user_id"`   // 省略可能: 指定時はその担当者IDのタスクのみ返す
	CreatedBy *uint `form:"created_by"` // 省略可能: 指定時はその作成者IDのタスクのみ返す
}

// taskListItem はタスク一覧の各要素。
// 一覧表示に必要な情報のみを含む（コメントなどの詳細は GetTask で取得する）。
type taskListItem struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Status    int       `json:"status"`
	Priority  *int      `json:"priority"` // タスク優先度
	CreatedBy string    `json:"created_by"` // 作成者の名前（IDではなく名前文字列）
	UserName  string    `json:"user_name"`  // 担当者の名前。未割り当ての場合は空文字
	CreatedAt time.Time `json:"created_at"`
}

// GetTasks はプロジェクトIDでタスク一覧を取得する。
// status・user_id・created_by のクエリパラメータが指定された場合は AND 条件で絞り込む。
// 結果が0件でも 200 で空配列を返す。
//
// 処理フロー:
//  1. クエリパラメータのバリデーション
//  2. 指定されたパラメータのみ AND 条件として絞り込みクエリを動的に構築
//  3. タスク一覧を取得
//  4. タスクに関連するユーザーID（作成者・担当者）を収集して一括取得
//  5. ユーザーIDと名前のマップを使ってレスポンスを組み立てる
func (h *TaskHandler) GetTasks(c *gin.Context) {
	// ① クエリパラメータを getTasksRequest 構造体にバインドしてバリデーションを実行する。
	//    project_id が未指定の場合は binding:"required" によりエラーになる。
	var req getTasksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② 絞り込みクエリを動的に構築する。
	//    project_id は常に WHERE 条件に追加する（必須パラメータ）。
	//    status・user_id・created_by はポインタ型のため、nil でない場合のみ条件に追加する。
	//    ポインタ型にしているのは「0」と「未指定」を区別するため
	//    （例: status=0 を「絞り込みなし」ではなく「ステータスが0」として扱う）。
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

	// ③ 構築したクエリでタスク一覧を取得する。task_id の昇順で返す。
	var tasks []model.Task
	if err := query.Order("task_id ASC").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// タスクが0件の場合は空配列を返す（null ではなく [] を返すことでフロントエンドの処理を統一する）。
	if len(tasks) == 0 {
		c.JSON(http.StatusOK, []taskListItem{})
		return
	}

	// ④ タスクに関連するユーザーID（作成者・担当者）を重複なく収集する。
	//    map をセットとして使うことで重複を自動的に排除できる。
	//    担当者（UserID）は nullable なので nil チェックが必要。
	userIDs := make(map[uint]struct{})
	for _, t := range tasks {
		userIDs[t.CreatedBy] = struct{}{}
		if t.UserID != nil {
			userIDs[*t.UserID] = struct{}{}
		}
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	// 収集したIDのユーザー情報を1回のSQLで一括取得する（タスクごとに個別取得するN+1問題を回避）。
	var users []model.User
	if err := h.db.Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ユーザーIDをキー、ユーザー名を値とするマップを構築する。
	// タスクとユーザーを O(1) で紐付けるために使用する。
	userMap := make(map[uint]string, len(users))
	for _, u := range users {
		userMap[u.UserID] = u.Name
	}

	// ⑤ 各タスクにユーザー名を解決してレスポンス用の配列を構築する。
	//    担当者未設定（UserID が nil）の場合は user_name を空文字にする。
	result := make([]taskListItem, len(tasks))
	for i, t := range tasks {
		assigneeName := ""
		if t.UserID != nil {
			assigneeName = userMap[*t.UserID]
		}
		result[i] = taskListItem{
			TaskID:    t.TaskID,
			Title:     t.Title,
			Status:    t.Status,
			Priority:  t.Priority,
			CreatedBy: userMap[t.CreatedBy],
			UserName:  assigneeName,
			CreatedAt: t.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, result)
}

// commentItem はタスク詳細レスポンスに含まれるコメントの各要素。
type commentItem struct {
	CommentID   uint      `json:"comment_id"`
	Content     string    `json:"content"`
	CreatedBy   string    `json:"created_by"`   // 投稿者の名前
	CreatedByID uint      `json:"created_by_id"` // 投稿者のユーザーID（編集・削除権限の判定に使用）
	CreatedAt   time.Time `json:"created_at"`
}

// getTaskResponse はタスク詳細取得APIのレスポンス。
// タスクの基本情報に加え、コメント一覧と作成者・担当者の名前を含む。
type getTaskResponse struct {
	TaskID      uint          `json:"task_id"`
	Title       string        `json:"title"`
	Status      int           `json:"status"`
	Priority    *int          `json:"priority"` // タスク優先度
	Content     string        `json:"content"`
	Comments    []commentItem `json:"comments"`
	CreatedBy   string        `json:"created_by"`   // 作成者の名前
	CreatedByID uint          `json:"created_by_id"` // 作成者のユーザーID
	UserName    string        `json:"user_name"`    // 担当者の名前。未割り当ての場合は空文字
	CreatedAt   time.Time     `json:"created_at"`
}

// GetTask はタスクIDに対応するタスクの詳細情報とコメント一覧を返す。
//
// 処理フロー:
//  1. パスパラメータ :task_id でタスクを取得
//  2. タスクに紐づくコメント一覧を取得
//  3. タスクとコメントに関連するユーザーIDを収集して一括取得
//  4. ユーザー名を解決してレスポンスを組み立てる
func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	// ① パスパラメータ :task_id でタスクを検索する。存在しない場合は 400 を返す。
	var task model.Task
	if err := h.db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② タスクに紐づくコメントをすべて取得する。
	var comments []model.Comment
	if err := h.db.Where("task_id = ?", task.TaskID).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ タスクとコメントに関連するユーザーID（タスク作成者・担当者・コメント投稿者）を
	//    重複なく収集する。map をセットとして使い重複を排除する。
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

	// 収集したIDのユーザー情報を1回のSQLで一括取得する（N+1問題を回避）。
	var users []model.User
	if err := h.db.Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}
	userMap := make(map[uint]string, len(users))
	for _, u := range users {
		userMap[u.UserID] = u.Name
	}

	// ④ コメントのユーザー名を解決してレスポンス用の配列を構築する。
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

	// 担当者未設定（UserID が nil）の場合は user_name を空文字にする。
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
	})
}

// updateTaskRequest はタスク更新APIのリクエストボディ。
// AssigneeUserID が 0 の場合は担当者なし（未割り当て）として扱う。
type updateTaskRequest struct {
	UserID         uint   `json:"user_id" binding:"required"`
	TaskID         uint   `json:"task_id" binding:"required"`
	Title          string `json:"title" binding:"required"`
	Content        string `json:"content"`
	Status         int    `json:"status" binding:"required"`
	Priority       string `json:"priority"` // "urgent"=1, "high"=2, "medium"=3, "low"=4, ""=null
	AssigneeUserID uint   `json:"assignee_user_id"` // 0 の場合は未割り当て
}

// updateTaskResponse はタスク更新APIのレスポンス。
type updateTaskResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    int       `json:"status"`
	Priority  *int      `json:"priority"` // タスク優先度
	CreatedBy string    `json:"created_by"` // 作成者の名前
	UserName  string    `json:"user_name"`  // 担当者の名前。未割り当ての場合は空文字
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdateTask はタスクのタイトル・内容・ステータス・担当者を更新する。
// AssigneeUserID が 0 の場合は担当者を未割り当てに変更する。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. 操作ユーザーの存在確認
//  3. 更新対象タスクの存在確認
//  4. タスク情報を更新してDBに保存
//  5. 作成者・担当者のユーザー名を解決してレスポンスを組み立てる
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	// ① リクエストボディを updateTaskRequest 構造体にバインドしてバリデーションを実行する。
	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② 操作ユーザーがDBに存在するか確認する。
	//    存在しない場合はトークン切れとして 401 を返す。
	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ 更新対象タスクがDBに存在するか確認する。
	var task model.Task
	if err := h.db.First(&task, req.TaskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ タスクの各フィールドをリクエストの値で更新する。
	//    AssigneeUserID が 0 の場合は担当者なし（user_id = null）として扱う。
	//    0 かどうかで判定しているのは、uint のゼロ値が「未指定」を意味するため。
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

	// ⑤ 更新後のタスクに関連するユーザーID（作成者・担当者）を収集して一括取得する（N+1問題を回避）。
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

	// 担当者未設定（UserID が nil）の場合は user_name を空文字にする。
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

// deleteTaskRequest はタスク削除APIのリクエストボディ。
type deleteTaskRequest struct {
	UserID uint `json:"user_id" binding:"required"`
	TaskID uint `json:"task_id" binding:"required"`
}

// DeleteTask はタスクとそれに紐づくコメントをすべて物理削除する。
// コメントを先に削除してからタスクを削除することで、参照整合性を保つ。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. 操作ユーザーの存在確認
//  3. 削除対象タスクの存在確認
//  4. トランザクション内でコメント→タスクの順に削除
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	// ① リクエストボディをバインドしてバリデーションを実行する。
	var req deleteTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② 操作ユーザーがDBに存在するか確認する。
	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ 削除対象タスクがDBに存在するか確認する。
	var task model.Task
	if err := h.db.First(&task, req.TaskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ コメント→タスクの順にトランザクションで削除する。
	//    コメントはタスクに対する外部キーを持つため、タスクより先に削除する必要がある。
	//    トランザクションにより、どちらかが失敗した場合は両方ロールバックされる。
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		// タスクに紐づくコメントをすべて削除する。
		if err := tx.Where("task_id = ?", req.TaskID).Delete(&model.Comment{}).Error; err != nil {
			return err
		}
		// コメント削除後にタスク本体を削除する。
		return tx.Delete(&task).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task_id": req.TaskID})
}

// createTaskRequest はタスク作成APIのリクエストボディ。
// UserName フィールドは担当者のユーザーIDを受け取る（フィールド名が実態と異なるため注意）。
// UserName が 0 の場合は担当者なし（未割り当て）として扱う。
type createTaskRequest struct {
	CreatedBy uint   `json:"created_by" binding:"required"`
	ProjectID uint   `json:"project_id" binding:"required"`
	Title     string `json:"title" binding:"required"`
	Content   string `json:"content"`
	Priority  string `json:"priority"` // "urgent"=1, "high"=2, "medium"=3, "low"=4, ""=null
	UserName  uint   `json:"user_name"` // 担当者のユーザーID。0 の場合は未割り当て
}

// createTaskResponse はタスク作成APIのレスポンス。
type createTaskResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Priority  *int      `json:"priority"` // タスク優先度
	CreatedBy string    `json:"created_by"` // 作成者の名前
	UserName  string    `json:"user_name"`  // 担当者の名前。未割り当ての場合は空文字
	CreatedAt time.Time `json:"created_at"`
}

// CreateTask は新規タスクを作成する。初期ステータスは「未着手（1）」に固定される。
// 担当者IDが指定された場合はユーザーの存在確認も行い、存在する場合のみ担当者として設定する。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. 作成者ユーザーの存在確認
//  3. 担当者が指定されている場合は存在確認を行い担当者情報を取得
//  4. タスクをDBに登録（初期ステータス: 1=未着手）
//  5. 作成者・担当者の名前を含めてレスポンスを返す
func (h *TaskHandler) CreateTask(c *gin.Context) {
	// ① リクエストボディを createTaskRequest 構造体にバインドしてバリデーションを実行する。
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② 作成者ユーザーがDBに存在するか確認する。
	//    存在しない場合はトークン切れとして 401 を返す。
	var creator model.User
	if err := h.db.First(&creator, req.CreatedBy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ②-b priority 文字列を数値に変換する。
	//    "urgent"=1, "high"=2, "medium"=3, "low"=4, ""=null。それ以外は 400 を返す。
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

	// ③ 担当者（UserName フィールドで受け取るユーザーID）が指定されている場合は存在確認を行う。
	//    UserName が 0 の場合は担当者なしとして扱い、この処理をスキップする。
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

	// ④ タスクをDBに登録する。
	//    Status は 1（未着手）で固定。CreatedAt・UpdatedAt は現在時刻をセットする。
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

	// ⑤ 作成者・担当者の名前を含めてレスポンスを返す。
	c.JSON(http.StatusOK, createTaskResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		Content:   task.Content,
		Priority:  task.Priority,
		CreatedBy: creator.Name,
		UserName:  assigneeName,
		CreatedAt: task.CreatedAt,
	})
}

func intPtr(v int) *int { return &v }
