package models

import (
	"time"

	"gorm.io/gorm"
)

type PaymentDetails struct {
	gorm.Model
	UserID        uint      `gorm:"not null;index" json:"user_id"`                      
	OrderID       uint      `gorm:"not null;index;unique" json:"order_id"`                      
	PaymentAmount float64   `gorm:"type:numeric(10,2);not null" json:"payment_amount"`         
	PaymentMethod string    `gorm:"type:varchar(50);not null" json:"payment_method"`        
	PaymentStatus string    `gorm:"type:varchar(50);not null;index" json:"payment_status"`    
	TransactionID string    `gorm:"type:varchar(255);index" json:"transaction_id"`            
	PaymentDate   time.Time `gorm:"not null" json:"payment_date"`
}