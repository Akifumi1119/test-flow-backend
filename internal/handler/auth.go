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

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
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
		c.JSON(http.StatusOK, gin.H{"exists": 1})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "システムエラーが発生しました。"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"exists": 3})
}
