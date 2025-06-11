package models

import (
	"time"
	"gorm.io/gorm"
)

type Otp struct {
    gorm.Model
    Email      string    `gorm:"type:varchar(255);not null;index" json:"email"`
    Otp        string    `gorm:"type:varchar(4);not null" json:"otp"`
    ExpireTime time.Time `gorm:"not null;index" json:"expire_time"`
}
