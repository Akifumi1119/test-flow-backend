package model

import "time"

type User struct {
	UserID    uint      `gorm:"primaryKey;autoIncrement;uniqueIndex;column:user_id"`
	Name      string    `gorm:"not null"`
	Email     string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"`
	Authority *int      `gorm:"column:authority"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (User) TableName() string {
	return "users"
}
