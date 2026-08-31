package model

import "time"

type Task struct {
	TaskID    uint      `gorm:"primaryKey;autoIncrement;uniqueIndex;column:task_id"`
	ProjectID uint      `gorm:"not null;column:project_id"`
	Title     string    `gorm:"not null"`
	Content   string    `gorm:"column:content"`
	Status    int       `gorm:"not null"`
	UserID    *uint     `gorm:"column:user_id"`
	CreatedBy uint      `gorm:"not null;column:created_by"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (Task) TableName() string {
	return "tasks"
}
