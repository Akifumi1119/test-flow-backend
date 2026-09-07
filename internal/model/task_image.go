package model

import "time"

type TaskImage struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	TaskID    uint      `gorm:"not null;index"`
	ImageURL  string    `gorm:"not null"`
	PublicID  string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (TaskImage) TableName() string {
	return "task_images"
}
