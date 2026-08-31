package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

// AuthHandler は認証関連のHTTPハンドラーをまとめた構造体。
// ログイン・ログアウト・ユーザー登録・ユーザー確認の4つの処理を提供する。
type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// loginRequest はログインAPIのリクエストボディ。
// email・password ともに必須で、email はメールアドレス形式のバリデーションが入る。
type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login はメールアドレスとパスワードで認証を行い、成功した場合に
// JWTアクセストークン（有効期限24時間）とリフレッシュトークン（有効期限7日）を発行する。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. メールアドレスでユーザーを検索
//  3. bcryptでパスワードを検証
//  4. JWTアクセストークンを生成・署名
//  5. リフレッシュトークンを生成してDBに保存
//  6. トークン・ユーザー情報をレスポンスとして返す
func (h *AuthHandler) Login(c *gin.Context) {
	// ① リクエストボディを loginRequest 構造体にバインドしてバリデーションを実行する。
	//    email の形式チェックや required チェックは gin の binding タグが自動で行う。
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ② メールアドレスでユーザーを検索する。
	//    ユーザーが存在しない場合も「パスワード不一致」と同じエラーメッセージを返すことで、
	//    登録済みメールアドレスを推測される「ユーザー列挙攻撃」を防ぐ。
	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "メールアドレスまたはパスワードが正しくありません"})
		return
	}

	// ③ 入力されたパスワードと、DBに保存されている bcrypt ハッシュを比較して検証する。
	//    bcrypt は照合時にハッシュからソルトを自動的に取り出して比較するため、
	//    元のパスワードをDBに保持する必要はない。
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "メールアドレスまたはパスワードが正しくありません"})
		return
	}

	// ④ JWTアクセストークンを生成する。
	//    ペイロードには sub（ユーザーID）と exp（有効期限: 現在時刻から24時間後のUNIXタイム）を含める。
	//    署名アルゴリズムは HS256（HMAC-SHA256）を使用する。
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.UserID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})

	// 署名に使うシークレットキーを環境変数から取得する。
	// 未設定の場合は開発用デフォルト値を使用するが、本番環境では必ず強力な値を設定すること。
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "secret"
	}

	// JWTを署名して文字列形式のトークンを生成する。
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "トークンの生成に失敗しました"})
		return
	}

	// ⑤ リフレッシュトークンを生成する。
	//    crypto/rand を使ってセキュアな乱数32バイトを生成し、16進数文字列にエンコードする（64文字）。
	//    これにより推測困難なトークンを生成できる。
	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "トークンの生成に失敗しました"})
		return
	}
	refreshTokenStr := hex.EncodeToString(refreshTokenBytes)

	// 生成したリフレッシュトークンをDBに保存する（有効期限: 現在時刻から7日後）。
	// ログアウト時はこのレコードを削除することでトークンを無効化する。
	refreshToken := model.RefreshToken{
		UserID:    user.UserID,
		Token:     refreshTokenStr,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := h.db.Create(&refreshToken).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "トークンの生成に失敗しました"})
		return
	}

	// ⑥ アクセストークン・リフレッシュトークン・ユーザー情報をレスポンスとして返す。
	//    フロントエンドはアクセストークンをリクエストヘッダーに、
	//    リフレッシュトークンはトークン再発行時に使用する。
	c.JSON(http.StatusOK, gin.H{
		"token":         signed,
		"refresh_token": refreshTokenStr,
		"user_id":       user.UserID,
		"name":          user.Name,
	})
}

// Logout はDBに保存されているリフレッシュトークンを削除してログアウトを完了する。
// アクセストークン自体はJWT仕様上サーバー側で無効化できないため、
// リフレッシュトークンを削除することで新しいアクセストークンの発行を防ぐ。
//
// 処理フロー:
//  1. JWTミドルウェアがコンテキストにセットしたuserIDを取得
//  2. 該当ユーザーのリフレッシュトークンをDBから全件削除
func (h *AuthHandler) Logout(c *gin.Context) {
	// ① JWTミドルウェア（middleware/auth.go）がリクエスト検証時にコンテキストにセットした
	//    userID を取得する。存在しない場合は認証されていないリクエストなので 401 を返す。
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証に失敗しました。"})
		return
	}

	// ② 該当ユーザーのリフレッシュトークンをDBから全件削除する。
	//    複数デバイスでログインしている場合にすべてのセッションを無効化する。
	if err := h.db.Where("user_id = ?", userID).Delete(&model.RefreshToken{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ログアウトしました。"})
}

// registerRequest はユーザー登録APIのリクエストボディ。
// email・password・name すべて必須で、email はメールアドレス形式のバリデーションが入る。
type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name" binding:"required"`
}

// Register は新規ユーザーをDBに登録する。
// パスワードは bcrypt でハッシュ化して保存し、平文はDBに残らない。
//
// 処理フロー:
//  1. リクエストボディのバリデーション
//  2. メールアドレスの重複チェック（すでに登録済みなら 409 Conflict）
//  3. パスワードを bcrypt でハッシュ化
//  4. ユーザーレコードをDBに挿入
func (h *AuthHandler) Register(c *gin.Context) {
	// ① リクエストボディを registerRequest 構造体にバインドしてバリデーションを実行する。
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "登録失敗しました。"})
		return
	}

	// ② 同じメールアドレスで登録済みのユーザーがいないか確認する。
	//    ErrRecordNotFound 以外のエラーはDB障害のためサーバーエラーとして返す。
	//    ErrRecordNotFound = ユーザー未存在 = 登録続行OK という判定になる。
	var existing model.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		if err == nil {
			// レコードが見つかった = メールアドレスが重複している
			c.JSON(http.StatusConflict, gin.H{"message": "登録失敗しました。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	// ③ パスワードを bcrypt でハッシュ化する。
	//    bcrypt.DefaultCost はコスト係数10で、総当たり攻撃への耐性とパフォーマンスのバランスが取れた標準値。
	//    ハッシュにはソルトが自動で埋め込まれるため、同じパスワードでも毎回異なるハッシュが生成される。
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	// ④ ユーザーレコードをDBに挿入する。パスワードはハッシュ文字列のみ保存する。
	user := model.User{
		Email:    req.Email,
		Password: string(hashed),
		Name:     req.Name,
	}
	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "登録成功しました。"})
}

// checkUserRequest はユーザー確認APIのクエリパラメータ。
// email（メールアドレス）と project_id（確認対象プロジェクト）が必須。
type checkUserRequest struct {
	Email     string `form:"email" binding:"required,email"`
	ProjectID uint   `form:"project_id" binding:"required"`
}

// CheckUser はメールアドレスでユーザーの存在と、指定プロジェクトへの参加状況を確認する。
// プロジェクトへのメンバー招待画面でメールアドレス入力後に呼び出されることを想定している。
//
// レスポンスの exists の値と意味:
//   - 1: ユーザーは存在するが、対象プロジェクトにはまだ参加していない（招待可能）
//   - 2: そのメールアドレスのユーザーが存在しない（未登録）
//   - 3: ユーザーが存在し、対象プロジェクトにすでに参加済み（招待不要）
//
// 処理フロー:
//  1. クエリパラメータのバリデーション
//  2. メールアドレスでユーザーを検索
//  3. 該当プロジェクトのメンバーシップを確認
//  4. 状況に応じた exists 値を返す
func (h *AuthHandler) CheckUser(c *gin.Context) {
	// ① クエリパラメータを checkUserRequest 構造体にバインドしてバリデーションを実行する。
	var req checkUserRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ② メールアドレスでユーザーを検索する。
	//    レコードが存在しない場合はユーザー未登録として exists=2 を返す。
	var user model.User
	err := h.db.Where("email = ?", req.Email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"exists": 2})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ③ ユーザーが存在した場合、対象プロジェクトへの参加状況を確認する。
	//    メンバーシップレコードが存在しない = 未参加 → exists=1（招待可能）を返す。
	//    招待画面でユーザー名を確認できるよう、exists=1 の場合のみ name も返す。
	var member model.ProjectMember
	err = h.db.Where("user_id = ? AND project_id = ?", user.UserID, req.ProjectID).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"exists": 1, "name": user.Name})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	// ④ ユーザーが存在し、かつ対象プロジェクトにすでに参加済み → exists=3 を返す。
	c.JSON(http.StatusOK, gin.H{"exists": 3})
}
