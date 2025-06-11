package models

import (
	"gorm.io/gorm"
)

type User struct {
    gorm.Model
    FirstName   string      `gorm:"type:varchar(255);not null" json:"first_name"`
    LastName    string      `gorm:"type:varchar(255);not null" json:"last_name"`
    Email       string      `gorm:"type:varchar(255);unique;not null;index" json:"email"`
    GoogleID    string      `gorm:"type:varchar(255);" json:"google_id"`
    Password    string      `gorm:"type:varchar(255)" json:"-"` 
    IsActive    bool        `gorm:"default:true" json:"is_active"` 
    UserDetails UserDetails `gorm:"foreignKey:UserID;references:ID"` 
    Addresses   []Address   `gorm:"foreignKey:UserID;references:ID"`
}

type UserDetails struct {
	gorm.Model
	Image       string `gorm:"type:text" json:"image"`
	PhoneNumber string `gorm:"type:varchar(20)" json:"phone_number"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	UserID      uint   `gorm:"not null" json:"user_id"`
}