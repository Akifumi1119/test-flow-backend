package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

// CommentHandler はコメント関連のHTTPハンドラーをまとめた構造体。
// コメントの作成・更新・削除の3つの処理を提供する。
type CommentHandler struct {
	db *gorm.DB
}

func NewCommentHandler(db *gorm.DB) *CommentHandler {
	return &CommentHandler{db: db}
}

// createCommentRequest はコメント作成APIのリクエストボディ。
// user_id・task_id・comment はすべて必須。
type createCommentRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	TaskID  uint   `json:"task_id" binding:"required"`
	Comment string `json:"comment" binding:"required"`
}

// createCommentResponse はコメント作成APIのレスポンス。
// 作成したコメントの情報に加え、紐づくタスクのIDとタイトルを含む。
type createCommentResponse struct {
	TaskID    uint      `json:"task_id"`
	Title     string    `json:"title"`     // コメントが紐づくタスクのタイトル
	CommentID uint      `json:"comment_id"`
	Comment   string    `json:"comment"`
	CreatedBy string    `json:"created_by"` // 投稿者の名前（IDではなく名前文字列）
	CreatedAt time.Time `json:"created_at"`
}

// CreateComment は指定したタスクに対してコメントを投稿する。
// 投稿者ユーザーと対象タスクの両方が存在することを確認してからコメントを保存する。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. 投稿者ユーザーの存在確認
//  3. 対象タスクの存在確認
//  4. コメントをDBに保存
//  5. 作成したコメントの情報をレスポンスとして返す
func (h *CommentHandler) CreateComment(c *gin.Context) {
	// ① リクエストボディを createCommentRequest 構造体にバインドしてバリデーションを実行する。
	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② 投稿者ユーザーがDBに存在するか確認する。
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

	// ③ コメントを投稿するタスクがDBに存在するか確認する。
	//    存在しない場合は 400 を返す。
	var task model.Task
	if err := h.db.First(&task, req.TaskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ コメントをDBに保存する。
	//    CreatedAt・UpdatedAt は現在時刻をセットする。
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

	// ⑤ 作成したコメントの情報をレスポンスとして返す。
	//    タスクのタイトルを含めることで、フロントエンドが画面を再取得せずに表示を更新できる。
	c.JSON(http.StatusOK, createCommentResponse{
		TaskID:    task.TaskID,
		Title:     task.Title,
		CommentID: comment.CommentID,
		Comment:   comment.Content,
		CreatedBy: user.Name,
		CreatedAt: comment.CreatedAt,
	})
}

// updateCommentRequest はコメント更新APIのリクエストボディ。
// user_id・comment_id・comment はすべて必須。
type updateCommentRequest struct {
	UserID    uint   `json:"user_id" binding:"required"`
	CommentID uint   `json:"comment_id" binding:"required"`
	Comment   string `json:"comment" binding:"required"`
}

// updateCommentResponse はコメント更新APIのレスポンス。
type updateCommentResponse struct {
	CommentID uint      `json:"comment_id"`
	Comment   string    `json:"comment"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdateComment は既存コメントの内容を更新する。
// 操作ユーザーと更新対象コメントの両方が存在することを確認してから更新する。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. 操作ユーザーの存在確認
//  3. 更新対象コメントの存在確認
//  4. コメント内容と更新日時を変更してDBに保存
//  5. 更新したコメントの情報をレスポンスとして返す
func (h *CommentHandler) UpdateComment(c *gin.Context) {
	// ① リクエストボディを updateCommentRequest 構造体にバインドしてバリデーションを実行する。
	var req updateCommentRequest
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

	// ③ 更新対象コメントがDBに存在するか確認する。
	var comment model.Comment
	if err := h.db.First(&comment, req.CommentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "タスクが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ コメント内容と更新日時を変更してDBに保存する。
	//    GORM の Save は主キーが存在する場合は UPDATE、存在しない場合は INSERT を実行する。
	comment.Content = req.Comment
	comment.UpdatedAt = time.Now()

	if err := h.db.Save(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ⑤ 更新後のコメント情報をレスポンスとして返す。
	c.JSON(http.StatusOK, updateCommentResponse{
		CommentID: comment.CommentID,
		Comment:   comment.Content,
		UpdatedAt: comment.UpdatedAt,
	})
}

// deleteCommentRequest はコメント削除APIのリクエストボディ。
// user_id・comment_id はともに必須。
type deleteCommentRequest struct {
	UserID    uint `json:"user_id" binding:"required"`
	CommentID uint `json:"comment_id" binding:"required"`
}

// DeleteComment は指定したコメントを物理削除する。
// 操作ユーザーと削除対象コメントの両方が存在することを確認してから削除する。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. 操作ユーザーの存在確認
//  3. 削除対象コメントの存在確認
//  4. コメントをDBから物理削除
//  5. 削除したコメントIDをレスポンスとして返す
func (h *CommentHandler) DeleteComment(c *gin.Context) {
	// ① リクエストボディを deleteCommentRequest 構造体にバインドしてバリデーションを実行する。
	var req deleteCommentRequest
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

	// ③ 削除対象コメントがDBに存在するか確認する。
	var comment model.Comment
	if err := h.db.First(&comment, req.CommentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "コメントが存在しません。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ コメントをDBから物理削除する。
	if err := h.db.Delete(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ⑤ 削除したコメントIDをレスポンスとして返す。
	//    フロントエンドはこのIDを使って画面上のコメントリストから該当要素を削除する。
	c.JSON(http.StatusOK, gin.H{"comment_id": req.CommentID})
}
