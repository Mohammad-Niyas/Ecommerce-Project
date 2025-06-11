package config

import (
	"ecommerce/models"
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func DBconnect() {
	
	dsn := os.Getenv("DSN")
	if dsn == "" {
		log.Fatal("DSN is not set in environment variables")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Database Connected Successfully")
	DB = db

	
	err = DB.AutoMigrate(
		&models.User{},
		&models.Otp{},
		&models.Admin{},
		&models.Category{},
		&models.CategoryOffer{},
		&models.Product{},
		&models.UserDetails{},
		&models.ProductImage{},
		&models.ProductVariant{},
		&models.ProductOffer{},
		&models.Address{},
		&models.Cart{},
		&models.CartItem{},
		&models.Wishlist{},
		&models.WishlistItem{},
		&models.Wallet{},
		&models.WalletTransaction{},
		&models.Order{},
		&models.OrderItem{},
		&models.ReturnRequest{},
		&models.PaymentDetails{},
		&models.ShippingAddress{},
		&models.Coupon{},
	)

	if err != nil {
		log.Fatal("Error migrating database: ", err)
	} else {
		fmt.Println("Database migration completed successfully")
	}
}
