package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"task-management/backend/internal/email"
	"task-management/backend/internal/model"
)

type AuthHandler struct {
	db          *gorm.DB
	emailClient *email.Client
}

func NewAuthHandler(db *gorm.DB, emailClient *email.Client) *AuthHandler {
	return &AuthHandler{db: db, emailClient: emailClient}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "メールアドレスまたはパスワードが正しくありません"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "メールアドレスまたはパスワードが正しくありません"})
		return
	}

	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "メールアドレスが未確認です。届いた確認コードを入力してください。"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.UserID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "secret"
	}

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "トークンの生成に失敗しました"})
		return
	}

	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "トークンの生成に失敗しました"})
		return
	}
	refreshTokenStr := hex.EncodeToString(refreshTokenBytes)

	refreshToken := model.RefreshToken{
		UserID:    user.UserID,
		Token:     refreshTokenStr,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := h.db.Create(&refreshToken).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "トークンの生成に失敗しました"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         signed,
		"refresh_token": refreshTokenStr,
		"user_id":       user.UserID,
		"name":          user.Name,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "認証に失敗しました。"})
		return
	}

	if err := h.db.Where("user_id = ?", userID).Delete(&model.RefreshToken{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ログアウトしました。"})
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "登録失敗しました。"})
		return
	}

	var existing model.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"message": "登録失敗しました。"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	skipVerification := os.Getenv("SKIP_EMAIL_VERIFICATION") == "true"

	user := model.User{
		Email:         req.Email,
		Password:      string(hashed),
		Name:          req.Name,
		EmailVerified: skipVerification,
	}
	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	if skipVerification {
		c.JSON(http.StatusOK, gin.H{"message": "登録成功しました。"})
		return
	}

	if err := h.sendVerificationCode(user.UserID, user.Email, user.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "確認メールの送信に失敗しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "登録成功しました。確認メールをお送りしましたので、コードを入力してください。"})
}

type verifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "入力値が不正です。"})
		return
	}

	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "確認コードが無効です。"})
		return
	}

	if user.EmailVerified {
		c.JSON(http.StatusOK, gin.H{"message": "すでに認証済みです。"})
		return
	}

	var verification model.EmailVerification
	err := h.db.Where("user_id = ? AND code = ? AND expires_at > ?", user.UserID, req.Code, time.Now()).
		First(&verification).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "確認コードが無効または期限切れです。"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	if err := h.db.Model(&user).Update("email_verified", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "サーバーエラーが発生しました。"})
		return
	}

	h.db.Where("user_id = ?", user.UserID).Delete(&model.EmailVerification{})

	c.JSON(http.StatusOK, gin.H{"message": "メールアドレスの確認が完了しました。"})
}

type resendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req resendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "入力値が不正です。"})
		return
	}

	var user model.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// ユーザー存在の有無を外部に漏らさないため成功レスポンスを返す
		c.JSON(http.StatusOK, gin.H{"message": "確認メールを送信しました。"})
		return
	}

	if user.EmailVerified {
		c.JSON(http.StatusOK, gin.H{"message": "すでに認証済みです。"})
		return
	}

	h.db.Where("user_id = ?", user.UserID).Delete(&model.EmailVerification{})

	if err := h.sendVerificationCode(user.UserID, user.Email, user.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "確認メールの送信に失敗しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "確認メールを送信しました。"})
}

type checkUserRequest struct {
	Email     string `form:"email" binding:"required,email"`
	ProjectID uint   `form:"project_id" binding:"required"`
}

func (h *AuthHandler) CheckUser(c *gin.Context) {
	var req checkUserRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

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

	c.JSON(http.StatusOK, gin.H{"exists": 3})
}

func (h *AuthHandler) sendVerificationCode(userID uint, userEmail, userName string) error {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())

	verification := model.EmailVerification{
		UserID:    userID,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := h.db.Create(&verification).Error; err != nil {
		return err
	}

	return h.emailClient.SendVerification(userEmail, userName, code)
}
