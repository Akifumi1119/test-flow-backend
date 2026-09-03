package db

import (
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"task-management/backend/internal/model"
)

const DeletedUserEmail = "system-deleted@local"

func New() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		getenv("DB_HOST", "localhost"),
		getenv("DB_USER", "postgres"),
		getenv("DB_PASSWORD", ""),
		getenv("DB_NAME", "task_management"),
		getenv("DB_PORT", "5432"),
		getenv("DB_SSLMODE", "disable"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Project{},
		&model.Task{},
		&model.Comment{},
		&model.ProjectMember{},
		&model.RefreshToken{},
	); err != nil {
		return nil, err
	}

	if err := ensureDeletedUser(db); err != nil {
		return nil, err
	}

	return db, nil
}

// EnsureDeletedUserID は「削除済みユーザー」センチネルのIDを返す。
func EnsureDeletedUserID(db *gorm.DB) (uint, error) {
	var u model.User
	err := db.Where("email = ?", DeletedUserEmail).First(&u).Error
	if err != nil {
		return 0, err
	}
	return u.UserID, nil
}

func ensureDeletedUser(db *gorm.DB) error {
	var u model.User
	err := db.Where("email = ?", DeletedUserEmail).First(&u).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	now := time.Now()
	return db.Create(&model.User{
		Name:      "削除済みユーザー",
		Email:     DeletedUserEmail,
		Password:  "-",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
