package models

import "gorm.io/gorm"

type Cart struct {
	gorm.Model
	UserID uint       `gorm:"not null;index:idx_user" json:"user_id"`
	Items  []CartItem `gorm:"foreignKey:CartID;references:ID"`
	User   User       `gorm:"foreignKey:UserID;references:ID"`
}

type CartItem struct {
	gorm.Model
	ProductID        uint           `gorm:"not null;index:idx_product" json:"product_id"`
	CartID           uint           `gorm:"not null;index:idx_cart" json:"cart_id"`
	ProductVariantID uint           `gorm:"not null;index:idx_variant" json:"product_variant_id"`
	Quantity         int            `gorm:"not null" json:"quantity"`
	Product          Product        `gorm:"foreignKey:ProductID" json:"product"`
	Cart             Cart           `gorm:"foreignKey:CartID" json:"cart"`
	ProductVariant   ProductVariant `gorm:"foreignKey:ProductVariantID" json:"product_variant"`
}
