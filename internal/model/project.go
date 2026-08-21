package model

import "time"

type Project struct {
	ProjectID   uint      `gorm:"primaryKey;autoIncrement;uniqueIndex;column:project_id"`
	ProjectName string    `gorm:"not null;column:project_name"`
	CreatedAt   time.Time `gorm:"not null"`
	CreatedBy   uint      `gorm:"not null;column:created_by"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (Project) TableName() string {
	return "projects"
}
