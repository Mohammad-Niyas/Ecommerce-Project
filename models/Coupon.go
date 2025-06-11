package models

import (
	"time"
	"gorm.io/gorm"
)

type Coupon struct {
	gorm.Model							
	CouponCode     string    `gorm:"type:varchar(50);unique;not null" json:"coupon_code"`
	Discount       float64   `gorm:"type:numeric(5,2);not null" json:"discount"`
	UsedCount      int       `gorm:"default:0" json:"used_count"`
	MinAmount      float64   `gorm:"type:numeric(10,2);not null" json:"min_amount"`
	MaxAmount      float64   `gorm:"type:numeric(10,2);not null" json:"max_amount"`
	UsageLimit     int       `gorm:"not null" json:"usage_limit"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	ExpirationDate time.Time `gorm:"not null" json:"expiration_date"`
}