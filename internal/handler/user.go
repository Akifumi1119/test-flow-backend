package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	internaldb "task-management/backend/internal/db"
	"task-management/backend/internal/model"
)

// updateUserProjectItem はユーザー更新リクエストの projects 配列の各要素。
// 脱退させたいプロジェクトの ID と名前を受け取る。
type updateUserProjectItem struct {
	ProjectID   uint   `json:"project_id"`
	ProjectName string `json:"project_name"`
}

// updateUserRequest はユーザー更新APIのリクエストボディ。
// name・email・projects はすべて省略可能で、指定された項目のみ更新する。
// email は省略時でも形式バリデーションは行う（omitempty により空文字はスキップ）。
type updateUserRequest struct {
	Name     string                  `json:"name"`
	Email    string                  `json:"email" binding:"omitempty,email"`
	Projects []updateUserProjectItem `json:"projects"`
}

// UpdateUser はユーザーの名前・メールアドレスを更新し、指定されたプロジェクトから脱退させる。
// 更新は指定された項目のみ行い、未指定の項目は変更しない。
// プロジェクト脱退は指定されたプロジェクトのみ対象とし、未指定のプロジェクトには影響しない。
//
// 処理フロー:
//  1. パスパラメータ :id からユーザーIDを取得・変換
//  2. リクエストボディのバリデーション
//  3. 更新対象ユーザーの存在確認
//  4. メールアドレスを変更する場合は他ユーザーとの重複チェック
//  5. トランザクション内でユーザー情報更新・プロジェクト脱退を実行
func (h *UserHandler) UpdateUser(c *gin.Context) {
	// ① パスパラメータ :id を文字列から uint64 に変換する。
	//    数値以外が渡された場合は 400 を返す。
	userIDParam := c.Param("id")
	userID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不正なユーザーIDです。"})
		return
	}

	// ② リクエストボディを updateUserRequest 構造体にバインドしてバリデーションを実行する。
	//    email は omitempty のため、空文字の場合は形式バリデーションをスキップする。
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "入力内容が正しくありません。"})
		return
	}

	// ③ 更新対象ユーザーがDBに存在するか確認する。
	//    存在しない場合は 404、その他のエラーは 500 を返す。
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "ユーザーが見つかりません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ メールアドレスを変更しようとしている場合のみ重複チェックを行う。
	//    自分自身と同じメールアドレスへの「変更なし」更新は許容するため、
	//    現在の email と異なる場合のみ他ユーザーとの重複を確認する。
	if req.Email != "" && req.Email != user.Email {
		var existing model.User
		// 自分自身（user_id != targetID）を除いて同メールアドレスのユーザーを検索する
		err := h.db.Where("email = ? AND user_id != ?", req.Email, userID).First(&existing).Error
		if err == nil {
			// 他のユーザーが同メールアドレスを使用している
			c.JSON(http.StatusConflict, gin.H{"message": "このメールアドレスはすでに使用されています。"})
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
			return
		}
	}

	// ⑤ ユーザー情報の更新とプロジェクト脱退をトランザクションで実行する。
	//    どちらかが失敗した場合はすべてロールバックされる。
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		// name・email は値が指定されている項目のみ updates マップに追加する。
		// GORM の Updates はゼロ値（空文字）を無視するため、map で明示的に指定する。
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

		// リクエストに projects が含まれている場合、該当プロジェクトのみ脱退する。
		// 指定されていないプロジェクトのメンバーシップには一切触れない。
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

// UserHandler はユーザー関連のHTTPハンドラーをまとめた構造体。
// ユーザー取得・更新・削除の3つの処理を提供する。
type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

// userProjectItem は GetUser レスポンスに含まれるプロジェクト情報。
// プロジェクトID・名前に加え、そのプロジェクトでの権限（authority）を返す。
type userProjectItem struct {
	ProjectID   uint   `json:"project_id"`
	ProjectName string `json:"project_name"`
	Authority   int    `json:"authority"`
}

// getUserResponse は GetUser のレスポンス全体。
// ユーザーの基本情報と所属プロジェクト一覧を返す。
type getUserResponse struct {
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Projects []userProjectItem `json:"projects"`
}

// GetUser はユーザーIDに対応するユーザーの基本情報と、所属プロジェクト一覧（権限含む）を返す。
//
// 処理フロー:
//  1. パスパラメータ :id からユーザーIDを取得・変換
//  2. ユーザーの存在確認
//  3. ユーザーのプロジェクトメンバーシップ一覧を取得
//  4. メンバーシップからプロジェクトIDと権限のマップを構築
//  5. プロジェクト情報を一括取得してレスポンスを組み立てる
func (h *UserHandler) GetUser(c *gin.Context) {
	// ① パスパラメータ :id を文字列から uint64 に変換する。
	userIDParam := c.Param("id")
	userID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不正なユーザーIDです。"})
		return
	}

	// ② ユーザーIDでレコードを取得する。存在しない場合は 404 を返す。
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "ユーザーが見つかりません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ ユーザーが所属するプロジェクトのメンバーシップレコードをすべて取得する。
	//    メンバーシップには ProjectID・Authority（権限）が含まれる。
	var members []model.ProjectMember
	if err := h.db.Where("user_id = ?", userID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	projects := []userProjectItem{}
	if len(members) > 0 {
		// ④ メンバーシップからプロジェクトIDの配列と、プロジェクトIDをキーとした権限マップを構築する。
		//    後のプロジェクト情報取得と権限の紐付けに使用する。
		projectIDs := make([]uint, len(members))
		authorityMap := make(map[uint]int, len(members))
		for i, m := range members {
			projectIDs[i] = m.ProjectID
			authorityMap[m.ProjectID] = m.Authority
		}

		// ⑤ プロジェクトIDのリストを使って、プロジェクト情報を1回のSQLで一括取得する（N+1問題を回避）。
		var projectList []model.Project
		if err := h.db.Where("project_id IN ?", projectIDs).Find(&projectList).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
			return
		}

		// プロジェクト情報と権限マップを組み合わせてレスポンス用の配列を構築する。
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

// DeleteUser は自分自身のアカウントを削除する。
// 他ユーザーのアカウントは削除できない（JWTのuserIDとパスパラメータが一致する場合のみ許可）。
//
// 削除時のデータ処理方針（ソフトデリートではなく物理削除）:
//   - 自分が作成したタスク・コメント・プロジェクトの作成者は「削除済みユーザー」センチネルに付け替える
//     （レコード自体は残すことでデータの整合性を保つ）
//   - 自分が担当しているタスクは未割り当て（user_id = null）に変更する
//   - プロジェクトメンバーシップ・リフレッシュトークンは物理削除する
//   - 最後にユーザーレコード本体を物理削除する
//
// 処理フロー:
//  1. パスパラメータ :id からユーザーIDを取得・変換
//  2. JWT由来の呼び出し元IDと照合し、自分自身かどうか確認
//  3. 「削除済みユーザー」センチネルのIDを取得
//  4. 削除対象がセンチネルユーザー自身でないことを確認
//  5. トランザクション内で関連データの付け替え・削除を実行
func (h *UserHandler) DeleteUser(c *gin.Context) {
	// ① パスパラメータ :id を文字列から uint64 に変換する。
	userIDParam := c.Param("id")
	targetID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不正なユーザーIDです。"})
		return
	}

	// ② JWTミドルウェアがコンテキストにセットした呼び出し元ユーザーID（callerID）を取得し、
	//    削除対象ID（targetID）と一致するか確認する。
	//    一致しない場合は他人のアカウントを削除しようとしているため 403 を返す。
	callerID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証に失敗しました。"})
		return
	}
	if uint(targetID) != callerID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"message": "他のユーザーを削除することはできません。"})
		return
	}

	// ③ 「削除済みユーザー」センチネルアカウントのIDを取得する。
	//    センチネルは起動時に db.go の ensureDeletedUser で自動生成される特殊アカウントで、
	//    ユーザー削除後も作成者情報が欠損しないよう、参照先として使用する。
	deletedUserID, err := internaldb.EnsureDeletedUserID(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ センチネルユーザー自体は削除できないようにガードする。
	//    センチネルが削除されると、既存の「削除済みユーザー」参照が孤立してしまう。
	if uint(targetID) == deletedUserID {
		c.JSON(http.StatusForbidden, gin.H{"message": "このユーザーは削除できません。"})
		return
	}

	// ⑤ 関連データの処理とユーザー削除をすべて1つのトランザクションで実行する。
	//    途中でエラーが発生した場合はすべてロールバックされ、データの不整合を防ぐ。
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		// 自分が作成したタスク（created_by = targetID）の作成者をセンチネルに付け替える。
		// タスクレコード自体は残すことで、プロジェクトのタスク履歴を保持する。
		if err := tx.Model(&model.Task{}).
			Where("created_by = ?", targetID).
			Update("created_by", deletedUserID).Error; err != nil {
			return err
		}

		// 自分が担当しているタスク（user_id = targetID）を未割り当て（null）に変更する。
		// 担当者不在のタスクとして引き続き管理できるようにする。
		if err := tx.Model(&model.Task{}).
			Where("user_id = ?", targetID).
			Update("user_id", nil).Error; err != nil {
			return err
		}

		// 自分が投稿したコメント（created_by = targetID）の作成者をセンチネルに付け替える。
		// コメント内容はそのまま残し、誰が投稿したかの記録のみセンチネルに変更する。
		if err := tx.Model(&model.Comment{}).
			Where("created_by = ?", targetID).
			Update("created_by", deletedUserID).Error; err != nil {
			return err
		}

		// 自分が作成したプロジェクト（created_by = targetID）の作成者をセンチネルに付け替える。
		// プロジェクト自体は継続して利用できるようにする。
		if err := tx.Model(&model.Project{}).
			Where("created_by = ?", targetID).
			Update("created_by", deletedUserID).Error; err != nil {
			return err
		}

		// 全プロジェクトのメンバーシップレコードを削除する。
		// これにより、削除済みユーザーがプロジェクトに参加した状態のままにならない。
		if err := tx.Where("user_id = ?", targetID).Delete(&model.ProjectMember{}).Error; err != nil {
			return err
		}

		// 発行済みリフレッシュトークンをすべて削除する。
		// これにより、削除後に既存トークンでの再ログインを防ぐ。
		if err := tx.Where("user_id = ?", targetID).Delete(&model.RefreshToken{}).Error; err != nil {
			return err
		}

		// 最後にユーザーレコード本体を物理削除する。
		// 上記の関連データを先に処理してから削除することで参照整合性を保つ。
		return tx.Delete(&model.User{}, targetID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ユーザーを削除しました。"})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// ChangePassword は現在のパスワードを確認したうえで新しいパスワードに変更する。
// JWTのuserIDとパスパラメータが一致する場合のみ許可する。
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userIDParam := c.Param("id")
	targetID, err := strconv.ParseUint(userIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不正なユーザーIDです。"})
		return
	}

	callerID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証に失敗しました。"})
		return
	}
	if uint(targetID) != callerID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"message": "他のユーザーのパスワードは変更できません。"})
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "入力内容が正しくありません。"})
		return
	}

	var user model.User
	if err := h.db.First(&user, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "ユーザーが見つかりません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "現在のパスワードが正しくありません。"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	if err := h.db.Model(&user).Update("password", string(hashed)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "パスワードを変更しました。"})
}
