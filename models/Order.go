package models

import (
	"time"

	"gorm.io/gorm"
)

type Order struct {
	gorm.Model
	OrderUID        string          `gorm:"index;not null;unique" json:"orderuid"`
	UserID          uint            `gorm:"not null;index"`
	CouponID        *uint           `gorm:"index" json:"coupon_id"`
	SubTotal        float64         `gorm:"index;type:numeric(10,2)"`
	TotalDiscount   float64         `gorm:"type:numeric(10,2)"`
	CouponDiscount  float64         `gorm:"type:numeric(10,2)"`
	ShippingCharge  float64         `gorm:"type:numeric(10,2)"`
	Tax             float64         `gorm:"index;type:numeric(10,2)"`
	TotalAmount     float64         `gorm:"index;type:numeric(10,2)"`
	OrderDate       time.Time       `gorm:"not null"`
	OrderItem       []OrderItem     `gorm:"foreignKey:OrderID;references:ID"`
	ShippingAddress ShippingAddress `gorm:"foreignKey:OrderID;references:ID"`
	PaymentDetails  PaymentDetails  `gorm:"foreignKey:OrderID;references:ID"`
	Coupon          Coupon          `gorm:"foreignKey:CouponID;references:ID" json:"coupon"`
	User            User            `gorm:"foreignKey:UserID;references:ID"`
}

type OrderItem struct {
    gorm.Model
    OrderID              uint      `gorm:"not null;index"`
    ProductVariantID     uint      `gorm:"not null;index"`
    ProductID            uint      `gorm:"not null;index"`
    Quantity             int       `gorm:"not null;index"`
    ProductName          string    `gorm:"not null;index"`
    Size                 string    `gorm:"type:varchar(20)"`
    ProductCategory      string    `gorm:"not null"`
    ProductActualPrice   float64   `gorm:"index;type:numeric(10,2);not null"`
    ProductSellPrice     float64   `gorm:"index;type:numeric(10,2);not null"`
    Tax                  float64   `gorm:"index;type:numeric(10,2)" json:"tax"`
    Total                float64   `gorm:"index;type:numeric(10,2)" json:"total"`
    ExpectedDeliveryDate time.Time `gorm:"index;not null"`
    OrderStatus          string    `gorm:"type:varchar(20);not null;default:'Processing'" json:"order_status"`
    Reason               string
    ReturnDate           time.Time
    DeliveryDate         time.Time
    ShippedDate          time.Time
    OutForDeliveryDate   time.Time 
    CancelDate           time.Time
    Product              Product   `gorm:"foreignKey:ProductID;references:ID" json:"product"`
}

type ReturnRequest struct {
	gorm.Model
	RequestUID       string         `gorm:"not null;index"`
	OrderItemID      uint           `gorm:"not null;index"`
	ProductID        uint           `gorm:"not null;index"`
	ProductVariantID uint           `gorm:"not null;index"`
	UserID           uint           `gorm:"not null;index"`
	Reason           string         `gorm:"not null"`
	Status           string         `gorm:"default:'pending'"`
	User             User           `gorm:"foreignKey:UserID;references:ID"`
	OrderItem        OrderItem      `gorm:"foreignKey:OrderItemID;references:ID"`
	Product          Product        `gorm:"foreignKey:ProductID" json:"product"`
	ProductVariant   ProductVariant `gorm:"foreignKey:ProductVariantID" json:"product_variant"`
}
