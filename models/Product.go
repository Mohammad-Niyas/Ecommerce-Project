package models

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
    gorm.Model
    ProductName   string           `gorm:"type:varchar(255);not null" json:"product_name"`
    Description   string           `gorm:"type:varchar(10000)" json:"description"`
    Brand         string           `gorm:"type:varchar(255)" json:"brand"`
    IsActive      bool             `gorm:"default:true" json:"is_active"`
    CategoryID    uint             `gorm:"not null;index" json:"category_id"`
    Category      Category         `gorm:"foreignKey:CategoryID;references:ID" json:"category"`
    Variants      []ProductVariant `gorm:"foreignKey:ProductID;references:ID" json:"variants"`
    Images        []ProductImage   `gorm:"foreignKey:ProductID;references:ID" json:"images"`
    ProductOffers []ProductOffer   `gorm:"foreignKey:ProductID;references:ID" json:"product_offers"`
}

type ProductImage struct {
	gorm.Model
	ProductID uint   `gorm:"index;not null"`
	ImageURL  string `gorm:"type:text;not null"`
}

type ProductVariant struct {
    gorm.Model
    ProductID    uint        `gorm:"index;not null" json:"product_id"`
    ActualPrice  float64     `gorm:"column:actual_price;type:numeric(10,2);not null" json:"actual_price"`
    SellingPrice float64     `gorm:"column:selling_price;type:numeric(10,2);not null" json:"selling_price"`
    Size         string      `gorm:"type:varchar(50)" json:"size"`
    StockCount   int         `gorm:"not null" json:"stock_count"`
    IsActive     bool        `gorm:"default:true" json:"is_active"`
}

type ProductOffer struct {
    gorm.Model
    OfferName       string    `gorm:"size:255;not null" json:"offer_name"`
    OfferDetails    string    `gorm:"size:255" json:"offer_details"`
    OfferPercentage float64   `gorm:"type:numeric(5,2);not null" json:"offer_percentage"`
    StartDate       time.Time `gorm:"not null" json:"start_date"`
    EndDate         time.Time `gorm:"not null" json:"end_date"`
    ProductID       uint      `gorm:"index;not null" json:"product_id"`
    Status          string    `gorm:"not null" json:"status"`
    Product         Product   `gorm:"foreignKey:ProductID;references:ID" json:"product"`
}