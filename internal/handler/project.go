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

// ProjectHandler はプロジェクト関連のHTTPハンドラーをまとめた構造体。
// プロジェクトの一覧取得・作成・更新・削除・メンバー一覧・権限確認の6つの処理を提供する。
type ProjectHandler struct {
	db *gorm.DB
}

func NewProjectHandler(db *gorm.DB) *ProjectHandler {
	return &ProjectHandler{db: db}
}

// projectListItem はプロジェクト一覧の各要素。
// プロジェクトの基本情報に加え、タスクの完了率・未完了率（小数点第1位まで）を含む。
type projectListItem struct {
	ProjectID         uint    `json:"project_id"`
	ProjectName       string  `json:"project_name"`
	TaskPerComplete   float64 `json:"task_per_complete"`   // タスク完了率（例: 66.7）
	TaskPerIncomplete float64 `json:"task_per_incomplete"` // タスク未完了率（例: 33.3）
}

// taskStats はプロジェクトごとのタスク集計結果を受け取るための構造体。
// GORM の Scan で SQL の集計結果をマッピングするために使用する。
type taskStats struct {
	ProjectID      uint
	TotalCount     int64
	CompletedCount int64
}

// GetProjects はユーザーが所属するプロジェクト一覧を、各プロジェクトのタスク完了率とともに返す。
// ユーザーがどのプロジェクトにも所属していない場合は空配列を返す。
//
// 処理フロー:
//  1. パスパラメータ :id からユーザーIDを取得・変換
//  2. ユーザーの存在確認
//  3. ユーザーのプロジェクトメンバーシップ一覧を取得
//  4. メンバーシップからプロジェクトIDの配列を構築してプロジェクト情報を一括取得
//  5. プロジェクトごとのタスク数・完了タスク数をSQLで集計
//  6. 完了率・未完了率を計算してレスポンスを組み立てる
func (h *ProjectHandler) GetProjects(c *gin.Context) {
	// ① パスパラメータ :id を文字列から uint64 に変換する。
	userIDParam := c.Param("id")
	userID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
		return
	}

	// ② ユーザーIDでレコードを取得する。存在しない場合はトークン切れとして 401 を返す。
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ ユーザーが所属するプロジェクトのメンバーシップレコードをすべて取得する。
	var members []model.ProjectMember
	if err := h.db.Where("user_id = ?", userID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// メンバーシップが0件 = どのプロジェクトにも所属していない → 空配列を返す。
	if len(members) == 0 {
		c.JSON(http.StatusOK, []projectListItem{})
		return
	}

	// ④ メンバーシップからプロジェクトIDの配列を構築する。
	//    後続のIN句クエリとタスク集計クエリで使用する。
	projectIDs := make([]uint, len(members))
	for i, m := range members {
		projectIDs[i] = m.ProjectID
	}

	// プロジェクトIDの配列を使って、プロジェクト情報を1回のSQLで一括取得する（N+1問題を回避）。
	var projects []model.Project
	if err := h.db.Where("project_id IN ?", projectIDs).Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ⑤ プロジェクトごとのタスク総数と完了タスク数（status=3）をSQLのCASE式でまとめて集計する。
	//    個別に2回クエリするより1回にまとめた方がパフォーマンスが良い。
	var stats []taskStats
	if err := h.db.Model(&model.Task{}).
		Select("project_id, COUNT(*) as total_count, SUM(CASE WHEN status = 3 THEN 1 ELSE 0 END) as completed_count").
		Where("project_id IN ?", projectIDs).
		Group("project_id").
		Scan(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// 集計結果をプロジェクトIDをキーとするマップに変換する（O(1)でのアクセスのため）。
	statsMap := make(map[uint]taskStats, len(stats))
	for _, s := range stats {
		statsMap[s.ProjectID] = s
	}

	// ⑥ プロジェクトごとに完了率・未完了率を計算してレスポンスを組み立てる。
	//    タスクが0件の場合は両方 0.0 を返す（ゼロ除算を回避するため total_count > 0 を確認）。
	//    math.Round を使って小数点第1位まで丸める（例: 0.6666... → 66.7）。
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

// createProjectRequest はプロジェクト作成APIのリクエストボディ。
type createProjectRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	ProjectName string `json:"project_name" binding:"required"`
}

// projectItem はプロジェクト作成APIのレスポンスで返すプロジェクト一覧の各要素。
type projectItem struct {
	ProjectName string `json:"project_name"`
	ProjectID   uint   `json:"project_id"`
}

// CreateProject は新規プロジェクトを作成し、作成者を自動的にマネージャーとしてメンバーに追加する。
// レスポンスとして、作成者が所属する全プロジェクトの一覧を返す（フロントエンドの状態更新を一度に行うため）。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. 作成者ユーザーの存在確認
//  3. プロジェクトをDBに登録
//  4. 作成者をマネージャー権限（authority=3）でメンバーに追加
//  5. 作成者が所属する全プロジェクト一覧を取得してレスポンスとして返す
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	// ① リクエストボディをバインドしてバリデーションを実行する。
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② 作成者ユーザーがDBに存在するか確認する。
	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ プロジェクトをDBに登録する。
	project := model.Project{
		ProjectName: req.ProjectName,
		CreatedBy:   req.UserID,
		CreatedAt:   time.Now(),
	}
	if err := h.db.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ 作成者をマネージャー権限（authority=3）でプロジェクトメンバーに追加する。
	//    authority の値: 1=一般メンバー, 2=未使用, 3=マネージャー
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

	// ⑤ 作成者が所属する全プロジェクトIDを取得してプロジェクト情報を一括取得する。
	//    新規作成したプロジェクトを含む最新の一覧をレスポンスとして返すことで、
	//    フロントエンドが追加のAPI呼び出しなしに状態を更新できるようにする。
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

// memberItem はプロジェクトメンバー一覧の各要素。
type memberItem struct {
	UserID uint   `json:"user_id"`
	Name   string `json:"name"`
}

// GetProjectMembers はプロジェクトに所属するメンバーのユーザーID・名前の一覧を返す。
// タスク担当者の選択肢やメンバー管理画面での表示に使用されることを想定している。
//
// 処理フロー:
//  1. パスパラメータ :id からプロジェクトIDを取得・変換
//  2. プロジェクトメンバーシップ一覧を取得
//  3. メンバーシップからユーザーIDを収集してユーザー情報を一括取得
//  4. ユーザーID・名前を組み合わせてレスポンスを返す
func (h *ProjectHandler) GetProjectMembers(c *gin.Context) {
	// ① パスパラメータ :id を文字列から uint64 に変換する。
	projectIDParam := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② 対象プロジェクトのメンバーシップレコードをすべて取得する。
	var members []model.ProjectMember
	if err := h.db.Where("project_id = ?", projectID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// メンバーが0件の場合は空配列を返す。
	if len(members) == 0 {
		c.JSON(http.StatusOK, []memberItem{})
		return
	}

	// ③ メンバーシップからユーザーIDの配列を構築し、ユーザー情報を1回のSQLで一括取得する（N+1問題を回避）。
	userIDs := make([]uint, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}

	var users []model.User
	if err := h.db.Where("user_id IN ?", userIDs).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ ユーザーID・名前を含むレスポンス配列を構築する。
	result := make([]memberItem, len(users))
	for i, u := range users {
		result[i] = memberItem{
			UserID: u.UserID,
			Name:   u.Name,
		}
	}

	c.JSON(http.StatusOK, result)
}

// getAuthorityRequest は権限取得APIのクエリパラメータ。
type getAuthorityRequest struct {
	UserID uint `form:"user_id" binding:"required"`
}

// GetAuthority は指定ユーザーの指定プロジェクト内における権限（authority）を返す。
// 対象プロジェクトにメンバーとして参加していない場合は一般権限（1）を返す。
//
// authority の値の意味:
//   - 1: 一般メンバー（未参加の場合もこの値を返す）
//   - 3: マネージャー（プロジェクトの管理権限を持つ）
//
// 処理フロー:
//  1. パスパラメータ :id からプロジェクトIDを取得・変換
//  2. クエリパラメータからユーザーIDを取得
//  3. プロジェクトメンバーシップから権限を検索
//  4. メンバーでない場合は 1、メンバーの場合は登録されている権限を返す
func (h *ProjectHandler) GetAuthority(c *gin.Context) {
	// ① パスパラメータ :id を文字列から uint64 に変換する。
	projectIDParam := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② クエリパラメータから対象ユーザーIDを取得する。
	var req getAuthorityRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ プロジェクトメンバーシップから権限を取得する。
	//    複数レコードが存在する場合を考慮し、authority の降順で最初の1件を取得する（最高権限を使用）。
	var member model.ProjectMember
	err = h.db.Where("project_id = ? AND user_id = ?", projectID, req.UserID).Order("authority DESC").First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// プロジェクトのメンバーでない場合は一般権限（1）を返す。
			// フロントエンドはこの値を元に表示や操作の制限を判定する。
			c.JSON(http.StatusOK, gin.H{"authority": 1})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ authority が 0 の場合も一般権限（1）として扱う。
	//    0 はデータ不整合を防ぐためのフォールバック値。
	authority := member.Authority
	if authority == 0 {
		authority = 1
	}

	c.JSON(http.StatusOK, gin.H{"authority": authority})
}

// updateProjectRequest はプロジェクト更新APIのリクエストボディ。
// Manager・RenameProject・Members はすべて省略可能で、指定された項目のみ更新する。
type updateProjectRequest struct {
	ProjectID     uint     `json:"project_id" binding:"required"`
	Manager       uint     `json:"manager"`         // 新マネージャーのユーザーID。0 の場合は変更しない
	RenameProject string   `json:"rename_project"`  // 新プロジェクト名。空文字の場合は変更しない
	Members       []string `json:"members"`         // 追加するメンバーのメールアドレスの配列
}

// UpdateProject はプロジェクトのマネージャー変更・プロジェクト名変更・メンバー追加を行う。
// 各操作は入力がある場合のみ実行され、すべてトランザクション内で処理される。
//
// マネージャー変更の仕様:
//   - 既存のマネージャー（authority=3）を一般権限（1）に降格する
//   - 新マネージャーの既存レコードを一旦削除してから authority=3 で再挿入する
//     （同一ユーザーが複数のメンバーシップレコードを持つ可能性があるため）
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. トランザクション内でマネージャー変更・プロジェクト名変更・メンバー追加を順に実行
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	// ① リクエストボディをバインドしてバリデーションを実行する。
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② マネージャー変更・プロジェクト名変更・メンバー追加をすべてトランザクションで実行する。
	//    いずれかの処理が失敗した場合は全体がロールバックされ、部分的な更新が発生しない。
	err := h.db.Transaction(func(tx *gorm.DB) error {
		// マネージャー更新（Manager が 0 以外の場合のみ実行）
		if req.Manager != 0 {
			// 現在のマネージャー（authority=3）全員を一般権限（1）に降格する。
			if err := tx.Model(&model.ProjectMember{}).
				Where("project_id = ? AND authority = 3", req.ProjectID).
				Update("authority", 1).Error; err != nil {
				return err
			}

			// 新マネージャーの既存メンバーシップレコードを削除してから authority=3 で再挿入する。
			// DELETE + INSERT にしているのは、既存レコードの authority を更新するだけでは
			// 複数レコードが存在する場合に不整合が生じる可能性があるため。
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

		// プロジェクト名を更新（RenameProject が空文字以外の場合のみ実行）
		if req.RenameProject != "" {
			if err := tx.Model(&model.Project{}).
				Where("project_id = ?", req.ProjectID).
				Update("project_name", req.RenameProject).Error; err != nil {
				return err
			}
		}

		// メンバー追加（Members が空でない場合のみ実行）
		// メールアドレスの配列を受け取り、各メールアドレスに対応するユーザーを一般メンバー（authority=1）として追加する。
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

// DeleteProject はプロジェクトと、その配下にあるすべてのデータ（コメント・タスク・メンバーシップ）を削除する。
// 削除はトランザクション内で行い、参照整合性を保つために依存関係の深い順（コメント→タスク→メンバー→プロジェクト）に削除する。
//
// 処理フロー:
//  1. パスパラメータ :id からプロジェクトIDを取得・変換
//  2. プロジェクトの存在確認
//  3. トランザクション内でコメント→タスク→メンバーシップ→プロジェクトの順に削除
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	// ① パスパラメータ :id を文字列から uint64 に変換する。
	projectIDParam := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② プロジェクトがDBに存在するか確認する。
	var project model.Project
	if err := h.db.First(&project, projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "トークン切れです。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ コメント→タスク→メンバーシップ→プロジェクトの順にトランザクションで削除する。
	//    外部キー制約を考慮し、参照される側（親）より先に参照する側（子）を削除する。
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		// プロジェクト配下のタスクIDを収集する。
		// コメントはタスクIDを外部キーとして持つため、先にタスクIDを特定する必要がある。
		var taskIDs []uint
		if err := tx.Model(&model.Task{}).
			Where("project_id = ?", projectID).
			Pluck("task_id", &taskIDs).Error; err != nil {
			return err
		}

		if len(taskIDs) > 0 {
			// タスクに紐づくコメントをすべて削除する（タスク削除より先に行う）。
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&model.Comment{}).Error; err != nil {
				return err
			}
			// コメント削除後にタスクをすべて削除する。
			if err := tx.Where("project_id = ?", projectID).Delete(&model.Task{}).Error; err != nil {
				return err
			}
		}

		// プロジェクトのメンバーシップレコードをすべて削除する。
		if err := tx.Where("project_id = ?", projectID).Delete(&model.ProjectMember{}).Error; err != nil {
			return err
		}

		// 最後にプロジェクトレコード本体を削除する。
		return tx.Delete(&project).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
