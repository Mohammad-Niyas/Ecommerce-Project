package models

import (
	"time"

	"gorm.io/gorm"
)

type Wallet struct {
	gorm.Model
	UserID uint `gorm:"not null;index"`
	Balance   float64   `gorm:"index;type:numeric(10,2);default:0.00" json:"balance"`
	User   User `gorm:"foreignKey:UserID;references:ID" json:"user"`
}

type WalletTransaction struct {
    gorm.Model
    WalletID           uint      `gorm:"not null;index" json:"wallet_id"`
    OrderID            *uint     `gorm:"index" json:"order_id"`
    OrderItemID        *uint     `gorm:"index" json:"order_item_id"`
    TransactionUID     string    `gorm:"index;not null;unique" json:"transaction_uid"`
    TransactionAmount  float64   `gorm:"index;type:numeric(10,2);not null" json:"transaction_amount"`
    TransactionType    string    `gorm:"type:varchar(20);not null" json:"transaction_type"`
    TransactionStatus  string    `gorm:"type:varchar(20);not null;default:'Pending'" json:"transaction_status"`
    TransactionDate    time.Time `gorm:"not null;index" json:"transaction_date"`
    Description        string    `gorm:"type:varchar(255)" json:"description"`
    Wallet             Wallet    `gorm:"foreignKey:WalletID;references:ID" json:"wallet"`
}