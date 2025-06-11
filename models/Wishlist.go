package models

import "gorm.io/gorm"

type Wishlist struct {
	gorm.Model
	UserID uint `gorm:"not null;index"`
	User   User `gorm:"foreignKey:UserID;references:ID"`
}

type WishlistItem struct {
    gorm.Model
    WishlistID       uint           `gorm:"not null;index"`
    ProductID        uint           `gorm:"not null;index"`
    ProductVariantID *uint          `gorm:"index"`
    Product          Product        `gorm:"foreignKey:ProductID;references:ID" json:"product"` 
    ProductVariant   ProductVariant `gorm:"foreignKey:ProductVariantID;references:ID" json:"product_variant"`
    Wishlist         Wishlist       `gorm:"foreignKey:WishlistID;references:ID"`
}
