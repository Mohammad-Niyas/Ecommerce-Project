package models

import "gorm.io/gorm"

type Address struct {
    gorm.Model
    FirstName      string  `gorm:"type:varchar(255);not null" json:"first_name"`
    LastName       string  `gorm:"type:varchar(255);not null" json:"last_name"`
    Email          *string `gorm:"type:varchar(255);" json:"email"`
    PhoneNumber    string  `gorm:"type:varchar(20);not null" json:"phone_number"`
    Country        string  `gorm:"type:varchar(255);not null" json:"country"`
    Postcode       string  `gorm:"type:varchar(10);not null" json:"postcode"`
    State          string  `gorm:"type:varchar(255);not null" json:"state"`
    City           string  `gorm:"type:varchar(255);not null" json:"city"`
    AddressLine    string  `gorm:"type:varchar(255);not null;column:address" json:"address"`
    Landmark       string  `gorm:"type:varchar(255)" json:"landmark"`
    AlternatePhone *string `gorm:"type:varchar(20)" json:"alternate_phone"`
    UserID         uint    `gorm:"type:int;not null;indexidx_user_id" json:"user_id"`
    DefaultAddress bool    `gorm:"type:bool;default:false;column:default_address" json:"default_address"`
    User           User    `gorm:"foreignKey:UserID;references:ID"`
}

type ShippingAddress struct {
    gorm.Model
    OrderID        uint    `gorm:"not null;index;unique" json:"order_id"`
    UserID         uint    `gorm:"not null;index" json:"user_id"`
    FirstName      string  `gorm:"type:varchar(255);not null" json:"first_name"`
    LastName       string  `gorm:"type:varchar(255);not null" json:"last_name"`
    Email          *string `gorm:"type:varchar(255)" json:"email"`
    PhoneNumber    string  `gorm:"type:varchar(20);not null" json:"phone_number"`
    Country        string  `gorm:"type:varchar(255);not null" json:"country"`
    Postcode       string  `gorm:"type:varchar(10);not null" json:"postcode"`
    State          string  `gorm:"type:varchar(255);not null" json:"state"`
    City           string  `gorm:"type:varchar(255);not null" json:"city"`
    AddressLine    string  `gorm:"type:varchar(255);not null;column:address" json:"address"`
    Landmark       string  `gorm:"type:varchar(255)" json:"landmark"`
    AlternatePhone *string `gorm:"type:varchar(20)" json:"alternate_phone"`
    User           User    `gorm:"foreignKey:UserID;references:ID"`
}