package model

import "time"

type CommentImage struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	CommentID uint      `gorm:"not null;index"`
	ImageURL  string    `gorm:"not null"`
	PublicID  string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (CommentImage) TableName() string {
	return "comment_images"
}
