package models

import "gorm.io/gorm"

type Admin struct {
	gorm.Model
	Name     string        `gorm:"type:varchar(255);not null" json:"name"`
	Email    string        `gorm:"type:varchar(255);unique;not null" json:"email"`
	Password string        `gorm:"not null" json:"password"`
}
