package models

import (
	"time"

	"gorm.io/gorm"
)

type Category struct {
    gorm.Model
    CategoryName    string          `gorm:"type:varchar(255);not null" json:"category_name"`
    Description     string          `gorm:"type:varchar(255)" json:"description"`
    List            bool            `gorm:"default:true" json:"list"`
    Products        []Product       `gorm:"foreignKey:CategoryID;references:ID" json:"products"`
    CategoryOffers  []CategoryOffer `gorm:"foreignKey:CategoryID;references:ID" json:"category_offers"`
}

type CategoryOffer struct {
    gorm.Model
    CategoryOfferName       string    `gorm:"size:255;not null" json:"category_offer_name"`
    CategoryOfferPercentage float64   `gorm:"type:numeric(5,2);not null" json:"category_offer_percentage"`
    OfferDescription        string    `gorm:"size:255" json:"offer_description"`
    CategoryID              uint      `gorm:"not null;index" json:"category_id"`
    OfferStatus             string    `gorm:"not null" json:"offer_status"`
    StartDate               time.Time `gorm:"not null" json:"start_date"`
    EndDate                 time.Time `gorm:"not null" json:"end_date"`
    Category                Category  `gorm:"foreignKey:CategoryID;references:ID" json:"category"`
}