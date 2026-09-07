package model

import "time"

type EmailVerification struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	UserID    uint      `gorm:"not null;index"`
	Code      string    `gorm:"not null;size:6"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (EmailVerification) TableName() string {
	return "email_verifications"
}
