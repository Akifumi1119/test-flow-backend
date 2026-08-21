package model

import "time"

type ProjectMember struct {
	ID        uint      `gorm:"primaryKey;autoIncrement;uniqueIndex"`
	ProjectID uint      `gorm:"not null;column:project_id"`
	UserID    uint      `gorm:"not null;column:user_id"`
	Authority int       `gorm:"not null;column:authority"`
	EntryDate time.Time `gorm:"not null;column:entry_date"`
}

func (ProjectMember) TableName() string {
	return "project_members"
}
