package model

import "time"

type Comment struct {
	CommentID uint      `gorm:"primaryKey;autoIncrement;uniqueIndex;column:comment_id"`
	TaskID    uint      `gorm:"not null;column:task_id"`
	Content   string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	CreatedBy uint      `gorm:"not null;column:created_by"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (Comment) TableName() string {
	return "comments"
}
